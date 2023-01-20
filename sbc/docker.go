package sbc

import (
	"errors"
	"fmt"
	"io"
	"os"
	"strings"
	"time"

	"github.com/ZeljkoBenovic/tsbc/cmd/flagnames"
	"github.com/docker/docker/api/types"
	"github.com/docker/docker/api/types/container"
	"github.com/docker/docker/api/types/mount"
	"github.com/spf13/viper"
)

type ContainerName int

const (
	Kamailio ContainerName = iota
	RtpEngine
	LetsEncrypt
)

var ErrContainerNameNotSupported = errors.New("selected container type not supported")

func (s *sbc) handleTLSCertificates() error {
	s.logger.Debug("Checking if SBC TLS certificate is already created")
	fqdnNames, err := s.db.GetAllFqdnNames()
	if err != nil {
		return fmt.Errorf("could not get fqdn names: %w", err)
	}

	if err = s.createAndRunLetsEncrypt(fqdnNames); err != nil {
		return fmt.Errorf("could not create and run lets encrypt node: %w", err)
	}

	return nil
}

func (s *sbc) removeLetsEncryptNode() error {
	nodeID, err := s.db.GetLetsEncryptNodeID()
	if err != nil {
		return fmt.Errorf("could not get letsencrypt container_id: %w", err)
	}

	if nodeID != "" {
		if err = s.dockerCl.ContainerRemove(s.ctx, nodeID, types.ContainerRemoveOptions{
			Force:         true,
			RemoveVolumes: true,
		}); err != nil {
			return fmt.Errorf("could not remove letsencrypt container: %w", err)
		}
	}

	return nil
}

func (s *sbc) createAndRunLetsEncrypt(fqdnNames []string) error {
	// remove the current letsencrypt container
	if err := s.removeLetsEncryptNode(); err != nil {
		return err
	}

	// TODO: modify certs location in kamailio container so that we don't need to create links

	firstFqdnSplitByDot := strings.Split(fqdnNames[0], ".")

	extraDomains := ""
	if len(fqdnNames) > 1 {
		extraDomains = strings.Join(fqdnNames[1:], ",")
	}

	envVars := []string{
		fmt.Sprintf("PUID=1000"),
		fmt.Sprintf("PGID=1000"),
		// TODO: sould be defined from flags
		fmt.Sprintf("TZ=Europe/Belgrade"),
		fmt.Sprintf("VALIDATION=http"),
		fmt.Sprintf("URL=%s", firstFqdnSplitByDot[1]+"."+firstFqdnSplitByDot[2]),
		fmt.Sprintf("SUBDOMAINS=%s", firstFqdnSplitByDot[0]),
		fmt.Sprintf("ONLY_SUBDOMAINS=true"),
		fmt.Sprintf("EXTRA_DOMAINS=%s", extraDomains),
		// TODO: should be defined from flags
		fmt.Sprintf("STAGING=true"),
	}

	if err := s.createAndRunContainer(LetsEncrypt, envVars); err != nil {
		return err
	}

	return nil
}

func (s *sbc) createAndRunSbcInfra() error {
	// environment variables for RTP Engine container
	rtpEngEnvVars := []string{
		fmt.Sprintf("RTP_MAX=%s", s.sbcData.RtpMaxPort),
		fmt.Sprintf("RTP_MIN=%s", s.sbcData.RtpMinPort),
		fmt.Sprintf("MEDIA_PUB_IP=%s", s.sbcData.MediaPublicIP),
		fmt.Sprintf("NG_LISTEN=%s", s.sbcData.NgListen),
	}

	fqdnNames, err := s.db.GetAllFqdnNames()
	if err != nil {
		s.logger.Error("Could not get the list of fqdn names")
		return err
	}

	// the first fqdn name will always be the folder for all certificates
	certFolderName := s.sbcData.SbcName
	if len(fqdnNames) > 1 {
		certFolderName = fqdnNames[0]
	}

	// environment variables for Kamailio container
	kamailioEnvVars := []string{
		fmt.Sprintf("NEW_CONFIG=%t", s.sbcData.NewConfig),
		fmt.Sprintf("EN_DUMP=%t", s.sbcData.EnableSipDump),
		fmt.Sprintf("ADVERTISE_IP=%s", s.sbcData.SbcName),
		fmt.Sprintf("ALIAS=%s", s.sbcData.SbcName),
		fmt.Sprintf("SBC_NAME=%s", s.sbcData.SbcName),
		fmt.Sprintf("CERT_FOLDER_NAME=%s", certFolderName),
		fmt.Sprintf("SBC_PORT=%s", s.sbcData.SbcTLSPort),
		fmt.Sprintf("HOST_IP=%s", viper.GetString(flagnames.HostIP)),
		fmt.Sprintf("UDP_SIP_PORT=%s", s.sbcData.SbcUDPPort),
		fmt.Sprintf("PBX_IP=%s", s.sbcData.PbxIP),
		fmt.Sprintf("PBX_PORT=%s", s.sbcData.PbxPort),
		fmt.Sprintf("RTP_ENG_IP=%s", viper.GetString(flagnames.HostIP)),
		fmt.Sprintf("RTP_ENG_PORT=%s", s.sbcData.RtpEnginePort),
	}

	if err := s.createAndRunContainer(RtpEngine, rtpEngEnvVars); err != nil {
		return fmt.Errorf("could not run rtp-engine container err=%w", err)
	}

	if err := s.createAndRunContainer(Kamailio, kamailioEnvVars); err != nil {
		return fmt.Errorf("could not run kamailio container err=%w", err)
	}

	return nil
}

func (s *sbc) createAndRunContainer(contName ContainerName, envVars []string) error {
	// define container custom parameters
	containerParams := struct {
		imageName               string
		containerName           string
		dbTableName             string
		rowID                   int64
		dockerDefaultHostConfig *container.HostConfig
	}{
		dockerDefaultHostConfig: &container.HostConfig{
			NetworkMode: "host",
			AutoRemove:  false,
			RestartPolicy: container.RestartPolicy{
				Name:              "on-failure",
				MaximumRetryCount: 10,
			},
		},
	}

	switch contName {
	case Kamailio:
		containerParams.imageName = viper.GetString(flagnames.KamailioImage)
		containerParams.containerName = s.sbcData.SbcName + "-kamailio"
		containerParams.dbTableName = "kamailio"
		containerParams.rowID = s.db.GetKamailioInsertID()
		containerParams.dockerDefaultHostConfig.Mounts = []mount.Mount{
			{
				Type:   mount.TypeVolume,
				Source: s.sbcData.SbcName + "-kamcfg",
				Target: "/etc/kamailio",
			},
			{
				// TODO: convert to bind mount as the user needs to be able to see dumps
				Type:   mount.TypeVolume,
				Source: s.sbcData.SbcName + "-sipdump",
				Target: "/tmp",
			},
			{
				Type:   mount.TypeVolume,
				Source: "certificates",
				Target: "/cert",
			},
		}
	case RtpEngine:
		containerParams.imageName = viper.GetString(flagnames.RtpImage)
		containerParams.containerName = s.sbcData.SbcName + "-rtp-engine"
		containerParams.dbTableName = "rtp_engine"
		containerParams.rowID = s.db.GetRTPEngineInsertID()
		containerParams.dockerDefaultHostConfig.Mounts = []mount.Mount{
			{
				Type:   mount.TypeVolume,
				Source: s.sbcData.SbcName + "-rtp_eng_tmp",
				Target: "/tmp",
			},
		}
	case LetsEncrypt:
		containerParams.imageName = "linuxserver/swag"
		containerParams.containerName = "certificates-handler"
		containerParams.dbTableName = "letsencrypt"
		// table index will always be the same, as we have only one instance,
		// and it gets replaced everytime
		containerParams.rowID = 1
		containerParams.dockerDefaultHostConfig.Mounts = []mount.Mount{
			{
				Type:   mount.TypeVolume,
				Source: "certificates",
				Target: "/config/etc/letsencrypt",
			},
		}
	default:
		return ErrContainerNameNotSupported
	}

	reader, err := s.dockerCl.ImagePull(s.ctx, containerParams.imageName, types.ImagePullOptions{})
	if err != nil {
		s.logger.Error("Could not pull Kamailio docker image", "image", containerParams.imageName, "err", err)

		return err
	}

	defer reader.Close()
	// TODO: output docker logs to a log file
	// output logs to console
	io.Copy(os.Stdout, reader)

	resp, err := s.dockerCl.ContainerCreate(s.ctx, &container.Config{
		Image: containerParams.imageName,
		Tty:   false,
		Env:   envVars,
	}, containerParams.dockerDefaultHostConfig, nil, nil, containerParams.containerName)
	if err != nil {
		s.logger.Error("Could not create new container", "image", containerParams.containerName, "err", err)

		return err
	}

	if err = s.db.SaveContainerID(containerParams.rowID, containerParams.dbTableName, resp.ID); err != nil {
		s.logger.Error("Could not save container ID", "err", err)

		return err
	}

	if contName == Kamailio {
		s.logger.Info("Starting kamailio...")
		s.logger.Debug("Sleeping kamailio container deployment due to the certificate generation")
		time.Sleep(30 * time.Second)
	}

	if err = s.dockerCl.ContainerStart(s.ctx, resp.ID, types.ContainerStartOptions{}); err != nil {
		s.logger.Error("Could not start container",
			"id", resp.ID,
			"image", containerParams.containerName,
			"err", err.Error())
	}

	s.logger.Info("Container started",
		"image_name", containerParams.containerName,
		"container_id", resp.ID)

	return nil
}

package sbc

import (
	"errors"
	"fmt"
	"io"
	"os"
	"strings"
	"time"

	"github.com/ZeljkoBenovic/tsbc/cmd/helpers/flagnames"
	"github.com/docker/docker/api/types"
	"github.com/docker/docker/api/types/container"
	"github.com/docker/docker/api/types/mount"
	"github.com/docker/docker/api/types/strslice"
	"github.com/spf13/viper"
)

type ContainerName int

const (
	KamailioContainer ContainerName = iota
	RTPEngineContainer
	LetsEncryptContainer
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

func (s *sbc) stopAllContainersAndRemoveOldTLSCertificate() error {
	allContainers, err := s.dockerCl.ContainerList(s.ctx, types.ContainerListOptions{All: true})
	if err != nil {
		s.logger.Error("Could not list all containers", "err", err)

		return err
	}

	le, err := s.db.GetLetsEncryptNodeID()
	if err != nil {
		s.logger.Error("Could not get letsencrypt node id", "err", err)

		return err
	}

	execID, err := s.dockerCl.ContainerExecCreate(s.ctx, le, types.ExecConfig{
		AttachStderr: true,
		AttachStdout: true,
		Cmd:          []string{"/bin/bash", "-c", "rm -rf /config"},
	})
	if err != nil {
		s.logger.Error("Could not create exec command", "err", err)

		return err
	}

	s.logger.Info("Removing existing certificates...")

	resp, err := s.dockerCl.ContainerExecAttach(s.ctx, execID.ID, types.ExecStartCheck{})
	if err != nil {
		s.logger.Error("Could not get exec response", "err", err)

		return err
	}

	defer resp.Close()

	data, _ := io.ReadAll(resp.Reader)
	s.logger.Debug("Docker exec result", "result", string(data))

	for _, id := range allContainers {
		if err := s.dockerCl.ContainerStop(s.ctx, id.ID, nil); err != nil {
			s.logger.Error("Could not stop container", "id", id)

			return err
		}
	}

	return nil
}

func (s *sbc) startAllContainers() {
	allContainers, err := s.dockerCl.ContainerList(s.ctx, types.ContainerListOptions{All: true})
	if err != nil {
		s.logger.Error("Could not list all containers", "err", err)
	}

	for _, id := range allContainers {
		if err = s.dockerCl.ContainerStart(s.ctx, id.ID, types.ContainerStartOptions{}); err != nil {
			s.logger.Error("Could not start container", "id", id)
		}
	}
}

func (s *sbc) removeLetsEncryptNode() error {
	nodeID, err := s.db.GetLetsEncryptNodeID()
	if err != nil {
		return fmt.Errorf("could not get letsencrypt container_id: %w", err)
	}

	if nodeID != "" {
		// stop all containers
		if err = s.stopAllContainersAndRemoveOldTLSCertificate(); err != nil {
			return err
		}

		// remove container
		if err = s.dockerCl.ContainerRemove(s.ctx, nodeID, types.ContainerRemoveOptions{
			Force:         true,
			RemoveVolumes: true,
		}); err != nil {
			return fmt.Errorf("could not remove letsencrypt container: %w", err)
		}

		// remove database entry
		if err = s.db.RemoveLetsEncryptInfo(nodeID); err != nil {
			return fmt.Errorf("could not remove letsencrypt database info: %w", err)
		}
	}

	return nil
}

func (s *sbc) createAndRunLetsEncrypt(fqdnNames []string) error {
	// remove the current letsencrypt container
	if err := s.removeLetsEncryptNode(); err != nil {
		return err
	}

	firstFqdnSplitByDot := strings.Split(fqdnNames[0], ".")

	extraDomains := ""
	if len(fqdnNames) > 1 {
		extraDomains = strings.Join(fqdnNames[1:], ",")
	}

	envVars := []string{
		fmt.Sprintf("PUID=1000"),
		fmt.Sprintf("PGID=1000"),
		fmt.Sprintf(fmt.Sprintf("TZ=%s", viper.GetString(flagnames.Timezone))),
		fmt.Sprintf("VALIDATION=http"),
		fmt.Sprintf("URL=%s", firstFqdnSplitByDot[1]+"."+firstFqdnSplitByDot[2]),
		fmt.Sprintf("SUBDOMAINS=%s", firstFqdnSplitByDot[0]),
		fmt.Sprintf("ONLY_SUBDOMAINS=true"),
		fmt.Sprintf("EXTRA_DOMAINS=%s", extraDomains),
		fmt.Sprintf(fmt.Sprintf("STAGING=%s", viper.GetString(flagnames.Staging))),
	}

	if err := s.createAndRunContainer(LetsEncryptContainer, envVars); err != nil {
		return err
	}

	s.startAllContainers()

	return nil
}

func (s *sbc) createAndRunSbcInfra() error {
	// environment variables for RTP Engine container
	rtpEngEnvVars := []string{
		fmt.Sprintf("RTP_MAX=%s", s.sbcData.RTPMaxPort),
		fmt.Sprintf("RTP_MIN=%s", s.sbcData.RTPMinPort),
		fmt.Sprintf("MEDIA_PUB_IP=%s", s.sbcData.MediaPublicIP),
		fmt.Sprintf("NG_LISTEN=%s", s.sbcData.NgListen),
	}

	fqdnNames, err := s.db.GetAllFqdnNames()
	if err != nil {
		s.logger.Error("Could not get the list of fqdn names")

		return err
	}

	// the first fqdn name will always be the folder for all certificates
	certFolderName := fqdnNames[0]

	// environment variables for Kamailio container
	kamailioEnvVars := []string{
		fmt.Sprintf("NEW_CONFIG=%t", s.sbcData.NewConfig),
		fmt.Sprintf("EN_SIPDUMP=%t", s.sbcData.EnableSIPDump),
		fmt.Sprintf("ADVERTISE_IP=%s", s.sbcData.SbcName),
		fmt.Sprintf("ALIAS=%s", s.sbcData.SbcName),
		fmt.Sprintf("SBC_NAME=%s", s.sbcData.SbcName),
		fmt.Sprintf("CERT_FOLDER_NAME=%s", certFolderName),
		fmt.Sprintf("SBC_PORT=%s", s.sbcData.SbcTLSPort),
		// TODO: store this in the DB and fetch it from there
		fmt.Sprintf("HOST_IP=%s", viper.GetString(flagnames.HostIP)),
		fmt.Sprintf("UDP_SIP_PORT=%s", s.sbcData.SbcUDPPort),
		fmt.Sprintf("PBX_IP=%s", s.sbcData.PbxIP),
		fmt.Sprintf("PBX_PORT=%s", s.sbcData.PbxPort),
		// TODO: store this in the DB and fetch it from there
		fmt.Sprintf("RTP_ENG_IP=%s", viper.GetString(flagnames.HostIP)),
		fmt.Sprintf("RTP_ENG_PORT=%s", s.sbcData.RTPEnginePort),
	}

	if err = s.createAndRunContainer(RTPEngineContainer, rtpEngEnvVars); err != nil {
		return fmt.Errorf("could not run rtp-engine container err=%w", err)
	}

	if err = s.createAndRunContainer(KamailioContainer, kamailioEnvVars); err != nil {
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
	case KamailioContainer:
		containerParams.imageName = viper.GetString(flagnames.KamailioImage)
		containerParams.containerName = s.sbcData.SbcName + "-kamailio"
		containerParams.dbTableName = "kamailio"
		containerParams.rowID = s.db.GetKamailioInsertID(s.sbcData.SbcName)
		containerParams.dockerDefaultHostConfig.Mounts = []mount.Mount{
			{
				Type:   mount.TypeVolume,
				Source: s.sbcData.SbcName + "-kamcfg",
				Target: "/etc/kamailio",
			},
			{
				Type:   mount.TypeVolume,
				Source: "certificates",
				Target: "/cert",
			},
		}
		containerParams.dockerDefaultHostConfig.Mounts = append(
			containerParams.dockerDefaultHostConfig.Mounts,
			s.handleSIPDumpVolume())

	case RTPEngineContainer:
		containerParams.imageName = viper.GetString(flagnames.RTPImage)
		containerParams.containerName = s.sbcData.SbcName + "-rtp-engine"
		containerParams.dbTableName = "rtp_engine"
		containerParams.rowID = s.db.GetRTPEngineInsertID(s.sbcData.SbcName)
		containerParams.dockerDefaultHostConfig.Mounts = []mount.Mount{
			{
				Type:   mount.TypeVolume,
				Source: s.sbcData.SbcName + "-rtp_eng_tmp",
				Target: "/tmp",
			},
		}

	case LetsEncryptContainer:
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
		containerParams.dockerDefaultHostConfig.CapAdd = strslice.StrSlice{"NET_ADMIN"}

	default:
		return ErrContainerNameNotSupported
	}

	reader, err := s.dockerCl.ImagePull(s.ctx, containerParams.imageName, types.ImagePullOptions{})
	if err != nil {
		s.logger.Error("Could not pull docker image", "image", containerParams.imageName, "err", err)

		return err
	}

	defer reader.Close()
	// output logs to log file
	_, _ = io.Copy(s.dockerLogFile, reader)

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

	if contName == KamailioContainer {
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

// handleSIPDumpVolume returns a bind mount or regular docker volume based on sip-dump flag
func (s *sbc) handleSIPDumpVolume() mount.Mount {
	sipDumpMount := mount.Mount{
		Type:   mount.TypeVolume,
		Source: s.sbcData.SbcName + "-sipdump",
		Target: "/tmp",
	}

	if s.sbcData.EnableSIPDump {
		currentDir, err := os.Getwd()
		if err != nil {
			s.logger.Debug("Could not get current working directory, setting path to /tmp")

			currentDir = "/tmp"
		}

		bindPath := currentDir + "/sipdump/" + s.sbcData.SbcName
		if err = os.MkdirAll(bindPath, 755); err != nil {
			s.logger.Error("Could not create directory using regular docker volume",
				"dir", bindPath, "err", err)

			// if the folder creation fails create just the regular volume mount
			return sipDumpMount
		}

		sipDumpMount = mount.Mount{
			Type:   mount.TypeBind,
			Source: bindPath,
			Target: "/tmp",
		}
	}

	return sipDumpMount
}

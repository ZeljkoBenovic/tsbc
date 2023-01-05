package sbc

import (
	"errors"
	"fmt"
	"io"
	"os"

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
)

var dockerDefaultHostConfig = &container.HostConfig{
	NetworkMode: "host",
	AutoRemove:  false,
	RestartPolicy: container.RestartPolicy{
		Name:              "no",
		MaximumRetryCount: 0,
	},
}
var ErrContainerNameNotSupported = errors.New("selected container type not supported")

func (s *sbc) createAndRunSbcInfra() error {
	// environment variables for RTP Engine container
	rtpEngEnvVars := []string{
		fmt.Sprintf("RTP_MAX=%s", s.sbcData.RtpMaxPort),
		fmt.Sprintf("RTP_MIN=%s", s.sbcData.RtpMinPort),
		fmt.Sprintf("MEDIA_PUB_IP=%s", s.sbcData.MediaPublicIP),
		fmt.Sprintf("NG_LISTEN=%s", s.sbcData.NgListen),
	}

	// environment variables for Kamailio container
	kamailioEnvVars := []string{
		fmt.Sprintf("NEW_CONFIG=%t", s.sbcData.NewConfig),
		fmt.Sprintf("EN_DUMP=%t", s.sbcData.EnableSipDump),
		fmt.Sprintf("ADVERTISE_IP=%s", s.sbcData.SbcName),
		fmt.Sprintf("ALIAS=%s", s.sbcData.SbcName),
		fmt.Sprintf("SBC_NAME=%s", s.sbcData.SbcName),
		fmt.Sprintf("SBC_PORT=%s", s.sbcData.SbcTLSPort),
		// TODO: add host ip into db, and set as host ip - HOST_IP
		fmt.Sprintf("UDP_SIP_PORT=%s", s.sbcData.SbcUDPPort),
		fmt.Sprintf("PBX_IP=%s", s.sbcData.PbxIP),
		fmt.Sprintf("PBX_PORT=%s", s.sbcData.PbxPort),
		// TODO: add host ip into db, get it and set as rptengine host - RTP_ENG_IP
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
	var (
		containerName        string
		containerNamePostFix string
	)

	switch contName {
	case Kamailio:
		containerName = viper.GetString(flagnames.KamailioImage)
		containerNamePostFix = "-kamailio"
		dockerDefaultHostConfig.Mounts = []mount.Mount{
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
		}
	case RtpEngine:
		containerName = viper.GetString(flagnames.RtpImage)
		containerNamePostFix = "-rtp-engine"
		dockerDefaultHostConfig.Mounts = []mount.Mount{
			{
				Type:   mount.TypeVolume,
				Source: s.sbcData.SbcName + "-rtp_eng_tmp",
				Target: "/tmp",
			},
		}
	default:
		return ErrContainerNameNotSupported
	}

	reader, err := s.dockerCl.ImagePull(s.ctx, containerName, types.ImagePullOptions{})
	if err != nil {
		s.logger.Error("Could not pull Kamailio docker image", "image", containerName, "err", err)

		return err
	}

	defer reader.Close()
	// output logs to console
	io.Copy(os.Stdout, reader)

	resp, err := s.dockerCl.ContainerCreate(s.ctx, &container.Config{
		Image: containerName,
		Tty:   false,
		Env:   envVars,
	}, dockerDefaultHostConfig, nil, nil, s.sbcData.SbcName+containerNamePostFix)
	if err != nil {
		s.logger.Error("Could not create new container", "image", containerName, "err", err)

		return err
	}

	if err = s.dockerCl.ContainerStart(s.ctx, resp.ID, types.ContainerStartOptions{}); err != nil {
		s.logger.Error("Could not start container",
			"id", resp.ID,
			"image", containerName,
			"err", err.Error())
	}

	s.logger.Info("Container started",
		"image_name", containerName,
		"container_id", resp.ID)

	return nil
}

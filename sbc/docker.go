package sbc

import (
	"io"
	"os"

	"github.com/docker/docker/api/types"
	"github.com/docker/docker/api/types/container"
	"github.com/docker/docker/pkg/stdcopy"
)

// TODO: handle creation of Kamailio and RtpEngine with appropriate env vars
func (s *sbc) createAndRunContainers() {
	reader, err := s.dockerCl.ImagePull(s.ctx, s.config.KamailioImage, types.ImagePullOptions{})
	if err != nil {
		s.logger.Error("Could not pull Kamailio docker image", "image", s.config.KamailioImage, "err", err)

		return
	}

	defer reader.Close()
	io.Copy(os.Stdout, reader)

	resp, err := s.dockerCl.ContainerCreate(s.ctx, &container.Config{
		Image: s.config.KamailioImage,
		Cmd:   []string{"echo", "hello world"},
		Tty:   false,
	}, nil, nil, nil, s.config.DomainName+"-kamailio")
	if err != nil {
		s.logger.Error("Could not create new container", "image", s.config.KamailioImage, "err", err)
		return
	}

	if err := s.dockerCl.ContainerStart(s.ctx, resp.ID, types.ContainerStartOptions{}); err != nil {
		s.logger.Error("Could not start container",
			"id", resp.ID,
			"image", s.config.KamailioImage,
			"err", err.Error())
	}

	statusCh, errCh := s.dockerCl.ContainerWait(s.ctx, resp.ID, container.WaitConditionNotRunning)
	select {
	case err := <-errCh:
		if err != nil {
			s.logger.Error("Error waiting for container", "id", resp.ID, "err", err)
			return
		}
	case <-statusCh:
	}

	out, err := s.dockerCl.ContainerLogs(s.ctx, resp.ID, types.ContainerLogsOptions{ShowStdout: true})
	if err != nil {
		panic(err)
	}

	stdcopy.StdCopy(os.Stdout, os.Stderr, out)
}

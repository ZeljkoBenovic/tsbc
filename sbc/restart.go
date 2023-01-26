package sbc

import "time"

func (s *sbc) Restart(fqdnName string) error {
	s.logger.Info("Restarting cluster", "fqdn", fqdnName)

	timeOut := time.Second * 30

	for _, containerID := range s.db.GetContainerIDsFromSbcFqdn(fqdnName) {
		if err := s.dockerCl.ContainerRestart(s.ctx, containerID, &timeOut); err != nil {
			s.logger.Error("Could not restart container", "id", containerID)

			return err
		}
	}

	s.logger.Info("Cluster restarted", "fqdn", fqdnName)

	return nil
}

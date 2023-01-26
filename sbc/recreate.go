package sbc

import "github.com/docker/docker/api/types"

func (s *sbc) Recreate(fqdnName string) error {
	var err error
	s.logger.Info("Recreating cluster", "fqdn", fqdnName)

	// get container ids from the fqdn
	for _, containerID := range s.db.GetContainerIDsFromSbcFqdn(fqdnName) {
		if err = s.dockerCl.ContainerRemove(s.ctx, containerID, types.ContainerRemoveOptions{
			RemoveVolumes: true,
			Force:         true,
		}); err != nil {
			s.logger.Error("Could not remove container", "id", containerID, "err", err)

			return err
		}
	}

	// get all sbc information form the sbc id
	s.sbcData, err = s.db.GetSBCParameters(s.db.GetSBCIdFromFqdn(fqdnName))
	if err != nil {
		s.logger.Error("Could not get sbc parameters", "fqdn", fqdnName, "err", err)

		return err
	}

	// create and run the cluster
	if err = s.createAndRunSbcInfra(); err != nil {
		s.logger.Error("Could not create and run cluster", "fqdn", fqdnName, "err", err)

		return err
	}

	s.logger.Info("Containers recreated", "fqdn", fqdnName)

	return nil
}

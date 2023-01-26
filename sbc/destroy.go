package sbc

import (
	"errors"
	"fmt"

	"github.com/docker/docker/api/types"
	"github.com/spf13/viper"
)

var ErrCouldNotGetContainerIDs = errors.New("could not get the list of container IDs")

func (s *sbc) Destroy(fqdnName string) error {
	// retrieve container ids from fqdn
	ids := s.db.GetContainerIDsFromSbcFqdn(fqdnName)
	if ids == nil {
		return ErrCouldNotGetContainerIDs
	}

	// destroy containers one by one
	for _, id := range ids {
		s.logger.Debug("Deleting container", "id", id)

		if err := s.destroyContainerWithVolumes(id); err != nil {
			s.logger.Error("Could not destroy container", "id", id, "err", err)

			continue
		}

		s.logger.Debug("Container deleted", "id", id)
	}

	s.logger.Info("Containers destroyed successfully")

	if err := s.db.RemoveSbcInfo(viper.GetString("destroy.fqdn")); err != nil {
		return fmt.Errorf("sould not remove sbc info from database: %w", err)
	}

	s.logger.Info("Database information deleted")

	return nil
}

func (s *sbc) destroyContainerWithVolumes(containerID string) error {
	// inspect container to get the list of volumes
	cDetails, err := s.dockerCl.ContainerInspect(s.ctx, containerID)
	if err != nil {
		return fmt.Errorf("could not inspect container: %w", err)
	}

	// remove container
	if err = s.dockerCl.ContainerRemove(s.ctx, containerID, types.ContainerRemoveOptions{
		RemoveVolumes: true,
		Force:         true,
	}); err != nil {
		return err
	}

	s.logger.Debug("Container removed", "id", containerID)

	for _, vol := range cDetails.Mounts {
		// dont delete certificates volume
		if vol.Name == "certificates" && !viper.GetBool("destroy.tls-node") {
			continue
		}
		if err = s.dockerCl.VolumeRemove(s.ctx, vol.Name, true); err != nil {
			s.logger.Error("Could not delete volume", "name", vol.Name, "err", err)
		}

		s.logger.Debug("Volume removed", "name", vol.Name)
	}

	return nil
}

func (s *sbc) DestroyLetsEncryptNode() error {
	// get LetsEncrypt node ID
	nodeId, err := s.db.GetLetsEncryptNodeID()
	if err != nil {
		return fmt.Errorf("could not get LetsEncrypt node id: %w", err)
	}

	// and destroy that node
	if err = s.destroyContainerWithVolumes(nodeId); err != nil {
		s.logger.Error("Could not destroy LetsEncrypt node", "err", err)
	}

	// delete database records
	if err = s.db.RemoveLetsEncryptInfo(nodeId); err != nil {
		return fmt.Errorf("could not remove LetsEncrypt node: %w", err)
	}

	s.logger.Info("LetsEncrypt node deleted")

	return nil
}

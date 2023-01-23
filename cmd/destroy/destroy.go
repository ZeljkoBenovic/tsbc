package destroy

import (
	"context"
	"fmt"
	"log"
	"os"

	"github.com/ZeljkoBenovic/tsbc/cmd/helpers/flagnames"
	"github.com/ZeljkoBenovic/tsbc/db"
	"github.com/docker/docker/api/types"
	"github.com/docker/docker/client"
	"github.com/hashicorp/go-hclog"
	"github.com/spf13/cobra"
	"github.com/spf13/viper"
)

// destroyCmd represents the destroy command
// TODO: edit long description
var destroyCmd = &cobra.Command{
	Use:   "destroy",
	Short: "Destroy SBC environment",
	Long: `A longer description that spans multiple lines and likely contains examples
and usage of using your command. For example:

Cobra is a CLI library for Go that empowers applications.
This application is a tool to generate the needed files
to quickly create a Cobra application.`,
	Run: runCommandHandler,
}

type destroy struct {
	db     db.IDB
	logger hclog.Logger
	dClt   *client.Client
	ctx    context.Context
}

func GetCmd() *cobra.Command {
	// define flags
	destroyCmd.Flags().String(flagnames.SbcFqdn, "", "SBC FQDN to destroy")
	destroyCmd.Flags().String(flagnames.LogLevel, "info", "set log level")
	destroyCmd.Flags().String(flagnames.DBFileLocation, "",
		fmt.Sprintf("sqlite file location, file name must end with .db (default: %s)", db.DefaultDBLocation()))
	destroyCmd.Flags().Bool(flagnames.DestroyTlsNode, false, "destroy LetsEncrypt instance")

	destroyCmd.MarkFlagsMutuallyExclusive(flagnames.SbcFqdn, flagnames.DestroyTlsNode)

	// bind flags to viper
	if err := viper.BindPFlag("destroy.fqdn", destroyCmd.Flag(flagnames.SbcFqdn)); err != nil {
		log.Fatalln("Could not bind destroy.fqdn err:", err.Error())
	}

	if err := viper.BindPFlag("destroy.log-level", destroyCmd.Flag(flagnames.LogLevel)); err != nil {
		log.Fatalln("Could not bind destroy.log-level:", err.Error())
	}

	if err := viper.BindPFlag("destroy.db-file", destroyCmd.Flag(flagnames.DBFileLocation)); err != nil {
		log.Fatalln("Could not bind destroy.db-file:", err.Error())
	}

	if err := viper.BindPFlag("destroy.tls-node", destroyCmd.Flag(flagnames.DestroyTlsNode)); err != nil {
		log.Fatalln("Could not bind destroy.tls-node", err.Error())
	}

	// add command to root
	return destroyCmd
}

func newDestroy() (*destroy, error) {
	logger := hclog.New(&hclog.LoggerOptions{
		Name:                 "sbc.destroy",
		Level:                hclog.LevelFromString(viper.GetString("destroy.log-level")),
		Color:                hclog.AutoColor,
		ColorHeaderAndFields: true,
	})

	dClt, err := client.NewClientWithOpts(client.FromEnv, client.WithAPIVersionNegotiation())
	if err != nil {
		return nil, fmt.Errorf("could not initialize docker: %w", err)
	}

	dbInst, err := db.NewDB(logger, defaultDBLocation())
	if err != nil {
		return nil, fmt.Errorf("could not create new database instance: %w", err)
	}

	return &destroy{
		db:     dbInst,
		logger: logger,
		dClt:   dClt,
		ctx:    context.Background(),
	}, nil
}

func runCommandHandler(cmd *cobra.Command, _ []string) {
	// create new destroy instance
	dst, err := newDestroy()
	if err != nil {
		dst.logger.Error("Could not run destroy", "err", err)

		os.Exit(1)
	}

	defer dst.close()

	// destroy LetsEncrypt instance only, if selected
	if viper.GetBool("destroy.tls-node") {
		dst.destroyLetsEncryptNode()

		return
	}

	// check that sbc-fqdn flag is set
	if viper.GetString("destroy.fqdn") == "" {
		dst.logger.Error("SBC FQDN flag not set, but it is required")
		os.Exit(1)
	}

	// retrieve container ids from fqdn
	ids := dst.db.GetContainerIDsFromSbcFqdn(viper.GetString("destroy.fqdn"))
	if ids == nil {
		dst.logger.Error("Could not get the list of container IDs", "err", err)

		os.Exit(1)
	}

	// destroy containers one by one
	for _, id := range ids {
		dst.logger.Debug("Deleting container", "id", id)

		if err = dst.destroyContainerWithVolumes(id); err != nil {
			dst.logger.Error("Could not destroy container", "id", id, "err", err)

			continue
		}

		dst.logger.Debug("Container deleted", "id", id)
	}

	dst.logger.Info("Containers destroyed successfully")

	if err = dst.db.RemoveSbcInfo(viper.GetString("destroy.fqdn")); err != nil {
		dst.logger.Error("Could not remove sbc info from database", "err", err)
		os.Exit(1)
	}

	dst.logger.Info("Database information deleted")
}

func (d *destroy) destroyLetsEncryptNode() {
	// get LetsEncrypt node ID
	nodeId, err := d.db.GetLetsEncryptNodeID()
	if err != nil {
		d.logger.Error("Could not get LetsEncrypt node id", "err", err)

		return
	}

	// and destroy that node
	if err = d.destroyContainerWithVolumes(nodeId); err != nil {
		d.logger.Warn("Could not destroy LetsEncrypt node", "err", err)
	}

	// delete database records
	if err = d.db.RemoveLetsEncryptInfo(nodeId); err != nil {
		d.logger.Error("Could not delete LetsEncrypt", "err", err)
	}

	d.logger.Info("LetsEncrypt node deleted")

	return
}

func (d *destroy) destroyContainerWithVolumes(containerID string) error {
	// inspect container to get the list of volumes
	cDetails, err := d.dClt.ContainerInspect(d.ctx, containerID)
	if err != nil {
		return fmt.Errorf("could not inspect container: %w", err)
	}

	// remove container
	if err = d.dClt.ContainerRemove(d.ctx, containerID, types.ContainerRemoveOptions{
		RemoveVolumes: true,
		Force:         true,
	}); err != nil {
		return err
	}

	d.logger.Debug("Container removed", "id", containerID)

	for _, vol := range cDetails.Mounts {
		// dont delete certificates volume
		if vol.Name == "certificates" && !viper.GetBool("destroy.tls-node") {
			continue
		}
		if err = d.dClt.VolumeRemove(d.ctx, vol.Name, true); err != nil {
			d.logger.Error("Could not delete volume", "name", vol.Name, "err", err)
		}

		d.logger.Debug("Volume removed", "name", vol.Name)
	}

	return nil
}

// close database and docker client
func (d *destroy) close() {
	_ = d.dClt.Close()
	_ = d.db.Close()
}

func defaultDBLocation() string {
	// set default location for database file
	if viper.GetString("destroy.db-file") == "" {
		return db.DefaultDBLocation()
	}

	return viper.GetString("destroy.db-file")
}

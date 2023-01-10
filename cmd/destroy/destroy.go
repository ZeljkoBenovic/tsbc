package destroy

import (
	"context"
	"log"
	"os"

	"github.com/ZeljkoBenovic/tsbc/cmd/flagnames"
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

	// mark required flags
	_ = destroyCmd.MarkFlagRequired(flagnames.SbcFqdn)

	// bind flags to viper
	if err := viper.BindPFlag("destroy.fqdn", destroyCmd.Flag(flagnames.SbcFqdn)); err != nil {
		log.Fatalln("Could not bind to flags err=", err.Error())
	}

	// add command to root
	return destroyCmd
}

func runCommandHandler(_ *cobra.Command, _ []string) {
	var (
		err error
		dst = destroy{
			ctx: context.Background(),
		}
	)

	defer dst.close()

	dst.logger = hclog.New(&hclog.LoggerOptions{
		Name:                 "sbc.destroy",
		Level:                hclog.LevelFromString(viper.GetString(flagnames.LogLevel)),
		Color:                hclog.AutoColor,
		ColorHeaderAndFields: true,
	})

	dst.db, err = db.NewDB(dst.logger)
	if err != nil {
		dst.logger.Error("Could not create new database instance", "err", err)

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

func (d *destroy) destroyContainerWithVolumes(containerID string) error {
	var err error
	// create new docker client instance
	d.dClt, err = client.NewClientWithOpts(client.FromEnv, client.WithAPIVersionNegotiation())
	if err != nil {
		return err
	}

	// remove container
	if err = d.dClt.ContainerRemove(d.ctx, containerID, types.ContainerRemoveOptions{
		RemoveVolumes: true,
		Force:         true,
	}); err != nil {
		return err
	}

	// TODO: remove volumes

	return nil
}

// close database and docker client
func (d *destroy) close() {
	d.dClt.Close()
	d.db.Close()
}

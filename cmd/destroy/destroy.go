package destroy

import (
	"context"
	"fmt"
	"log"
	"os"

	"github.com/ZeljkoBenovic/tsbc/cmd/helpers/flagnames"
	"github.com/ZeljkoBenovic/tsbc/db"
	"github.com/ZeljkoBenovic/tsbc/sbc"
	"github.com/docker/docker/client"
	"github.com/hashicorp/go-hclog"
	"github.com/spf13/cobra"
	"github.com/spf13/viper"
)

// destroyCmd represents the destroy command
var destroyCmd = &cobra.Command{
	Use:   "destroy",
	Short: "Destroy SBC cluster or TLS node",
	Example: "tsbc destroy --sbc-fqdn sbc.test1.com\n" +
		"tsbc destroy --tls-node",
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
	destroyCmd.Flags().Bool(flagnames.DestroyTLSNode, false, "destroy LetsEncrypt instance")

	destroyCmd.MarkFlagsMutuallyExclusive(flagnames.SbcFqdn, flagnames.DestroyTLSNode)

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

	if err := viper.BindPFlag("destroy.tls-node", destroyCmd.Flag(flagnames.DestroyTLSNode)); err != nil {
		log.Fatalln("Could not bind destroy.tls-node", err.Error())
	}

	// add command to root
	return destroyCmd
}

func runCommandHandler(cmd *cobra.Command, _ []string) {
	hlog := hclog.New(&hclog.LoggerOptions{
		Name:                 "destroy",
		Color:                hclog.AutoColor,
		ColorHeaderAndFields: true,
	})

	sbcFqdn := viper.GetString("destroy.fqdn")

	sbcInst, err := sbc.NewSBC()
	if err != nil {
		hlog.Error("Could not create new sbc instance", "err", err)

		os.Exit(1)
	}

	// destroy LetsEncrypt instance only, if selected
	if viper.GetBool("destroy.tls-node") {
		if err = sbcInst.DestroyLetsEncryptNode(); err != nil {
			hlog.Error("Could not remove LetsEncrypt node", "err", err)
		}

		return
	}

	// check that sbc-fqdn flag is set
	if sbcFqdn == "" {
		hlog.Error("SBC FQDN flag not set, but it is required")
		os.Exit(1)
	}

	defer sbcInst.Close()

	if err = sbcInst.Destroy(sbcFqdn); err != nil {
		hlog.Error("Could not destroy cluster", "fqdn")
	}
}

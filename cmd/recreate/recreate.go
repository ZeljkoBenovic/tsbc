package recreate

import (
	"log"

	"github.com/ZeljkoBenovic/tsbc/cmd/helpers/flagnames"
	"github.com/ZeljkoBenovic/tsbc/sbc"
	"github.com/hashicorp/go-hclog"
	"github.com/spf13/cobra"
	"github.com/spf13/viper"
)

var recreateCmd = &cobra.Command{
	Use:     "recreate",
	Short:   "Command used to recreate SBC nodes",
	Example: "tsbc recreate --fqdn-name sbc.test.com",
	Run:     recreateCommandHandler,
}

func GetCmd() *cobra.Command {
	recreateCmd.Flags().String(flagnames.SbcFqdn, "", "fqdn of the sbc cluster to restart")
	recreateCmd.Flags().String(flagnames.LogLevel, "info", "set log level")

	_ = recreateCmd.MarkFlagRequired(flagnames.SbcFqdn)

	// bind flags to viper
	if err := viper.BindPFlag("recreate.fqdn", recreateCmd.Flag(flagnames.SbcFqdn)); err != nil {
		log.Fatalln("Could not bind restart.fqdn err:", err.Error())
	}

	if err := viper.BindPFlag("recreate.log-level", recreateCmd.Flag(flagnames.LogLevel)); err != nil {
		log.Fatalln("Could not bind restart.log-level err:", err.Error())
	}

	return recreateCmd
}

func recreateCommandHandler(_ *cobra.Command, _ []string) {
	lg := hclog.New(&hclog.LoggerOptions{
		Name:                 "recreate",
		Level:                hclog.LevelFromString(viper.GetString("recreate.log-level")),
		Color:                hclog.AutoColor,
		ColorHeaderAndFields: true,
	})

	sbcInst, err := sbc.NewSBC()
	if err != nil {
		lg.Error("Could not create new sbc instance", "err", err)
	}

	defer sbcInst.Close()

	if err = sbcInst.Recreate(viper.GetString("recreate.fqdn")); err != nil {
		lg.Error("Could not recreate sbc cluster", "err", err, "fqdn", viper.GetString("recreate.fqdn"))
	}
}

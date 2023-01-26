package restart

import (
	"log"

	"github.com/ZeljkoBenovic/tsbc/cmd/helpers/flagnames"
	"github.com/ZeljkoBenovic/tsbc/sbc"
	"github.com/hashicorp/go-hclog"
	"github.com/spf13/cobra"
	"github.com/spf13/viper"
)

var restartCmd = &cobra.Command{
	Use:     "restart",
	Short:   "Command used to restart SBC nodes",
	Example: "tsbc restart --fqdn-name sbc.test.com",
	Run:     restartCommandHandler,
}

func GetCmd() *cobra.Command {
	restartCmd.Flags().String(flagnames.SbcFqdn, "", "fqdn of the sbc cluster to restart")
	restartCmd.Flags().String(flagnames.LogLevel, "info", "set log level")

	_ = restartCmd.MarkFlagRequired(flagnames.SbcFqdn)

	// bind flags to viper
	if err := viper.BindPFlag("restart.fqdn", restartCmd.Flag(flagnames.SbcFqdn)); err != nil {
		log.Fatalln("Could not bind restart.fqdn err:", err.Error())
	}

	if err := viper.BindPFlag("restart.log-level", restartCmd.Flag(flagnames.LogLevel)); err != nil {
		log.Fatalln("Could not bind restart.log-level err:", err.Error())
	}

	return restartCmd
}

func restartCommandHandler(_ *cobra.Command, _ []string) {
	lg := hclog.New(&hclog.LoggerOptions{
		Name:                 "restart",
		Level:                hclog.LevelFromString(viper.GetString("restart.log-level")),
		Color:                hclog.AutoColor,
		ColorHeaderAndFields: true,
	})

	sbcInst, err := sbc.NewSBC()
	if err != nil {
		lg.Error("Could not create new sbc instance", "err", err)
	}

	defer sbcInst.Close()

	if err = sbcInst.Restart(viper.GetString("restart.fqdn")); err != nil {
		lg.Error("Could not restart sbc cluster", "err", err, "fqdn", viper.GetString("restart.fqdn"))
	}
}

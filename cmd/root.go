package cmd

import (
	"fmt"
	"log"

	"github.com/ZeljkoBenovic/tsbc/cmd/destroy"
	"github.com/ZeljkoBenovic/tsbc/cmd/list"
	"github.com/ZeljkoBenovic/tsbc/cmd/recreate"
	"github.com/ZeljkoBenovic/tsbc/cmd/restart"
	"github.com/ZeljkoBenovic/tsbc/cmd/run"
	"github.com/spf13/cobra"
)

// TODO: fix long description
// rootCmd represents the base command when called without any subcommands
var rootCmd = &cobra.Command{
	Use:   "tsbc",
	Short: "TSBC connects your local PBX with MS Teams",
	Long: "TSBC allows the interconnection between the internal PBX system," +
		"that is running on plain old SIP on UDP protoco" +
		"and the MS Teams VoIP platform, which uses SSIP (Secure SIP) on TCP/TLS protocol.",
}

// Execute adds all child commands to the root command and sets flags appropriately.
// This is called by main.main(). It only needs to happen once to the rootCmd.
func Execute() {
	// TODO: add restart and list (sbcs) command
	rootCmd.AddCommand(
		run.GetCmd(),
		destroy.GetCmd(),
		restart.GetCmd(),
		recreate.GetCmd(),
		list.GetCmd(),
	)

	err := rootCmd.Execute()
	if err != nil {
		log.Fatalln(fmt.Sprintf("Could not execute command err=%s", err.Error()))
	}
}

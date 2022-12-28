package run

import (
	"github.com/ZeljkoBenovic/tsbc/sbc"
	"github.com/spf13/cobra"
)

// TODO: fix long description
// runCmd represents the run command
var runCmd = &cobra.Command{
	Use:   "run",
	Short: "A brief description of your command",
	Long: `A longer description that spans multiple lines and likely contains examples
and usage of using your command. For example:

Cobra is a CLI library for Go that empowers applications.
This application is a tool to generate the needed files
to quickly create a Cobra application.`,
	Run: runCommandHandler,
}

func Initialize(rootCmd *cobra.Command) {
	// TODO: add flags and viper config

	rootCmd.AddCommand(runCmd)
}

func runCommandHandler(cmd *cobra.Command, args []string) {
	// TODO: process config and pass it to the sbc instance
	sbcInstance := sbc.NewSBC(sbc.Config{
		DomainName: "sbc.testing.com",
		Port:       "5060",
	})

	sbcInstance.Run()
}

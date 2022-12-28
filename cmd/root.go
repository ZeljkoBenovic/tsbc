package cmd

import (
	"fmt"
	"log"

	"github.com/ZeljkoBenovic/tsbc/cmd/run"
	"github.com/spf13/cobra"
)

// TODO: fix long description
// rootCmd represents the base command when called without any subcommands
var rootCmd = &cobra.Command{
	Use:   "tsbc",
	Short: "TSBC connect your PBX with MS Teams",
	Long: `A longer description that spans multiple lines and likely contains
examples and usage of using your application. For example:

Cobra is a CLI library for Go that empowers applications.
This application is a tool to generate the needed files
to quickly create a Cobra application.`,
}

// Execute adds all child commands to the root command and sets flags appropriately.
// This is called by main.main(). It only needs to happen once to the rootCmd.
func Execute() {
	err := rootCmd.Execute()
	if err != nil {
		log.Fatalln(fmt.Sprintf("Could not execute command err=%s", err.Error()))
	}
}

func init() {
	run.Initialize(rootCmd)
}



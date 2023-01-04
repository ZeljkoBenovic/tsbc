package run

import (
	"log"

	"github.com/ZeljkoBenovic/tsbc/sbc"
	"github.com/spf13/cobra"
	"github.com/spf13/viper"
)

// TODO: fix long description
// runCmd represents the run command
var runCmd = &cobra.Command{
	Use:   "run",
	Short: "run new sbc instance",
	Long: `A longer description that spans multiple lines and likely contains examples
and usage of using your command. For example:

Cobra is a CLI library for Go that empowers applications.
This application is a tool to generate the needed files
to quickly create a Cobra application.`,
	Run: runCommandHandler,
}

func Initialize(rootCmd *cobra.Command) {
	// TODO: create const for flag names
	// general flags
	runCmd.Flags().Bool("fresh", false, "create new database and schema")
	runCmd.Flags().String("sbc-fqdn", "", "fqdn that Kamailio will advertise")
	// kamailio flags
	runCmd.Flags().Bool("kamailio-new-config", true, "generate new config file for Kamailio")
	runCmd.Flags().Bool("kamailio-sip-dump", false, "enable sip capture for Kamailio")
	runCmd.Flags().String("kamailio-sbc-port", "5061", "sbc tls port that will be advertised to MS Teams")
	runCmd.Flags().String("kamailio-udp-sip-port", "5060", "sbc udp port that will be advertised to internal PBX")
	runCmd.Flags().String("kamailio-pbx-ip", "", "ip address of internal PBX")
	runCmd.Flags().String("kamailio-pbx-port", "5060", "sip port of internal PBX")
	runCmd.Flags().String("kamailio-rtpeng-port", "20001", "rtp engine signalisation port")
	runCmd.Flags().String("kamailio-image", "zeljkoiphouse/kamailio:latest", "kamailio docker image name")
	// rtp engine flags
	runCmd.Flags().String("rtp-min-port", "20501", "start port for RTP")
	runCmd.Flags().String("rtp-max-port", "21000", "end port for RTP")
	runCmd.Flags().String("rtp-public-ip", "", "public ip for RTP transport")
	runCmd.Flags().String("rtp-signal-port", "20001", "port used to communicate with Kamailio")
	runCmd.Flags().String("rtp-image", "zeljkoiphouse/rtpengine:latest", "rtp engine docker image name")

	_ = runCmd.MarkFlagRequired("sbc-fqdn")
	_ = runCmd.MarkFlagRequired("rtp-public-ip")
	_ = runCmd.MarkFlagRequired("kamailio-pbx-ip")

	// bind flags to viper
	if err := viper.BindPFlags(runCmd.Flags()); err != nil {
		log.Fatalln("Could not bind to flags err=", err.Error())
	}

	rootCmd.AddCommand(runCmd)
}

func runCommandHandler(cmd *cobra.Command, args []string) {
	// TODO: process config and pass it to the sbc instance
	// create new sbc instance and pass parameters
	sbcInstance, err := sbc.NewSBC(sbc.Config{
		// TODO: handle log level via flags
		LogLevel: "debug",
	})
	if err != nil {
		log.Fatalln("Could not create sbc instance err=", err.Error())
	}

	sbcInstance.Run()
}

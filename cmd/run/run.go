package run

import (
	"log"

	"github.com/ZeljkoBenovic/tsbc/cmd/flagnames"
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

func GetCmd() *cobra.Command {
	// general flags
	runCmd.Flags().String(flagnames.SbcFqdn, "", "fqdn that Kamailio will advertise")
	runCmd.Flags().String(flagnames.HostIP, "", "the static lan ip address of the docker host")
	runCmd.Flags().String(flagnames.LogLevel, "info", "log output level")
	// kamailio flags
	runCmd.Flags().Bool(flagnames.KamailioNewConfig, true, "generate new config file for Kamailio")
	runCmd.Flags().Bool(flagnames.KamailioSipDump, false, "enable sip capture for Kamailio")
	runCmd.Flags().String(flagnames.KamailioSbcPort, "5061", "sbc tls port that will be advertised to MS Teams")
	runCmd.Flags().String(flagnames.KamailioUdpSipPort, "5060", "sbc udp port that will be advertised to internal PBX")
	runCmd.Flags().String(flagnames.KamailioPbxIp, "", "ip address of internal PBX")
	runCmd.Flags().String(flagnames.KamailioPbxPort, "5060", "sip port of internal PBX")
	runCmd.Flags().String(flagnames.KamailioRtpEngPort, "20001", "rtp engine signalisation port")
	runCmd.Flags().String(flagnames.KamailioImage, "zeljkoiphouse/kamailio:v0.2", "kamailio docker image name")
	// rtp engine flags
	runCmd.Flags().String(flagnames.RtpMinPort, "20501", "start port for RTP")
	runCmd.Flags().String(flagnames.RtpMaxPort, "21000", "end port for RTP")
	runCmd.Flags().String(flagnames.RtpPublicIp, "", "public ip for RTP transport")
	runCmd.Flags().String(flagnames.RtpSignalPort, "20001", "port used to communicate with Kamailio")
	runCmd.Flags().String(flagnames.RtpImage, "zeljkoiphouse/rtpengine:latest", "rtp engine docker image name")

	_ = runCmd.MarkFlagRequired(flagnames.SbcFqdn)
	_ = runCmd.MarkFlagRequired(flagnames.RtpPublicIp)
	_ = runCmd.MarkFlagRequired(flagnames.KamailioPbxIp)
	_ = runCmd.MarkFlagRequired(flagnames.HostIP)

	// bind flags to viper
	if err := viper.BindPFlags(runCmd.Flags()); err != nil {
		log.Fatalln("Could not bind to flags err=", err.Error())
	}

	return runCmd
}

func runCommandHandler(cmd *cobra.Command, args []string) {
	// create new sbc instance and pass parameters
	sbcInstance, err := sbc.NewSBC()
	if err != nil {
		log.Fatalln("Could not create sbc instance err=", err.Error())
	}

	sbcInstance.Run()
}

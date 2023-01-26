package run

import (
	"fmt"
	"log"

	"github.com/ZeljkoBenovic/tsbc/cmd/helpers/flagnames"
	"github.com/ZeljkoBenovic/tsbc/db"
	"github.com/ZeljkoBenovic/tsbc/sbc"
	"github.com/spf13/cobra"
	"github.com/spf13/viper"
)

// runCmd represents the run command
var runCmd = &cobra.Command{
	Use:     "run",
	Short:   "Command used to deploy a new SBC cluster",
	Run:     runCommandHandler,
	Example: "tsbc run --kamailio-pbx-ip 192.168.1.1 --sbc-fqdn sbc.test1.com --rtp-public-ip 1.1.1.1  --host-ip 192.168.10.1",
}

func GetCmd() *cobra.Command {
	// general flags
	runCmd.Flags().String(flagnames.SbcFqdn, "", "fqdn that Kamailio will advertise")
	runCmd.Flags().String(flagnames.HostIP, "", "the static lan ip address of the docker host")
	runCmd.Flags().String(flagnames.LogLevel, "info", "log output level")
	runCmd.Flags().String(flagnames.LogFileLocation, "", "log file location")
	runCmd.Flags().String(flagnames.DockerLogFileLocation, "/var/log/tsbc/docker.log", "docker log file location")
	runCmd.Flags().String(flagnames.DBFileLocation, "",
		fmt.Sprintf("sqlite file location, file name must end with .db (default: %s)", db.DefaultDBLocation()))
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
	// letsencrypt flags
	runCmd.Flags().String(flagnames.Timezone, "Europe/Belgrade", "set the timezone")
	runCmd.Flags().String(flagnames.Staging, "false", "set staging environment for LetsEncrypt node")

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

	defer sbcInstance.Close()

	sbcInstance.Run()
}

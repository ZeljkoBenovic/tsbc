package list

import (
	"bytes"
	"fmt"

	"github.com/ZeljkoBenovic/tsbc/sbc"
	"github.com/ZeljkoBenovic/tsbc/sbc/types"
	"github.com/fatih/color"
	"github.com/hashicorp/go-hclog"
	"github.com/rodaine/table"
	"github.com/spf13/cobra"
)

var listCmd = &cobra.Command{
	Use:     "list",
	Short:   "Get a list of all the deployed SBCs",
	Example: "tsbc list",
	Run:     runListCommand,
}

func GetCmd() *cobra.Command {
	return listCmd
}

func runListCommand(_ *cobra.Command, _ []string) {
	hlog := hclog.New(&hclog.LoggerOptions{
		Name:                 "list",
		Color:                hclog.AutoColor,
		ColorHeaderAndFields: true,
	})

	sbcInst, err := sbc.NewSBC()
	if err != nil {
		hlog.Error("Could not create new sbc instance", "err", err)

		return
	}

	sbcsInfo, err := sbcInst.List()
	if err != nil {
		hlog.Error("Could not get a list of SBCs", "err", err)
	}

	displaySBCInformation(sbcsInfo)
}

func displaySBCInformation(sbcs []types.Sbc) {
	var buff bytes.Buffer

	if len(sbcs) == 0 {
		buff.WriteString("===========================\n")
		buff.WriteString("==NO SBCs INSTANCES FOUND==\n")
		buff.WriteString("===========================\n")

		fmt.Print(buff.String())

		return
	}

	buff.WriteString("[SBC INFORMATION]\n")
	headerFmt := color.New(color.FgGreen, color.Underline).SprintfFunc()
	columnFmt := color.New(color.FgYellow).SprintfFunc()

	tbl := table.New("FQDN", "TLS_PORT", "UDP_PORT", "PBX_IP",
		"PBX_PORT", "PUBLIC_IP", "RTP_MIN", "RTP_MAX")
	tbl.WithHeaderFormatter(headerFmt).WithFirstColumnFormatter(columnFmt)

	for _, sbcInfo := range sbcs {
		tbl.AddRow(sbcInfo.SbcName, sbcInfo.SbcTLSPort, sbcInfo.SbcUDPPort, sbcInfo.PbxIP,
			sbcInfo.PbxPort, sbcInfo.MediaPublicIP, sbcInfo.RtpMinPort, sbcInfo.RtpMaxPort)
	}

	tbl.Print()
}

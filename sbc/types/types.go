package types

type Sbc struct {
	Fqdn string
	Kamailio
	RtpEngine

	LogFileLocation       string
	DockerLogFileLocation string
	SQLiteFileLocation    string
}

type Kamailio struct {
	NewConfig     bool
	EnableSipDump bool
	SbcName       string
	SbcTLSPort    string
	SbcUDPPort    string
	PbxIP         string
	PbxPort       string
	RtpEnginePort string
}

type RtpEngine struct {
	RtpMaxPort    string
	RtpMinPort    string
	MediaPublicIP string
	NgListen      string
}

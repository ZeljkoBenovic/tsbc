package types

type Sbc struct {
	Fqdn string
	Kamailio
	RTPEngine

	LogFileLocation       string
	DockerLogFileLocation string
	SQLiteFileLocation    string
}

type Kamailio struct {
	NewConfig     bool
	EnableSIPDump bool
	SbcName       string
	SbcTLSPort    string
	SbcUDPPort    string
	PbxIP         string
	PbxPort       string
	RTPEnginePort string
}

type RTPEngine struct {
	RTPMaxPort    string
	RTPMinPort    string
	MediaPublicIP string
	NgListen      string
}

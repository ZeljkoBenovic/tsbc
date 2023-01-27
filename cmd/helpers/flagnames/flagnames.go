package flagnames

// define flag names as consts
const (
	SbcFqdn  string = "sbc-fqdn"
	HostIP   string = "host-ip"
	Timezone string = "timezone"
	Staging  string = "staging"

	DestroyTLSNode string = "tls-node"

	LogLevel              string = "log-level"
	LogFileLocation       string = "log-file"
	DockerLogFileLocation string = "docker-log"
	DBFileLocation        string = "db-file"

	KamailioNewConfig  string = "kamailio-new-config"
	KamailioSIPDump    string = "kamailio-sip-dump"
	KamailioSbcPort    string = "kamailio-sbc-port"
	KamailioUDPSIPPort string = "kamailio-udp-sip-port"
	KamailioPbxIP      string = "kamailio-pbx-ip"
	KamailioPbxPort    string = "kamailio-pbx-port"
	KamailioRTPEngPort string = "kamailio-rtpeng-port"
	KamailioImage      string = "kamailio-image"

	RTPMinPort    string = "rtp-min-port"
	RTPMaxPort    string = "rtp-max-port"
	RTPPublicIP   string = "rtp-public-ip"
	RTPSignalPort string = "rtp-signal-port"
	RTPImage      string = "rtp-image"
)

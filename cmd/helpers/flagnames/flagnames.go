package flagnames

// define flag names as consts
const (
	SbcFqdn  string = "sbc-fqdn"
	HostIP   string = "host-ip"
	Timezone string = "timezone"
	Staging  string = "staging"

	DestroyTlsNode string = "tls-node"

	LogLevel              string = "log-level"
	LogFileLocation       string = "log-file"
	DockerLogFileLocation string = "docker-log"
	DBFileLocation        string = "db-file"

	KamailioNewConfig  string = "kamailio-new-config"
	KamailioSipDump    string = "kamailio-sip-dump"
	KamailioSbcPort    string = "kamailio-sbc-port"
	KamailioUdpSipPort string = "kamailio-udp-sip-port"
	KamailioPbxIp      string = "kamailio-pbx-ip"
	KamailioPbxPort    string = "kamailio-pbx-port"
	KamailioRtpEngPort string = "kamailio-rtpeng-port"
	KamailioImage      string = "kamailio-image"

	RtpMinPort    string = "rtp-min-port"
	RtpMaxPort    string = "rtp-max-port"
	RtpPublicIp   string = "rtp-public-ip"
	RtpSignalPort string = "rtp-signal-port"
	RtpImage      string = "rtp-image"
)

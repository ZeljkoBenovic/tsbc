package flagnames

// define flag names as consts
// needs to be separate package to avoid recursive import error
const (
	LogLevel string = "log-level"
	SbcFqdn  string = "sbc-fqdn"
	HostIP   string = "host-ip"

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

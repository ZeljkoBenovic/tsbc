# MS Teams Session Border Controller (TSBC)

TSBC allows the interconnection between the internal PBX system, that is running on plain old SIP on UDP protocol 
and the MS Teams VoIP platform, which uses SSIP (Secure SIP) on TCP/TLS protocol.

Interconnecting the local PBX system and MS Teams platform, requires the implementation of a dedicated local 
SBC device, which sits between MS Teams VoIP platform and local PBX, which can be quite costly.    
Other solution is to expose local PBX system to the public world, which is not considered the best practise 
security wise. Even if the PBX is exposed to the public, being able to communicate with MS Teams platform, 
it would still require changing some SIP headers to comply with MS Teams security specifications. 
Usually, these local PBX systems like Asterisk do not have the capability to manipulate SIP headers in such 
a specific fashion.

TSBC connects your local PBX (any SIP compatible PBX) with MS Teams voice platform.  
It sits between MS Teams and local PBX, translating SIP/RTP traffic. On MS Teams side SSIP/TLS and on the 
other, local, SIP/UDP traffic.

## Prerequisites

* Docker `>= 20.10.17`

## Deployed infrastructure

TSBC deploys two (or three) docker containers.   
* `zeljkoiphouse/kamailio:v0.2` - [Kamailio](https://www.kamailio.org/w/) based container which handles
  all the SIP signalisation traffic between local PBX and MS Teams VoIP platform.
* `zeljkoiphouse/rtpengine` - [RTPEngine](https://github.com/sipwise/rtpengine) based container which handles 
  all the RTP (media) traffic.
* `linuxserver/swag` - container that handles TLS certificates utilising LetsEncrypt service.
  There will always be only one container per docker host.


## Command usage

* [tsbc](docs/cmd_usage/tsbc.md)- TSBC root level command
* [tsbc destroy](docs/cmd_usage/tsbc_destroy.md)	 - Destroy SBC cluster or TLS node
* [tsbc list](docs/cmd_usage/tsbc_list.md)	 - Get a list of all the deployed SBCs
* [tsbc recreate](docs/cmd_usage/tsbc_recreate.md)	 - Command used to recreate SBC nodes
* [tsbc restart](docs/cmd_usage/tsbc_restart.md)	 - Command used to restart SBC nodes
* [tsbc run](docs/cmd_usage/tsbc_run.md)	 - Command used to deploy a new SBC cluster

## Docker host requirements
* All traffic from MS Teams platform IP 
[addresses](https://learn.microsoft.com/en-us/microsoftteams/direct-routing-plan#microsoft-365-office-365-and-office-365-gcc-environments) 
forwarded to docker host.
* DNS name for the SBC tied to the public IP address of docker host
* Ports `tcp/80` and `tcp/443` forwarded to docker host as they are needed for certificate verification  
* Local `PBX` and `TSBC` host, directly reachable on the IP level (same LAN or routed) 

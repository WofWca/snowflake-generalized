package main

import (
	"flag"
	"log"
	"net"
	"os"
	"strings"

	"github.com/WofWca/snowflake-generalized/common"
	safelog "gitlab.torproject.org/tpo/anti-censorship/pluggable-transports/ptutil/safelog"
	snowflakeClient "gitlab.torproject.org/tpo/anti-censorship/pluggable-transports/snowflake/v2/client/lib"
)

func main() {
	// For the list of parameters of the original client, see
	// - https://gitlab.torproject.org/tpo/anti-censorship/pluggable-transports/snowflake/-/blob/97e21e3a29f8dd8306ed893a8341ce91846b02f7/client/snowflake.go#L166-179
	// - https://gitlab.torproject.org/tpo/anti-censorship/pluggable-transports/snowflake/-/blob/97e21e3a29f8dd8306ed893a8341ce91846b02f7/client/snowflake.go#L80-128

	// TODO maybe spin up a public broker?
	brokerURL := flag.String("broker-url", "", "URL of signaling broker")
	serverId := flag.String(
		"server-id",
		"AAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAA",
		"40 hex character server ID to which to forward the connections",
	)
	// It's not yet possible to ask the broker to connect to a server by its URL
	// directly. There is an MR for this.
	// serverUrl := flag.String("server-url", "", "Server URL")
	listenAddr := flag.String(
		"listen-address",
		"localhost:2080",
		"Listen for application connections on this `address` and forward them to the server",
	)
	// noTCP := flag.Bool("no-tcp", false)
	// noUDP := flag.Bool("no-udp", false)
	// TODO also add socket file support or something like that.
	// https://gitlab.torproject.org/tpo/anti-censorship/pluggable-transports/snowflake/-/issues/40131
	// ssh appears to have something like this.

	iceServersCommas := flag.String(
		"ice",
		// Copy-pasted from
		// https://gitlab.torproject.org/tpo/anti-censorship/pluggable-transports/snowflake/-/blob/98db63ad01d9d78b8cd8aad77219a3d900bfdfef/client/README.md#L38
		"stun:stun.l.google.com:19302,stun:stun.antisip.com:3478,stun:stun.bluesip.net:3478,stun:stun.dus.net:3478,stun:stun.epygi.com:3478,stun:stun.sonetel.com:3478,stun:stun.uls.co.za:3478,stun:stun.voipgate.com:3478,stun:stun.voys.nl:3478",
		"comma-separated list of ICE servers",
	)

	frontDomainsCommas := flag.String("fronts", "", "comma-separated list of front domains")
	ampCacheURL := flag.String("ampcache", "", "URL of AMP cache to use as a proxy for signaling")
	sqsQueueURL := flag.String("sqsqueue", "", "URL of SQS Queue to use as a proxy for signaling")
	sqsCredsStr := flag.String("sqscreds", "", "credentials to access SQS Queue")

	utlsNoSni := flag.Bool("utls-nosni", false, "remove SNI from Client Hello")
	utlsImitate := flag.String(
		"utls-imitate",
		"",
		"imitate TLS client hello fingerprint of other client.\nPossible values: https://gitlab.torproject.org/tpo/anti-censorship/pluggable-transports/snowflake/-/blob/33248f3dec5594c985cfd11e6c6143ddaa5613c0/common/utls/client_hello_id.go#L13-27",
	)

	// logFilename := flag.String("log", "", "name of log file")
	keepLocalAddresses := flag.Bool(
		"keep-local-addresses",
		false,
		"keep local LAN address ICE candidates",
	)
	unsafeLogging := flag.Bool("unsafe-logging", false, "prevent logs from being scrubbed")
	max := flag.Int("max", 1,
		"capacity for number of multiplexed WebRTC peers")
	// versionFlag := flag.Bool("version", false, "display version info to stderr and quit")
	flag.Parse()

	if *brokerURL == "" {
		flag.Usage()
		log.Fatal("\"broker-url\" must be specified because the default broker only supports Tor relays.\nSee https://gitlab.torproject.org/tpo/anti-censorship/pluggable-transports/snowflake/-/issues/40166")
	}

	listener, err := net.Listen("tcp", *listenAddr)
	if err != nil {
		log.Fatalf("Failed to listen on \"%v\": %v", *listenAddr, err)
	}

	var frontDomains []string
	if *frontDomainsCommas != "" {
		frontDomains = strings.Split(strings.TrimSpace(*frontDomainsCommas), ",")
	}

	config := snowflakeClient.ClientConfig{
		BrokerURL:         *brokerURL,
		BridgeFingerprint: *serverId,
		// RelayURL: relayURL,

		FrontDomains: frontDomains,
		AmpCacheURL:  *ampCacheURL,
		SQSQueueURL:  *sqsQueueURL,
		SQSCredsStr:  *sqsCredsStr,

		ICEAddresses:       strings.Split(strings.TrimSpace(*iceServersCommas), ","),
		KeepLocalAddresses: *keepLocalAddresses,
		Max:                *max,

		UTLSRemoveSNI: *utlsNoSni,
		UTLSClientID:  *utlsImitate,
	}
	snowflakeClientTransport, err := snowflakeClient.NewSnowflakeClient(config)
	if err != nil {
		log.Fatal("Failed to start snowflake transport: ", err)
	}

	// Setting scrubber _after_ initial checks
	// so that addresses are printed properly.
	logOutput := os.Stdout
	if *unsafeLogging {
		log.SetOutput(logOutput)
	} else {
		log.SetOutput(&safelog.LogScrubber{Output: logOutput})
	}

	// Also UDP in the future
	log.Printf("Forwarding TCP connections to \"%v\" to server \"%v\"", *listenAddr, *serverId)
	acceptLoop(listener, snowflakeClientTransport)
}

func acceptLoop(ln net.Listener, snowflakeClientTransport *snowflakeClient.Transport) {
	for {
		netConn, err := ln.Accept()
		if err != nil {
			if err, ok := err.(net.Error); ok && err.Temporary() {
				continue
			}
			log.Print("Failed to accept connection", err)
			// TODO is this what we want? This will terminate the client.
			break
		}
		log.Print("Got new connection! Forwarding")

		go func() {
			// TODO perf: making a new Snowflake proxy connection
			// per each TCP connection is not great.
			// E.g. for the case of SOCKS proxy. E.g. a browser SOCKS client
			// makes a new TCP connection per nearly every HTTP request,
			// so each request would be delayed by the amount of time
			// it takes to get a new Snowflake proxy.
			snowflakeClientConn, err := snowflakeClientTransport.Dial()
			if err != nil {
				log.Print("Snowflake dial failed", err)
				netConn.Close()
				return
			}

			// TODO should we utilize `shutdownChan`?
			shutdownChan := make(chan struct{})
			common.CopyLoop(snowflakeClientConn, netConn, shutdownChan)
		}()
	}
}

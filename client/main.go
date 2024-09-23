package main

import (
	"flag"
	"log"
	"net"
	"os"
	"strings"

	"github.com/WofWca/snowflake-generalized/common"
	pionUDP "github.com/pion/transport/v3/udp"
	"github.com/xtaci/smux"
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
	destinationProtocol := flag.String(
		"destination-protocol",
		"tcp",
		"what type of packets to forward, i.e. what protocol the target "+
			"application (WireGuard, SOCKS server) is using, \"udp\" or \"tcp\".\n"+
			"This value must be the same on the target server",
	)
	// noTCP := flag.Bool("no-tcp", false)
	// noUDP := flag.Bool("no-udp", false)
	// TODO also add socket file support or something like that.
	// https://gitlab.torproject.org/tpo/anti-censorship/pluggable-transports/snowflake/-/issues/40131
	// ssh appears to have something like this.
	//
	// TODO perf: in UDP mode, make the client-server connection
	// unreliable and unordered. When
	// https://gitlab.torproject.org/tpo/anti-censorship/pluggable-transports/snowflake/-/issues/40352
	// is done.

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

	var listener net.Listener
	switch *destinationProtocol {
	case "tcp":
		listenAddrStruct, err := net.ResolveTCPAddr("tcp", *listenAddr)
		if err != nil {
			log.Fatal(err)
		}
		listener, err = net.ListenTCP("tcp", listenAddrStruct)
		if err != nil {
			log.Fatalf(
				"Failed to listen on \"%v\" %v: %v",
				*listenAddr,
				destinationProtocol,
				err,
			)
		}
	case "udp":
		listenAddrStruct, err := net.ResolveUDPAddr("udp", *listenAddr)
		if err != nil {
			log.Fatal(err)
		}
		listener, err = pionUDP.Listen("udp", listenAddrStruct)
		if err != nil {
			log.Fatalf(
				"Failed to listen on \"%v\" %v: %v",
				*listenAddr,
				destinationProtocol,
				err,
			)
		}
	default:
		log.Fatal("`destination-protocol` parameter value must either be \"tcp\" or \"udp\"")
	}

	var frontDomains []string
	if *frontDomainsCommas != "" {
		frontDomains = strings.Split(strings.TrimSpace(*frontDomainsCommas), ",")
	}

	// Setting scrubber _after_ initial checks
	// so that addresses are printed properly.
	logOutput := os.Stdout
	if *unsafeLogging {
		log.SetOutput(logOutput)
	} else {
		log.SetOutput(&safelog.LogScrubber{Output: logOutput})
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
	snowflakeClientConn, err := snowflakeClientTransport.Dial()
	if err != nil {
		// TODO should we retry?
		log.Fatal("Snowflake dial failed", err)
	}

	// Why use a multiplexer instead of `snowflakeClientTransport.Dial()`-ing
	// per each TCP connection?
	// Firstly, connecting to a new proxy takes some seconds
	// (sometimes minutes!).
	// This is not great if we expect to receive connections frequently.
	// E.g. for the case of SOCKS proxy. A browser SOCKS client
	// makes a new TCP connection per nearly every HTTP request,
	// so each request would be delayed by the amount of time
	// it takes to get a new Snowflake proxy.
	//
	// Secondly, apparently the previous `Dial()` gets discarded
	// and connection lost.
	//
	// https://github.com/xtaci/smux?tab=readme-ov-file#usage
	//
	// TODO perf: Snowflake already uses smux internally.
	// Can we maybe modify the library so that it exposes the session
	// so we can use it?
	// https://gitlab.torproject.org/tpo/anti-censorship/pluggable-transports/snowflake/-/blob/bf116939935b0a2ae2adf4f5976c349aae96e48b/client/lib/snowflake.go#L211-212

	smuxConfig := smux.DefaultConfig()
	// Connecting with Snowflake might take some minutes sometimes.
	// Let's not close the connection on our own, and let Snowflake handle that.
	//
	// TODO we probably don't want to terminate the client at all and
	// just keep retrying.
	smuxConfig.KeepAliveDisabled = true

	snowflakeClientMuxSession, err := smux.Client(snowflakeClientConn, smuxConfig)
	if err != nil {
		log.Fatal(err)
	}

	log.Printf(
		"Forwarding %v connections to \"%v\" to server \"%v\"",
		*destinationProtocol,
		*listenAddr,
		*serverId,
	)
	acceptLoop(listener, snowflakeClientMuxSession)
}

func acceptLoop(ln net.Listener, snowflakeClientMuxSession *smux.Session) {
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
			defer netConn.Close()
			// TODO handle errors carefully.
			// E.g. there is `ErrGoAway` which occurs when stream IDs
			// get exhausted, and when that happens,
			// we can never open a new stream, which means that
			// we probably need to recreate a smux session.
			snowflakeStream, err := snowflakeClientMuxSession.OpenStream()
			if err != nil {
				log.Print("smux.OpenStream() failed: ", err)
				return
			}
			defer snowflakeStream.Close()

			// TODO should we utilize `shutdownChan`?
			shutdownChan := make(chan struct{})
			common.CopyLoop(snowflakeStream, netConn, shutdownChan)
		}()
	}
}

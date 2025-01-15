package main

import (
	"flag"
	"fmt"
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
		"",
		"40 hex character server ID to which to forward the connections."+
			"See also \"server-url\".",
	)
	serverUrl := flag.String(
		"server-url",
		"",
		"Server `URL` to which to forward the connections",
	)

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
	singleConnMode := flag.Bool(
		"single-connection-mode",
		false,
		"If you want to do only a single TCP / UDP"+
			" connection to the destination"+
			" (e.g. if the destination server is a WireGuard"+
			" or an OpenVPN server), you can toggle this flag on."+
			"\nIt turns off multiplexing, and thus it _might_"+
			" improve connection performance."+
			"\nThe value of this flag must be the same for both"+
			" the server and the client.",
	)

	serverIsOldVersion := flag.Bool(
		"server-is-old-version",
		false,
		"Prior to 2025-01-13, we used smux version 1 instead of the latest 2."+
			" If the server is of that older version, use this flag."+
			"\nThis flag has no effect when using single-connection-mode",
	)

	iceServersCommas := flag.String(
		"ice",
		// Copy-pasted from
		// curl https://gitlab.torproject.org/tpo/applications/tor-browser-build/-/raw/main/projects/tor-expert-bundle/pt_config.json | grep "ice=.* "
		"stun:stun.antisip.com:3478,stun:stun.epygi.com:3478,stun:stun.uls.co.za:3478,stun:stun.voipgate.com:3478,stun:stun.mixvoip.com:3478,stun:stun.nextcloud.com:3478,stun:stun.bethesda.net:3478,stun:stun.nextcloud.com:443",
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

	if *serverUrl == "" && *serverId == "" {
		flag.Usage()
		log.Fatal("Specify \"server-url\" or \"server-id\"")
	} else if *serverUrl != "" && *serverId != "" {
		flag.Usage()
		log.Fatal("Don't specify both \"server-url\" and \"server-id\"")
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
		RelayURL:          *serverUrl,

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

	var idOrUrlString string
	if *serverUrl != "" {
		idOrUrlString = *serverUrl
	} else {
		idOrUrlString = fmt.Sprintf("with ID %v", serverId)
	}
	log.Printf(
		"Forwarding %v connections to \"%v\" to server %v",
		*destinationProtocol,
		*listenAddr,
		idOrUrlString,
	)

	if *singleConnMode {
		for {
			err := serveOneConnInSingleConnMode(listener, snowflakeClientTransport)
			if err != nil {
				return
			}
		}
	} else {
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
		if *serverIsOldVersion {
			smuxConfig.Version = 1
		} else {
			// This seems to ~double TCP connection speed, as of 2025-01-13.
			// Or so did it look based on a single test.
			smuxConfig.Version = 2
		}
		// Connecting with Snowflake might take some minutes sometimes.
		// Let's not close the connection on our own, and let Snowflake handle that.
		//
		// TODO we probably don't want to terminate the client at all and
		// just keep retrying.
		smuxConfig.KeepAliveDisabled = true
		// This seems to increase download speed by about x2,
		// at least for the SOCKS example, based on eyeball tests.
		// See https://gitlab.torproject.org/tpo/anti-censorship/pluggable-transports/snowflake/-/merge_requests/48
		smuxConfig.MaxStreamBuffer = snowflakeClient.StreamSize

		snowflakeClientMuxSession, err := smux.Client(snowflakeClientConn, smuxConfig)
		if err != nil {
			log.Fatal(err)
		}

		muxModeAcceptLoop(listener, snowflakeClientMuxSession)
	}
}

func muxModeAcceptLoop(
	ln net.Listener,
	snowflakeClientMuxSession *smux.Session,
) {
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

// If an error is returned, this function should not be called another time.
func serveOneConnInSingleConnMode(
	ln net.Listener,
	snowflakeClientTransport *snowflakeClient.Transport,
) error {
	snowflakeClientConn, err := snowflakeClientTransport.Dial()
	if err != nil {
		// TODO should we retry? With a timeout?
		log.Print("Snowflake dial failed", err)
		return err
	}
	defer snowflakeClientConn.Close()
	// TODO it looks like the connection doesn't actually get fully closed.
	// You can reproduce by doing a bunch of
	// `curl localhost:2080` + Ctrl + C.
	// The client will be printing `Traffic Bytes (in|out)`
	// a lot more frequently.
	// Maybe we really need to create a new `snowflakeClientTransport`
	// for each `ln.Accept()`.

	netConn, err := ln.Accept()
	if err != nil {
		log.Print("Failed to accept connection", err)
		if err, ok := err.(net.Error); ok && err.Temporary() {
			return nil
		}
		// TODO is this what we want? This will terminate the client.
		return err
	}
	defer netConn.Close()
	log.Print("Got new connection! Forwarding")

	// Perhaps instead of blocking here we could make a new
	// Snowflake client connection per each network connection,
	// instead of never doing `ln.Accept()`.
	// Though remember that apparently `snowflakeClientTransport.Dial()`
	// closes all the previous `snowflakeClientConn` for the instance of
	// `snowflakeClientTransport`,
	// so a new `snowflakeClientTransport` needs to be created every time.

	// TODO should we utilize `shutdownChan`?
	shutdownChan := make(chan struct{})
	common.CopyLoop(snowflakeClientConn, netConn, shutdownChan)

	return nil
}

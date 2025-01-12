package main

import (
	"flag"
	"io"
	"log"
	"net"
	"net/http"
	"os"
	"strings"

	"github.com/WofWca/snowflake-generalized/common"
	"github.com/xtaci/smux"
	"gitlab.torproject.org/tpo/anti-censorship/pluggable-transports/ptutil/safelog"
	snowflakeServer "gitlab.torproject.org/tpo/anti-censorship/pluggable-transports/snowflake/v2/server/lib"
	"golang.org/x/crypto/acme/autocert"
)

func main() {
	var listenAddr string
	var destinationAddr string
	var destinationProtocol string
	var singleConnMode bool
	var acmeEmail string
	var acmeHostnamesCommas string
	var acmeCertCacheDir string
	var disableTLS bool
	// var logFilename string
	var unsafeLogging bool
	// var versionFlag bool

	// For the original Snowflake server CLI parameters, see
	// https://gitlab.torproject.org/tpo/anti-censorship/pluggable-transports/snowflake/-/blob/6d2011ded71dc53662fa0f256fbf9c3036c474a4/server/server.go#L139-144

	flag.StringVar(
		&listenAddr,
		"listen-address",
		"localhost:7901",
		"Listen for proxy connections on this `address` and forward them to \"destination-addr\".\nSet to \":7901\" to listen on port 7901 on all interfaces.",
	)
	flag.StringVar(
		&destinationAddr,
		"destination-address",
		"", // "localhost:1080", we probably should not have a default address for security reasons
		"Forward client connections to this `address`.\nThis can also be a remote address.",
	)
	flag.StringVar(
		&destinationProtocol,
		"destination-protocol",
		"tcp",
		"what type of packets to send to the destination, "+
			"i.e. what protocol the target application (WireGuard, SOCKS server) "+
			" is using, \"udp\" or \"tcp\".\n",
	)
	// TODO feat: ideally we'd want to support both modes simultaneously,
	// and let the client specify the mode when it connects,
	// e.g. with a WebSocket URL query param.
	flag.BoolVar(
		&singleConnMode,
		"single-connection-mode",
		false,
		"If each Snowflake client connection makes only a single TCP / UDP"+
			" connection to destination-address"+
			" (e.g. if the destination server is a WireGuard"+
			" or an OpenVPN server), you can toggle this flag on."+
			"\nIt turns off multiplexing, and thus it _might_"+
			" improve connection performance."+
			"\nThe value of this flag must be the same for both"+
			" the server and the client.",
	)
	flag.StringVar(&acmeEmail, "acme-email", "", "optional contact email for Let's Encrypt notifications")
	flag.StringVar(&acmeHostnamesCommas, "acme-hostnames", "", "comma-separated hostnames for TLS certificate")
	flag.StringVar(&acmeCertCacheDir, "acme-cert-cache", "acme-cert-cache", "directory in which certificates should be cached")
	flag.BoolVar(&disableTLS, "disable-tls", false, "don't use HTTPS")
	// flag.StringVar(&logFilename, "log", "", "log file to write to")
	flag.BoolVar(&unsafeLogging, "unsafe-logging", false, "prevent logs from being scrubbed")
	// flag.BoolVar(&versionFlag, "version", false, "display version info to stderr and quit")
	flag.Parse()

	if destinationProtocol != "tcp" && destinationProtocol != "udp" {
		log.Fatal("`destination-protocol` must either be \"tcp\" or \"udp\"")
	}

	if destinationAddr == "" {
		flag.Usage()
		log.Fatalf("\"destination-address\" must be specified")
	}
	listenAddrStruct, err := net.ResolveTCPAddr("tcp", listenAddr)
	if err != nil {
		log.Fatalf("error resolving listen address: %s", err.Error())
	}

	// var certManager *autocert.Manager = nil
	var transport *snowflakeServer.Transport
	if !disableTLS {
		if acmeHostnamesCommas == "" {
			log.Fatal("the --acme-hostnames option is required, unless --disable-tls")
		}

		acmeHostnames := strings.Split(acmeHostnamesCommas, ",")
		log.Printf("ACME hostnames: %q", acmeHostnames)

		var cache autocert.Cache
		if acmeCertCacheDir != "" {
			log.Printf("caching ACME certificates in directory %q", acmeCertCacheDir)
			cache = autocert.DirCache(acmeCertCacheDir)
		} else {
			log.Printf("disabling ACME certificate cache: %s", err)
		}

		certManager := autocert.Manager{
			Cache:      cache,
			Prompt:     autocert.AcceptTOS,
			HostPolicy: autocert.HostWhitelist(acmeHostnames...),
			Email:      acmeEmail,
		}
		go func() {
			log.Printf("Starting HTTP-01 listener")
			log.Fatal(http.ListenAndServe(":80", certManager.HTTPHandler(nil)))
		}()

		transport = snowflakeServer.NewSnowflakeServer(certManager.GetCertificate)
	} else {
		transport = snowflakeServer.NewSnowflakeServer(nil)
	}

	numKCPInstances := 1
	ln, err := transport.Listen(listenAddrStruct, numKCPInstances)
	if err != nil {
		log.Fatalf("error opening listener: %s", err.Error())
	}

	log.Printf(
		"Listening for proxy connections on %v \"%v\" and forwarding them to \"%v\"",
		destinationProtocol,
		listenAddrStruct,
		destinationAddr,
	)

	// Setting scrubber _after_ initial checks
	// so that addresses are printed properly.
	logOutput := os.Stdout
	if unsafeLogging {
		log.SetOutput(logOutput)
	} else {
		log.SetOutput(&safelog.LogScrubber{Output: logOutput})
	}

	for {
		clientConn, err := ln.Accept()
		if err != nil {
			if err, ok := err.(net.Error); ok && err.Temporary() {
				continue
			}
			log.Printf("Failed to accept proxy connection: %s", err)
			// This will terminate the server.
			// The original Snowflake server does the same:
			// https://gitlab.torproject.org/tpo/anti-censorship/pluggable-transports/snowflake/-/blob/6d2011ded71dc53662fa0f256fbf9c3036c474a4/server/server.go#L99-111
			break
		}
		log.Printf(
			"Got Snowflake client connection! Forwarding to %v \"%v\"",
			destinationProtocol,
			destinationAddr,
		)

		if singleConnMode {
			go serveSnowflakeConnectionInSingleConnMode(
				&clientConn,
				&destinationAddr,
				&destinationProtocol,
			)
		} else {
			go serveSnowflakeConnectionInMuxMode(
				&clientConn,
				&destinationAddr,
				&destinationProtocol,
			)
		}
	}
}

// Closes the connection when it finishes serving it.
func serveSnowflakeConnectionInMuxMode(
	snowflakeConn *net.Conn,
	destinationAddr *string,
	destinationProtocol *string,
) {
	defer (*snowflakeConn).Close()

	smuxConfig := smux.DefaultConfig()
	// Let's not close the connection on our own, and let Snowflake handle that.
	smuxConfig.KeepAliveDisabled = true
	// See https://gitlab.torproject.org/tpo/anti-censorship/pluggable-transports/snowflake/-/merge_requests/48
	// and the similar line in client/main.go
	smuxConfig.MaxStreamBuffer = snowflakeServer.StreamSize

	muxSession, err := smux.Server(*snowflakeConn, smuxConfig)
	if err != nil {
		log.Print("Mux session open error", err)
		return
	}
	defer muxSession.Close()

	for {
		stream, err := muxSession.AcceptStream()
		if err != nil {
			// Otherwise it's a regular connection close
			// TODO or is it? There is `ErrTimeout`?
			if err != io.ErrClosedPipe {
				log.Print("AcceptStream error", err)
			}
			return
		}
		log.Print("New stream!", stream.ID())

		go func() {
			defer stream.Close()
			destinationConn, err := net.Dial(*destinationProtocol, *destinationAddr)
			if err != nil {
				log.Print("Failed to dial destination address", err)
				// Hmm should we also snowflakeConn.Close()
				return
			}
			defer destinationConn.Close()

			// TODO should we utilize `shutdownChan`?
			shutdownChan := make(chan struct{})
			common.CopyLoop(stream, destinationConn, shutdownChan)
		}()
	}
}

func serveSnowflakeConnectionInSingleConnMode(
	snowflakeConn *net.Conn,
	destinationAddr *string,
	destinationProtocol *string,
) {
	defer (*snowflakeConn).Close()

	destinationConn, err := net.Dial(*destinationProtocol, *destinationAddr)
	if err != nil {
		log.Print("Failed to dial destination address", err)
		return
	}
	defer destinationConn.Close()

	// TODO should we utilize `shutdownChan`?
	shutdownChan := make(chan struct{})
	common.CopyLoop(*snowflakeConn, destinationConn, shutdownChan)
}

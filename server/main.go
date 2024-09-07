package main

import (
	"flag"
	"io"
	"log"
	"net"
	"os"

	"github.com/WofWca/snowflake-generalized/common"
	"github.com/xtaci/smux"
	"gitlab.torproject.org/tpo/anti-censorship/pluggable-transports/ptutil/safelog"
	snowflakeServer "gitlab.torproject.org/tpo/anti-censorship/pluggable-transports/snowflake/v2/server/lib"
)

func main() {
	var listenAddr string
	var destinationAddr string
	// var acmeEmail string
	// var acmeHostnamesCommas string
	// var disableTLS bool
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
	// flag.StringVar(&acmeEmail, "acme-email", "", "optional contact email for Let's Encrypt notifications")
	// flag.StringVar(&acmeHostnamesCommas, "acme-hostnames", "", "comma-separated hostnames for TLS certificate")
	// flag.BoolVar(&disableTLS, "disable-tls", false, "don't use HTTPS")
	// flag.StringVar(&logFilename, "log", "", "log file to write to")
	flag.BoolVar(&unsafeLogging, "unsafe-logging", false, "prevent logs from being scrubbed")
	// flag.BoolVar(&versionFlag, "version", false, "display version info to stderr and quit")
	flag.Parse()

	// The snowflake server runs a websocket server. To run this securely, you will
	// need a valid certificate.
	// certManager := &autocert.Manager{
	// 	Prompt:     autocert.AcceptTOS,
	// 	HostPolicy: autocert.HostWhitelist("snowflake.yourdomain.com"),
	// 	Email:      "you@yourdomain.com",
	// }

	if destinationAddr == "" {
		flag.Usage()
		log.Fatalf("\"destination-address\" must be specified")
	}
	listenAddrStruct, err := net.ResolveTCPAddr("tcp", listenAddr)
	if err != nil {
		log.Fatalf("error resolving listen address: %s", err.Error())
	}

	transport := snowflakeServer.NewSnowflakeServer(nil)
	numKCPInstances := 1
	ln, err := transport.Listen(listenAddrStruct, numKCPInstances)
	if err != nil {
		log.Fatalf("error opening listener: %s", err.Error())
	}

	log.Printf(
		"Listening for proxy connections on \"%v\" and forwarding them to \"%v\"",
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
		log.Printf("Got Snowflake client connection! Forwarding to \"%v\"", destinationAddr)
		go serveSnowflakeConnection(&clientConn, &destinationAddr)
	}
}

// Closes the connection when it finishes serving it.
func serveSnowflakeConnection(snowflakeConn *net.Conn, destinationAddr *string) {
	defer (*snowflakeConn).Close()

	smuxConfig := smux.DefaultConfig()
	// Let's not close the connection on our own, and let Snowflake handle that.
	smuxConfig.KeepAliveDisabled = true

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
			destinationConn, err := net.Dial("tcp", *destinationAddr)
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

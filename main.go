package main

import (
	"context"
	"crypto/tls"
	"flag"
	"log"
	"net"
	"time"

	"github.com/stargazer39/udp-to-tcp-client/tunnel"
)

func main() {
	// TODO - Write Tests
	// Client implementation
	udpAddrStr := flag.String("u", ":50899", "UDP from addr")
	tcpAddrStr := flag.String("h", "139.162.51.182:8088", "Host server addr")
	tlsEnabled := flag.Bool("tls", false, "Enable TLS")
	tlsSNI := flag.String("sni", "google.com", "TLS SNI")
	allowInsercure := flag.Bool("insecure", false, "Allow insecure TLS")

	flag.Parse()

	ctx, cancel := context.WithCancel(context.Background())

	defer cancel()

	handle := tunnel.NewTunnelFromAddr(*tcpAddrStr, *udpAddrStr, *tlsEnabled, &tls.Config{InsecureSkipVerify: *allowInsercure, ServerName: *tlsSNI})

	handle.OnUDPReady("c1", func(udpAddr net.Addr) {
		log.Println(udpAddr)
	})

	go func() {
		for {
			time.Sleep(time.Second)
			log.Println(handle.GetTotal())
		}
	}()

	if err := handle.Run(ctx); err != nil {
		log.Println(err)
	}
}

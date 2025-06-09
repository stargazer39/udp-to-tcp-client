package main

import (
	"context"
	"crypto/tls"
	"flag"
	"log"
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
	token := flag.String("token", "", "Token for authentication")

	flag.Parse()

	if *token == "" {
		log.Fatal("Token is required")
	}

	ctx, cancel := context.WithCancel(context.Background())

	defer cancel()

	handle := tunnel.NewTunnelFromAddr(*tcpAddrStr, *udpAddrStr, *tlsEnabled, &tls.Config{InsecureSkipVerify: *allowInsercure, ServerName: *tlsSNI})

	addr, err := handle.Start(ctx)

	if err != nil {
		log.Fatal(err)
	}

	log.Printf("ListenUDP %s\n", addr)

	go func() {
		for {
			time.Sleep(time.Second * 2)
			log.Println(handle.GetTotal())
		}
	}()

	if err := handle.Run(ctx, tunnel.FirstMessage{
		Token: *token,
	}); err != nil {
		log.Println(err)
	}
}

package main

import (
	"context"
	"flag"
	"log"

	"github.com/stargazer39/udp-to-tcp-client/tunnel"
)

func main() {
	// Client implementation
	udpAddrStr := flag.String("u", ":50899", "UDP from addr")
	tcpAddrStr := flag.String("h", "139.162.51.182:8088", "Host server addr")

	flag.Parse()

	ctx, cancel := context.WithCancel(context.Background())

	defer cancel()

	handle := tunnel.NewTunnelFromAddr(ctx, *tcpAddrStr, *udpAddrStr, false, nil)

	if err := handle.Start(ctx); err != nil {
		log.Println(err)
	}
}

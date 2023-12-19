package udptotcpclient

import (
	"context"
	"flag"
	"fmt"
	"log"
	"net"
	"os"
	"testing"
	"time"
)

func TestMain(m *testing.M) {
	// Client implementation
	udpAddrStr := flag.String("u", ":51280", "UDP from addr")
	tcpAddrStr := flag.String("h", "139.162.51.182:8088", "Host server addr")

	flag.Parse()

	udpAddr, err := net.ResolveUDPAddr("udp", *udpAddrStr)

	if err != nil {
		fmt.Println(err)
		os.Exit(1)
	}

	// Initialize TCP client
	tcpAddr, err := net.ResolveTCPAddr("tcp4", *tcpAddrStr)

	if err != nil {
		fmt.Println(err)
		os.Exit(1)
	}

	ctx, cancel := context.WithCancel(context.Background())

	go func() {
		time.Sleep(time.Second * 60)
		cancel()
	}()

	if err := ServeUDPOverTCP(ctx, tcpAddr, udpAddr); err != nil {
		log.Println(err)
	}
}

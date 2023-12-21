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
	udpAddrStr := flag.String("u", ":0", "UDP from addr")
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

	defer cancel()
	go func() {
		time.Sleep(time.Second * 60)
		// cancel()
	}()

	handle := New(tcpAddr, udpAddr)

	go func() {
		log.Println(handle.GetUDPAddr())
		for {
			log.Println(handle.GetTotal())
			time.Sleep(time.Second)
		}
	}()
	if err := handle.Start(ctx); err != nil {
		log.Println(err)
	}

	time.Sleep(time.Minute * 5)
	if err := handle.Stop(); err != nil {
		log.Println(err)
	}
}

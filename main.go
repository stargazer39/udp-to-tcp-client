package main

import (
	"context"
	"flag"
	"fmt"
	"log"
	"net"
	"os"
	"time"
)

const UDP_BUFFER_SIZE = 8192

func main() {
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

	if err := ServeUDPOverTCP(tcpAddr, udpAddr); err != nil {
		time.Sleep(time.Second)
	}
}

func ServeUDPOverTCP(tcpAddr *net.TCPAddr, udpAddr *net.UDPAddr) error {
	// Dial to the address with UDP
	uConn, err := net.ListenUDP("udp", udpAddr)

	if err != nil {
		fmt.Println(err)
		os.Exit(1)
	}

	// Connect to the address with tcp
	conn, err := net.DialTCP("tcp", nil, tcpAddr)

	if err != nil {
		return err
	}

	defer conn.Close()

	log.Println("Connected to Server")

	var started bool

	fails := 0

	udpCopyContext, cancel := context.WithCancel(context.Background())

	defer cancel()

	for {
	tryAgain:

		if fails > 5 {
			return fmt.Errorf("retries exceeded")
		}

		var udpFrame = make([]byte, UDP_BUFFER_SIZE)

		n, ad, err := uConn.ReadFromUDP(udpFrame)

		// log.Println("got udp ", n)
		if err != nil {
			fails++
			log.Printf("error - %v, retry - %d", err, fails)
			goto tryAgain
		}

		if err := sendBuffer(udpFrame[:n], conn); err != nil {
			log.Println(err)
			return err
		}

		// log.Println("wrote udp")
		if !started {
			started = true
			log.Println("got 1st packet. copying server to udp")
			go copyServerToUDP(udpCopyContext, conn, uConn, *ad)
		}
	}
}

func copyServerToUDP(ctx context.Context, conn *net.TCPConn, cConn *net.UDPConn, uAddr net.UDPAddr) {
	for {
		select {
		case <-ctx.Done():
			log.Println("copyServerToUDP context canceled")
			return
		default:
		}

		buf, err := recvbuffer(conn)

		if err != nil {
			log.Println(err)
			break
		}

		// Write to udp
		if _, err := cConn.WriteToUDP(buf, &uAddr); err != nil {
			log.Println(err)
			break
		}
	}
}

package main

import (
	"context"
	"encoding/binary"
	"flag"
	"fmt"
	"io"
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

retry:
	// Dial to the address with UDP
	uConn, err := net.ListenUDP("udp", udpAddr)

	if err != nil {
		fmt.Println(err)
		os.Exit(1)
	}

	if err := ServeUDPOverTCP(tcpAddr, uConn); err != nil {
		uConn.Close()
		time.Sleep(time.Second)
		goto retry
	}
}

func ServeUDPOverTCP(tcpAddr *net.TCPAddr, uConn *net.UDPConn) error {
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

func sendBuffer(buffer []byte, conn net.Conn) error {
	length := make([]byte, 2)

	binary.LittleEndian.PutUint16(length, uint16(len(buffer)))

	i, err := conn.Write(length)

	if err != nil {
		return err
	}

	if i != len(length) {
		log.Fatal("len")
	}

	j, err := conn.Write(buffer)

	if err != nil {
		return err
	}

	if j != len(buffer) {
		log.Fatal("buf")
	}

	return nil
}

func recvbuffer(conn net.Conn) ([]byte, error) {
	length := make([]byte, 2)

	if _, err := io.ReadFull(conn, length); err != nil {
		return nil, err
	}

	msg := make([]byte, binary.LittleEndian.Uint16(length))

	if _, err := io.ReadFull(conn, msg); err != nil {
		return nil, err
	}

	return msg, nil
}

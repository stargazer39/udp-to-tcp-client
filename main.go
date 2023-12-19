package udptotcpclient

import (
	"context"
	"fmt"
	"log"
	"net"
)

const UDP_BUFFER_SIZE = 8192

func ServeUDPOverTCP(ctx context.Context, tcpAddr *net.TCPAddr, udpAddr *net.UDPAddr) error {
	// Dial to the address with UDP
	uConn, err := net.ListenUDP("udp", udpAddr)

	if err != nil {
		log.Println(err)
		return err
	}

	defer uConn.Close()

	// Connect to the address with tcp
	conn, err := net.DialTCP("tcp", nil, tcpAddr)

	if err != nil {
		return err
	}

	defer conn.Close()

	log.Println("Connected to Server")

	var started bool

	fails := 0

	udpCopyContext, cancel := context.WithCancel(ctx)

	defer cancel()

out:
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

		select {
		case <-ctx.Done():
			log.Println("copyServerToUDP context canceled")
			break out
		default:
		}
	}

	return fmt.Errorf("context was canceled")
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

package udptotcpclient

import (
	"context"
	"fmt"
	"log"
	"net"
)

type UDPOverTCP struct {
	tcpAddr    *net.TCPAddr
	udpAddr    *net.UDPAddr
	bufferSize int
	logger     *log.Logger
}

func New(tcpAddr *net.TCPAddr, udpAddr *net.UDPAddr) *UDPOverTCP {
	return &UDPOverTCP{
		tcpAddr:    tcpAddr,
		udpAddr:    udpAddr,
		bufferSize: 8192,
		logger:     log.Default(),
	}
}

func (ut *UDPOverTCP) Start(ctx context.Context) error {
	// Dial to the address with UDP
	uConn, err := net.ListenUDP("udp", ut.udpAddr)

	if err != nil {
		ut.logger.Println(err)
		return err
	}

	ut.logger.Println("Listening on UDP", uConn.LocalAddr())

	defer uConn.Close()

	// Connect to the address with tcp
	conn, err := net.DialTCP("tcp", nil, ut.tcpAddr)

	if err != nil {
		ut.logger.Println(err)
		return err
	}

	defer conn.Close()

	ut.logger.Println("Connected to Server", ut.tcpAddr)

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

		var udpFrame = make([]byte, ut.bufferSize)

		n, ad, err := uConn.ReadFromUDP(udpFrame)

		// log.Println("got udp ", n)
		if err != nil {
			fails++
			ut.logger.Printf("error - %v, retry - %d", err, fails)
			goto tryAgain
		}

		if err := sendBuffer(udpFrame[:n], conn); err != nil {
			ut.logger.Println(err)
			return err
		}

		// log.Println("wrote udp")
		if !started {
			started = true
			ut.logger.Println("got 1st packet. copying server to udp")
			go copyServerToUDP(udpCopyContext, conn, uConn, *ad, ut.logger)
		}

		select {
		case <-ctx.Done():
			ut.logger.Println("copyServerToUDP context canceled")
			break out
		default:
		}
	}

	return fmt.Errorf("context was canceled")
}

func copyServerToUDP(ctx context.Context, conn *net.TCPConn, cConn *net.UDPConn, uAddr net.UDPAddr, logger *log.Logger) {
	for {
		select {
		case <-ctx.Done():
			logger.Println("copyServerToUDP context canceled")
			return
		default:
		}

		buf, err := recvbuffer(conn)

		if err != nil {
			logger.Println(err)
			break
		}

		// Write to udp
		if _, err := cConn.WriteToUDP(buf, &uAddr); err != nil {
			logger.Println(err)
			break
		}
	}
}

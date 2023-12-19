package udptotcpclient

import (
	"context"
	"fmt"
	"log"
	"net"
	"sync"
)

type UDPOverTCP struct {
	tcpAddr       *net.TCPAddr
	udpAddr       *net.UDPAddr
	bufferSize    int
	logger        *log.Logger
	totalSent     float32
	totalReceived float32
	sentMut       sync.RWMutex
	recvMut       sync.RWMutex
	udpPortMut    sync.Mutex
	udpPort       net.Addr
}

func New(tcpAddr *net.TCPAddr, udpAddr *net.UDPAddr) *UDPOverTCP {
	portMut := sync.Mutex{}
	portMut.Lock()

	return &UDPOverTCP{
		tcpAddr:    tcpAddr,
		udpAddr:    udpAddr,
		bufferSize: 8192,
		logger:     log.Default(),
		udpPortMut: portMut,
	}
}

func (ut *UDPOverTCP) Start(ctx context.Context) error {
	// Dial to the address with UDP
	uConn, err := net.ListenUDP("udp", ut.udpAddr)

	if err != nil {
		ut.logger.Println(err)
		return err
	}

	ut.udpPort = uConn.LocalAddr()
	ut.udpPortMut.Unlock()

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

		ut.sentMut.Lock()
		ut.totalSent += float32(n) / 1024
		ut.sentMut.Unlock()

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
			go copyServerToUDP(udpCopyContext, conn, uConn, *ad, ut.logger, func(i int) {
				ut.recvMut.Lock()
				ut.totalReceived += float32(n) / 1024
				ut.recvMut.Unlock()
			})
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

func (ut *UDPOverTCP) GetTotal() (float32, float32) {
	ut.sentMut.RLock()
	defer ut.sentMut.RUnlock()

	ut.recvMut.RLock()
	defer ut.recvMut.RUnlock()
	return ut.totalReceived, ut.totalSent
}

func (ut *UDPOverTCP) GetUDPAddr() net.Addr {
	ut.udpPortMut.Lock()
	defer ut.udpPortMut.Unlock()

	return ut.udpPort
}

func copyServerToUDP(ctx context.Context, conn *net.TCPConn, cConn *net.UDPConn, uAddr net.UDPAddr, logger *log.Logger, written func(i int)) {
	for {
		select {
		case <-ctx.Done():
			logger.Println("copyServerToUDP context canceled")
			return
		default:
		}

		buf, err := recvbuffer(conn)

		written(len(buf))
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

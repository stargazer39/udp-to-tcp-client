package udptotcpclient

import (
	"context"
	"crypto/tls"
	"log"
	"net"
	"sync"
)

type UDPOverTCP struct {
	udpConn       *net.UDPConn
	tcpConn       net.Conn
	ctx           context.Context
	ctxCancelFunc context.CancelFunc
	started       bool
	fails         int

	bufferSize    int
	logger        *log.Logger
	totalSent     float32
	totalReceived float32
	sentMut       sync.RWMutex
	recvMut       sync.RWMutex
	udpPortMut    *sync.Mutex
}

func NewTunnelFromAddr(tcpAddress string, udpAddress string, enableTLS bool, tlsConfig *tls.Config) (*UDPOverTCP, error) {
	udpAddr, err := net.ResolveUDPAddr("udp4", "0.0.0.0:0")

	if err != nil {
		return nil, err
	}

	// Initialize TCP client
	tcpAddr, err := net.ResolveTCPAddr("tcp4", tcpAddress)

	if err != nil {
		return nil, err
	}

	// Dial to the address with UDP
	uConn, err := net.ListenUDP("udp", udpAddr)

	if err != nil {
		return nil, err
	}

	var conn net.Conn

	if enableTLS {
		c, err := tls.Dial("tcp", tcpAddress, tlsConfig)

		if err != nil {
			return nil, err
		}

		conn = c
	} else {
		c, err := net.DialTCP("tcp", nil, tcpAddr)

		if err != nil {
			return nil, err
		}

		conn = c
	}

	if err != nil {
		return nil, err
	}

	return NewFromRaw(uConn, conn), nil
}

func NewFromRaw(udpConn *net.UDPConn, tcpConn net.Conn) *UDPOverTCP {
	portMut := sync.Mutex{}
	portMut.Lock()

	return &UDPOverTCP{
		bufferSize: 8192,
		logger:     log.Default(),
		udpPortMut: &portMut,
		udpConn:    udpConn,
		tcpConn:    tcpConn,
	}
}

func (ut *UDPOverTCP) Start(ctx context.Context) error {
	ut.udpPortMut.Unlock()

	ut.logger.Println("Listening on UDP", ut.udpConn.LocalAddr())

	udpCopyContext, cancel := context.WithCancel(ctx)

	ut.ctx = udpCopyContext
	ut.ctxCancelFunc = cancel

	go ut.copyUDPToServer()

	return nil
	// return fmt.Errorf("context was canceled")
}

func (ut *UDPOverTCP) Stop() error {
	ut.ctxCancelFunc()
	ut.udpConn.Close()
	// ut.tcpConn.Close()
	return nil
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

	return ut.udpConn.LocalAddr()
}

func (ut *UDPOverTCP) copyUDPToServer() {
out:
	for {
	tryAgain:

		if ut.fails > 5 {
			ut.logger.Println("retries exceeded")
			return
		}

		var udpFrame = make([]byte, ut.bufferSize)

		n, ad, err := ut.udpConn.ReadFromUDP(udpFrame)

		ut.sentMut.Lock()
		ut.totalSent += float32(n) / 1024
		ut.sentMut.Unlock()

		// log.Println("got udp ", n)
		if err != nil {
			ut.fails++
			ut.logger.Printf("error - %v, retry - %d", err, ut.fails)
			goto tryAgain
		}

		if err := sendBuffer(udpFrame[:n], ut.tcpConn); err != nil {
			ut.logger.Println(err)
			return
		}

		// log.Println("wrote udp")
		if !ut.started {
			ut.started = true
			ut.logger.Println("got 1st packet. copying server to udp")
			go ut.copyServerToUDP(*ad)
		}

		select {
		case <-ut.ctx.Done():
			ut.logger.Println("copyServerToUDP context canceled")
			break out
		default:
		}
	}
}

func (ut *UDPOverTCP) copyServerToUDP(uAddr net.UDPAddr) {
	for {
		select {
		case <-ut.ctx.Done():
			ut.logger.Println("copyServerToUDP context canceled")
			return
		default:
		}

		buf, err := recvbuffer(ut.tcpConn)

		ut.recvMut.Lock()
		ut.totalReceived += float32(len(buf)) / 1024
		ut.recvMut.Unlock()

		if err != nil {
			ut.logger.Println(err)
			break
		}

		// Write to udp
		if _, err := ut.udpConn.WriteToUDP(buf, &uAddr); err != nil {
			ut.logger.Println(err)
			break
		}
	}
}

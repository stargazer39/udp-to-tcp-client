package tunnel

import (
	"context"
	"crypto/tls"
	"fmt"
	"log"
	"net"
	"time"
)

type UDPOverTCP struct {
	raw        *UDPOverTCPRaw
	udpAddress string
	tls        bool
	tlsConfig  *tls.Config
	tcpAddress string
}

func NewTunnelFromAddr(ctx context.Context, tcpAddress string, udpAddress string, enableTLS bool, tlsConfig *tls.Config) (*UDPOverTCP, error) {
	return &UDPOverTCP{
		tls:        enableTLS,
		tcpAddress: tcpAddress,
		udpAddress: udpAddress,
		tlsConfig:  tlsConfig,
	}, nil
}

func connectToTun(ctx context.Context, tcpAddress string, enableTLS bool, tlsConfig *tls.Config) (net.Conn, error) {
	log.Println("Connecting to", tcpAddress)

	tcpDialer := net.Dialer{Timeout: time.Second * 30, Deadline: time.Now().Add(time.Second * 30)}
	var conn net.Conn

	if enableTLS {
		c, err := tls.DialWithDialer(&tcpDialer, "tcp", tcpAddress, tlsConfig)

		if err != nil {
			return nil, err
		}

		conn = c
	} else {
		c, err := tcpDialer.DialContext(ctx, "tcp", tcpAddress)

		if err != nil {
			return nil, err
		}

		conn = c
	}

	// go func() {
	// 	time.Sleep(time.Second * 30)
	// 	conn.Close()
	// }()

	return conn, nil
}

func (ut *UDPOverTCP) GetUDPAddr() net.Addr {
	return ut.raw.GetUDPAddr()
}

func (ut *UDPOverTCP) GetTotal() (float32, float32) {
	return ut.raw.GetTotal()
}

func (ut *UDPOverTCP) Start(pCtx context.Context) error {
	udpAddr, err := net.ResolveUDPAddr("udp4", ut.udpAddress)

	if err != nil {
		return err
	}

	// Dial to the address with UDP
	uConn, err := net.ListenUDP("udp", udpAddr)

	if err != nil {
		return err
	}

	for {
		select {
		case <-pCtx.Done():
			return fmt.Errorf("context canceled")
		default:
		}

		retries := 0

	retry:
		conn, err := connectToTun(pCtx, ut.tcpAddress, ut.tls, ut.tlsConfig)

		if err != nil {
			log.Println(err)
			time.Sleep(time.Second * 10)
			retries++

			if retries > 5 {
				goto retry
			}
			return err
		}

		ut.raw = NewFromRaw(uConn, conn)

		if err := ut.raw.Start(pCtx); err != nil {
			log.Println("raw", err)
		}

		log.Println("stopped")
		time.Sleep(time.Second * 10)
	}
}

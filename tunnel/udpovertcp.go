package tunnel

import (
	"context"
	"crypto/tls"
	"fmt"
	"log"
	"net"
	"sync"
	"time"
)

type UDPReadyCallback func(udpAddr net.Addr)
type UDPOverTCP struct {
	raw        *UDPOverTCPRaw
	udpAddress string
	tls        bool
	tlsConfig  *tls.Config
	tcpAddress string

	mut            sync.Mutex
	readyCallbacks map[string]UDPReadyCallback
	readyMapMut    sync.RWMutex
	done           chan error
	udpConn        *net.UDPConn
}

func NewTunnelFromAddr(tcpAddress string, udpAddress string, enableTLS bool, tlsConfig *tls.Config) *UDPOverTCP {
	return &UDPOverTCP{
		tls:            enableTLS,
		tcpAddress:     tcpAddress,
		udpAddress:     udpAddress,
		tlsConfig:      tlsConfig,
		mut:            sync.Mutex{},
		readyCallbacks: make(map[string]UDPReadyCallback),
		readyMapMut:    sync.RWMutex{},
		done:           make(chan error),
	}
}

func connectToTun(ctx context.Context, tcpAddress string, enableTLS bool, tlsConfig *tls.Config) (net.Conn, error) {
	log.Println("Connecting to", tcpAddress)

	tcpDialer := net.Dialer{Timeout: time.Second * 5, Deadline: time.Now().Add(time.Second * 5)}

	conn, err := tcpDialer.DialContext(ctx, "tcp", tcpAddress)

	if err != nil {
		return nil, err
	}

	if enableTLS {
		newConn := tls.Client(conn, tlsConfig)
		if err := newConn.HandshakeContext(ctx); err != nil {
			return nil, err
		}
		return newConn, nil
	}

	return conn, nil
}

func (ut *UDPOverTCP) OnUDPReady(id string, callback UDPReadyCallback) {
	ut.readyMapMut.Lock()
	defer ut.readyMapMut.Unlock()

	if _, ok := ut.readyCallbacks[id]; ok {
		log.Panic("callback with id ", id, " already exists")
	}

	ut.readyCallbacks[id] = callback
}

func (ut *UDPOverTCP) RemoveOnUDPReady(id string) {
	ut.readyMapMut.Lock()
	defer ut.readyMapMut.Unlock()

	delete(ut.readyCallbacks, id)
}

func (ut *UDPOverTCP) notifyReadiness(udpAddr net.Addr) {
	ut.readyMapMut.RLock()
	defer ut.readyMapMut.RUnlock()

	for _, v := range ut.readyCallbacks {
		go v(udpAddr)
	}
}

func (ut *UDPOverTCP) GetTotal() (float32, float32) {
	return ut.raw.GetTotal()
}

func (ut *UDPOverTCP) Start(ctx context.Context) (net.Addr, error) {
	udpAddr, err := net.ResolveUDPAddr("udp4", ut.udpAddress)

	if err != nil {
		return nil, err
	}

	// Dial to the address with UDP
	uConn, err := net.ListenUDP("udp", udpAddr)

	if err != nil {
		return nil, err
	}

	ut.udpConn = uConn

	return ut.udpConn.LocalAddr(), nil
}

func (ut *UDPOverTCP) Run(pCtx context.Context) error {

	defer ut.udpConn.Close()

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

		// go func() {
		// 	time.Sleep(time.Second * 30)
		// 	conn.Close()
		// }()

		ut.mut.Lock()
		ut.raw = NewFromRaw(ut.udpConn, conn)
		ut.notifyReadiness(ut.raw.GetUDPAddr())
		ut.mut.Unlock()

		if err := ut.raw.Start(pCtx); err != nil {
			log.Println("raw", err)
		}

		conn.Close()

		log.Println("stopped")
		time.Sleep(time.Second * 10)
	}
}

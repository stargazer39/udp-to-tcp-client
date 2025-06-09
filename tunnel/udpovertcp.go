package tunnel

import (
	"context"
	"crypto/tls"
	"encoding/binary"
	"encoding/json"
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

type FirstMessage struct {
	Token string `json:"token"`
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

func connectToTun(ctx context.Context, tcpAddress string, enableTLS bool, tlsConfig *tls.Config, fm FirstMessage) (net.Conn, error) {
	log.Println("Connecting to", tcpAddress)

	tcpDialer := net.Dialer{Timeout: time.Second * 5, Deadline: time.Now().Add(time.Second * 5)}

	var finalConn net.Conn

	conn, err := tcpDialer.DialContext(ctx, "tcp", tcpAddress)

	if err != nil {
		return nil, err
	}

	if enableTLS {
		newConn := tls.Client(conn, tlsConfig)
		if err := newConn.HandshakeContext(ctx); err != nil {
			return nil, err
		}
		finalConn = newConn
	} else {
		finalConn = conn
	}

	// Send the first message
	jsonData, err := json.Marshal(fm)

	if err != nil {
		finalConn.Close()
		return nil, fmt.Errorf("failed to marshal first message: %w", err)
	}

	length := make([]byte, 2)

	binary.LittleEndian.PutUint16(length, uint16(len(jsonData)))

	if _, err := finalConn.Write(length); err != nil {
		finalConn.Close()
		return nil, fmt.Errorf("failed to write length: %w", err)
	}

	log.Printf("Sent first message with %d length", len(jsonData))

	if _, err := finalConn.Write(jsonData); err != nil {
		finalConn.Close()
		return nil, fmt.Errorf("failed to write first message: %w", err)
	}

	log.Println("Sent first message to", tcpAddress)

	return finalConn, nil
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

func (ut *UDPOverTCP) GetTotal() (float32, float32) {
	if ut.raw == nil {
		return 0, 0
	}

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

func (ut *UDPOverTCP) Run(pCtx context.Context, firstMsg FirstMessage) error {

	defer ut.udpConn.Close()

	for {
		select {
		case <-pCtx.Done():
			return fmt.Errorf("context canceled")
		default:
		}

		retries := 0

	retry:
		conn, err := connectToTun(pCtx, ut.tcpAddress, ut.tls, ut.tlsConfig, firstMsg)

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
		ut.mut.Unlock()

		if err := ut.raw.Start(pCtx); err != nil {
			log.Println("raw", err)
		}

		conn.Close()
		time.Sleep(time.Second * 10)
	}
}

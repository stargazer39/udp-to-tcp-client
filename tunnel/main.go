package tunnel

import (
	"context"
	"fmt"
	"log"
	"net"
	"os"
	"sync"
	"time"

	"golang.org/x/sync/errgroup"
)

type UDPOverTCPRaw struct {
	udpConn *net.UDPConn
	tcpConn net.Conn
	fails   int

	bufferSize    int
	logger        *log.Logger
	totalSent     float32
	totalReceived float32
	sentMut       sync.RWMutex
	recvMut       sync.RWMutex
}

const UDP_TIMEOUT = time.Second * 10

func NewFromRaw(udpConn *net.UDPConn, tcpConn net.Conn) *UDPOverTCPRaw {
	return &UDPOverTCPRaw{
		bufferSize: 8192,
		logger:     log.Default(),
		udpConn:    udpConn,
		tcpConn:    tcpConn,
	}
}

func (ut *UDPOverTCPRaw) Start(ctx context.Context) error {
	group, ctx := errgroup.WithContext(ctx)

	group.Go(func() error {
		return ut.run(ctx)
	})

	return group.Wait()
}

func (ut *UDPOverTCPRaw) GetTotal() (float32, float32) {
	ut.sentMut.RLock()
	defer ut.sentMut.RUnlock()

	ut.recvMut.RLock()
	defer ut.recvMut.RUnlock()
	return ut.totalReceived, ut.totalSent
}

func (ut *UDPOverTCPRaw) GetUDPAddr() net.Addr {
	return ut.udpConn.LocalAddr()
}

func (ut *UDPOverTCPRaw) run(pCtx context.Context) error {
	group, ctx := errgroup.WithContext(pCtx)

	var started bool

	for {

		select {
		case <-ctx.Done():
			return ctx.Err()
		default:
		}

	tryAgain:

		if ut.fails > 5 {
			return fmt.Errorf("retries exceeded")
		}

		var udpFrame = make([]byte, ut.bufferSize)

		ut.udpConn.SetReadDeadline(time.Now().Add(UDP_TIMEOUT))

		n, ad, err := ut.udpConn.ReadFromUDP(udpFrame)

		if os.IsTimeout(err) {
			continue
		}
		// log.Println("got udp ", n)
		if err != nil {
			ut.fails++
			ut.logger.Printf("error - %v, retry - %d", err, ut.fails)
			goto tryAgain
		}

		if err := sendBuffer(udpFrame[:n], ut.tcpConn); err != nil {
			return err
		}

		ut.sentMut.Lock()
		ut.totalSent += float32(n) / 1024
		ut.sentMut.Unlock()

		if !started {
			started = true
			ut.logger.Println("got 1st packet. copying server to udp")

			if ad == nil {
				log.Println("udp addr is nil")
			}

			group.Go(func() error {
				return ut.copyServerToUDP(ctx, *ad)
			})
		}

	}
}

func (ut *UDPOverTCPRaw) copyServerToUDP(ctx context.Context, uAddr net.UDPAddr) error {
	for {
		select {
		case <-ctx.Done():
			return ctx.Err()
		default:
		}

		buf, err := recvbuffer(ut.tcpConn)

		if err != nil {
			return err
		}

		ut.recvMut.Lock()
		ut.totalReceived += float32(len(buf)) / 1024
		ut.recvMut.Unlock()

		ut.udpConn.SetWriteDeadline(time.Now().Add(UDP_TIMEOUT))
		// Write to udp
		if _, err := ut.udpConn.WriteToUDP(buf, &uAddr); err != nil {
			return err
		}
	}
}

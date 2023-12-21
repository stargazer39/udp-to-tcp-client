package tunnel

import (
	"encoding/binary"
	"io"
	"log"
	"net"
)

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

package rcon

import (
	"bytes"
	"encoding/binary"
	"errors"
	"fmt"
	"io"
	"net"
	"sync"
	"time"
)

const (
	packetExec = 2
	packetAuth = 3
)

type Client struct {
	conn    net.Conn
	timeout time.Duration
	mu      sync.Mutex
	nextID  int32
}

func Dial(address string, password string, timeout time.Duration) (*Client, error) {
	conn, err := net.DialTimeout("tcp", address, timeout)
	if err != nil {
		return nil, err
	}
	c := &Client{conn: conn, timeout: timeout, nextID: 1}
	if err := c.authenticate(password); err != nil {
		_ = conn.Close()
		return nil, err
	}
	return c, nil
}

func (c *Client) Close() error {
	if c == nil || c.conn == nil {
		return nil
	}
	return c.conn.Close()
}

func (c *Client) Command(command string) (string, error) {
	c.mu.Lock()
	defer c.mu.Unlock()

	id := c.nextID
	c.nextID++
	if err := c.writePacket(id, packetExec, command); err != nil {
		return "", err
	}
	packet, err := c.readPacket()
	if err != nil {
		return "", err
	}
	if packet.id != id {
		return packet.body, fmt.Errorf("unexpected RCON response id %d, wanted %d", packet.id, id)
	}
	return packet.body, nil
}

func (c *Client) authenticate(password string) error {
	c.mu.Lock()
	defer c.mu.Unlock()

	id := c.nextID
	c.nextID++
	if err := c.writePacket(id, packetAuth, password); err != nil {
		return err
	}
	packet, err := c.readPacket()
	if err != nil {
		return err
	}
	if packet.id == -1 {
		return errors.New("RCON authentication failed")
	}
	if packet.id != id {
		return fmt.Errorf("unexpected RCON auth response id %d, wanted %d", packet.id, id)
	}
	return nil
}

type packet struct {
	id   int32
	kind int32
	body string
}

func (c *Client) writePacket(id int32, kind int32, body string) error {
	if err := c.conn.SetDeadline(time.Now().Add(c.timeout)); err != nil {
		return err
	}
	payload := bytes.Buffer{}
	_ = binary.Write(&payload, binary.LittleEndian, id)
	_ = binary.Write(&payload, binary.LittleEndian, kind)
	payload.WriteString(body)
	payload.WriteByte(0)
	payload.WriteByte(0)

	size := int32(payload.Len())
	if err := binary.Write(c.conn, binary.LittleEndian, size); err != nil {
		return err
	}
	_, err := c.conn.Write(payload.Bytes())
	return err
}

func (c *Client) readPacket() (packet, error) {
	if err := c.conn.SetDeadline(time.Now().Add(c.timeout)); err != nil {
		return packet{}, err
	}
	var size int32
	if err := binary.Read(c.conn, binary.LittleEndian, &size); err != nil {
		return packet{}, err
	}
	if size < 10 {
		return packet{}, fmt.Errorf("invalid RCON packet size %d", size)
	}
	data := make([]byte, size)
	if _, err := io.ReadFull(c.conn, data); err != nil {
		return packet{}, err
	}
	p := packet{}
	p.id = int32(binary.LittleEndian.Uint32(data[0:4]))
	p.kind = int32(binary.LittleEndian.Uint32(data[4:8]))
	body := data[8:]
	if len(body) >= 2 {
		body = body[:len(body)-2]
	}
	p.body = string(body)
	return p, nil
}

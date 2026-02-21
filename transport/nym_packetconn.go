package transport

import (
	"encoding/binary"
	"fmt"
	"io"
	"net"
	"time"
)

// nymPacketConn implements net.PacketConn over a stream connection (net.Conn) using
// length-prefixed framing. Each packet is transmitted as a 4-byte big-endian length
// followed by the packet payload. This emulates UDP-like semantics over the Nym
// SOCKS5 stream transport.
//
// Wire format per packet:
//
//	[4 bytes: uint32 big-endian payload length][N bytes: payload]
type nymPacketConn struct {
	conn net.Conn
}

// newNymPacketConn wraps a net.Conn in a nymPacketConn for packet-over-stream framing.
func newNymPacketConn(conn net.Conn) *nymPacketConn {
	return &nymPacketConn{conn: conn}
}

// ReadFrom reads a length-prefixed packet from the stream.
// The source address is always the remote address of the underlying connection.
func (c *nymPacketConn) ReadFrom(p []byte) (n int, addr net.Addr, err error) {
	// Read 4-byte length prefix
	var lenBuf [4]byte
	if _, err = io.ReadFull(c.conn, lenBuf[:]); err != nil {
		return 0, nil, fmt.Errorf("nym packet: failed to read length prefix: %w", err)
	}

	pktLen := binary.BigEndian.Uint32(lenBuf[:])
	if int(pktLen) > len(p) {
		// Drain the oversized payload to keep the stream in sync, then return an error.
		_, drainErr := io.CopyN(io.Discard, c.conn, int64(pktLen))
		msg := fmt.Sprintf("nym packet: buffer too small (%d) for packet of size %d", len(p), pktLen)
		if drainErr != nil {
			msg += fmt.Sprintf(" (drain error: %v)", drainErr)
		}
		return 0, nil, fmt.Errorf("%s", msg)
	}

	if _, err = io.ReadFull(c.conn, p[:pktLen]); err != nil {
		return 0, nil, fmt.Errorf("nym packet: failed to read payload: %w", err)
	}

	return int(pktLen), c.conn.RemoteAddr(), nil
}

// WriteTo writes p as a length-prefixed packet to the stream.
// The addr parameter is accepted for interface compatibility but ignored;
// all writes go to the remote end of the underlying connection.
func (c *nymPacketConn) WriteTo(p []byte, _ net.Addr) (n int, err error) {
	// Build frame: 4-byte big-endian length prefix + payload
	frame := make([]byte, 4+len(p))
	binary.BigEndian.PutUint32(frame[:4], uint32(len(p)))
	copy(frame[4:], p)

	if _, err = c.conn.Write(frame); err != nil {
		return 0, fmt.Errorf("nym packet: write failed: %w", err)
	}

	return len(p), nil
}

// Close closes the underlying stream connection.
func (c *nymPacketConn) Close() error {
	return c.conn.Close()
}

// LocalAddr returns the local address of the underlying connection.
func (c *nymPacketConn) LocalAddr() net.Addr {
	return c.conn.LocalAddr()
}

// SetDeadline sets the read and write deadlines on the underlying connection.
func (c *nymPacketConn) SetDeadline(t time.Time) error {
	return c.conn.SetDeadline(t)
}

// SetReadDeadline sets the read deadline on the underlying connection.
func (c *nymPacketConn) SetReadDeadline(t time.Time) error {
	return c.conn.SetReadDeadline(t)
}

// SetWriteDeadline sets the write deadline on the underlying connection.
func (c *nymPacketConn) SetWriteDeadline(t time.Time) error {
	return c.conn.SetWriteDeadline(t)
}

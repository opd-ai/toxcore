//go:build !linux && !freebsd && !darwin
// +build !linux,!freebsd,!darwin

package transport

import (
	"errors"
	"net"
)

// ErrReusePortNotSupported is returned on platforms that don't support SO_REUSEPORT.
var ErrReusePortNotSupported = errors.New("SO_REUSEPORT not supported on this platform")

// createReusePortSockets returns an error on platforms that don't support SO_REUSEPORT.
// The transport will fall back to a single socket.
func createReusePortSockets(listenAddr string, numSockets int) ([]net.PacketConn, net.Addr, error) {
	return nil, nil, ErrReusePortNotSupported
}

package transport

import (
	"fmt"
	"net"
)

// startMockSOCKS5Server starts a minimal TCP server that accepts connections
// on a random local port. It is used as a stand-in SOCKS5 server in tests
// that only need to verify dial reachability and do not require a full
// SOCKS5 handshake.
//
// The returned listener's goroutine exits automatically when Close() is called,
// so no separate cleanup is needed beyond listener.Close().
func startMockSOCKS5Server() (net.Listener, error) {
	listener, err := net.Listen("tcp", "127.0.0.1:0")
	if err != nil {
		return nil, fmt.Errorf("failed to start mock SOCKS5 server: %w", err)
	}

	// Accept connections in background; Accept returns an error on listener.Close().
	go func() {
		for {
			conn, err := listener.Accept()
			if err != nil {
				return // listener closed
			}
			conn.Close()
		}
	}()

	return listener, nil
}

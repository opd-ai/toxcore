//go:build linux || freebsd || darwin
// +build linux freebsd darwin

package transport

import (
	"fmt"
	"net"
	"os"
	"syscall"

	"github.com/sirupsen/logrus"
	"golang.org/x/sys/unix"
)

// createReusePortSockets creates multiple UDP sockets sharing the same address via SO_REUSEPORT.
// This enables kernel-level load balancing across CPU cores.
func createReusePortSockets(listenAddr string, numSockets int) ([]net.PacketConn, net.Addr, error) {
	// Resolve the address first
	addr, err := net.ResolveUDPAddr("udp", listenAddr)
	if err != nil {
		return nil, nil, err
	}

	sockets := make([]net.PacketConn, 0, numSockets)
	var sharedAddr net.Addr

	for i := 0; i < numSockets; i++ {
		conn, err := createReusePortSocket(addr)
		if err != nil {
			// Close any already-created sockets
			for _, s := range sockets {
				s.Close()
			}
			return nil, nil, err
		}

		sockets = append(sockets, conn)
		if sharedAddr == nil {
			sharedAddr = conn.LocalAddr()
			// If port was 0 (ephemeral), update addr for subsequent sockets
			if addr.Port == 0 {
				if updatedAddr, ok := sharedAddr.(*net.UDPAddr); ok {
					addr = updatedAddr
				}
			}
		}
	}

	logrus.WithFields(logrus.Fields{
		"function":    "createReusePortSockets",
		"listen_addr": sharedAddr.String(),
		"num_sockets": numSockets,
	}).Info("Created SO_REUSEPORT sockets")

	return sockets, sharedAddr, nil
}

// createReusePortSocket creates a single UDP socket with SO_REUSEPORT enabled.
func createReusePortSocket(addr *net.UDPAddr) (net.PacketConn, error) {
	// Determine address family
	family := unix.AF_INET
	if addr.IP.To4() == nil && addr.IP != nil {
		family = unix.AF_INET6
	}

	// Create socket
	fd, err := unix.Socket(family, unix.SOCK_DGRAM, unix.IPPROTO_UDP)
	if err != nil {
		return nil, err
	}

	// Set SO_REUSEADDR
	if err := unix.SetsockoptInt(fd, unix.SOL_SOCKET, unix.SO_REUSEADDR, 1); err != nil {
		unix.Close(fd)
		return nil, err
	}

	// Set SO_REUSEPORT - this is the key option
	if err := unix.SetsockoptInt(fd, unix.SOL_SOCKET, unix.SO_REUSEPORT, 1); err != nil {
		unix.Close(fd)
		return nil, err
	}

	// Build sockaddr and bind using syscall (compatible with both unix and syscall packages)
	var sockaddr syscall.Sockaddr
	if family == unix.AF_INET6 {
		sa6 := &syscall.SockaddrInet6{Port: addr.Port}
		if addr.IP != nil {
			copy(sa6.Addr[:], addr.IP.To16())
		}
		sockaddr = sa6
	} else {
		sa4 := &syscall.SockaddrInet4{Port: addr.Port}
		if addr.IP != nil {
			copy(sa4.Addr[:], addr.IP.To4())
		}
		sockaddr = sa4
	}

	// Use syscall.Bind which works with syscall.Sockaddr
	if err := syscall.Bind(fd, sockaddr); err != nil {
		unix.Close(fd)
		return nil, err
	}

	// Convert to net.PacketConn via file descriptor
	file := os.NewFile(uintptr(fd), fmt.Sprintf("udp-reuseport-%d", fd))
	conn, err := net.FilePacketConn(file)
	file.Close() // Close file, fd is duplicated by FilePacketConn
	if err != nil {
		unix.Close(fd)
		return nil, err
	}

	return conn, nil
}

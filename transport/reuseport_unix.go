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
	addr, err := net.ResolveUDPAddr("udp", listenAddr)
	if err != nil {
		return nil, nil, err
	}

	sockets := make([]net.PacketConn, 0, numSockets)
	var sharedAddr net.Addr

	for i := 0; i < numSockets; i++ {
		conn, err := createReusePortSocket(addr)
		if err != nil {
			closeAllSockets(sockets)
			return nil, nil, err
		}

		sockets = append(sockets, conn)
		if sharedAddr == nil {
			sharedAddr = conn.LocalAddr()
			addr = updateAddrIfEphemeral(addr, sharedAddr)
		}
	}

	logReusePortSocketsCreated(sharedAddr, numSockets)
	return sockets, sharedAddr, nil
}

// closeAllSockets closes all provided packet connections.
func closeAllSockets(sockets []net.PacketConn) {
	for _, s := range sockets {
		s.Close()
	}
}

// updateAddrIfEphemeral updates addr's port if it was ephemeral (0).
func updateAddrIfEphemeral(addr *net.UDPAddr, sharedAddr net.Addr) *net.UDPAddr {
	if addr.Port == 0 {
		if updatedAddr, ok := sharedAddr.(*net.UDPAddr); ok {
			return updatedAddr
		}
	}
	return addr
}

// logReusePortSocketsCreated logs successful creation of SO_REUSEPORT sockets.
func logReusePortSocketsCreated(sharedAddr net.Addr, numSockets int) {
	logrus.WithFields(logrus.Fields{
		"function":    "createReusePortSockets",
		"listen_addr": sharedAddr.String(),
		"num_sockets": numSockets,
	}).Info("Created SO_REUSEPORT sockets")
}

// createReusePortSocket creates a single UDP socket with SO_REUSEPORT enabled.
func createReusePortSocket(addr *net.UDPAddr) (net.PacketConn, error) {
	family := determineAddressFamily(addr)

	fd, err := createAndConfigureSocket(family)
	if err != nil {
		return nil, err
	}

	if err := bindSocket(fd, addr, family); err != nil {
		unix.Close(fd)
		return nil, err
	}

	return convertToPacketConn(fd)
}

// determineAddressFamily returns the address family based on the IP version.
func determineAddressFamily(addr *net.UDPAddr) int {
	if addr.IP.To4() == nil && addr.IP != nil {
		return unix.AF_INET6
	}
	return unix.AF_INET
}

// createAndConfigureSocket creates a UDP socket with SO_REUSEADDR and SO_REUSEPORT options.
func createAndConfigureSocket(family int) (int, error) {
	fd, err := unix.Socket(family, unix.SOCK_DGRAM, unix.IPPROTO_UDP)
	if err != nil {
		return 0, err
	}

	if err := unix.SetsockoptInt(fd, unix.SOL_SOCKET, unix.SO_REUSEADDR, 1); err != nil {
		unix.Close(fd)
		return 0, err
	}

	if err := unix.SetsockoptInt(fd, unix.SOL_SOCKET, unix.SO_REUSEPORT, 1); err != nil {
		unix.Close(fd)
		return 0, err
	}

	return fd, nil
}

// bindSocket binds the socket to the specified address.
func bindSocket(fd int, addr *net.UDPAddr, family int) error {
	sockaddr := buildSockaddr(addr, family)
	return syscall.Bind(fd, sockaddr)
}

// buildSockaddr constructs a syscall.Sockaddr from a net.UDPAddr.
func buildSockaddr(addr *net.UDPAddr, family int) syscall.Sockaddr {
	if family == unix.AF_INET6 {
		sa6 := &syscall.SockaddrInet6{Port: addr.Port}
		if addr.IP != nil {
			copy(sa6.Addr[:], addr.IP.To16())
		}
		return sa6
	}

	sa4 := &syscall.SockaddrInet4{Port: addr.Port}
	if addr.IP != nil {
		copy(sa4.Addr[:], addr.IP.To4())
	}
	return sa4
}

// convertToPacketConn converts a file descriptor to a net.PacketConn.
func convertToPacketConn(fd int) (net.PacketConn, error) {
	file := os.NewFile(uintptr(fd), fmt.Sprintf("udp-reuseport-%d", fd))
	conn, err := net.FilePacketConn(file)
	file.Close() // Close file, fd is duplicated by FilePacketConn
	if err != nil {
		unix.Close(fd)
		return nil, err
	}
	return conn, nil
}

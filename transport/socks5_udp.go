// Package transport provides network transport implementations for Tox.
package transport

import (
	"encoding/binary"
	"errors"
	"fmt"
	"io"
	"net"
	"sync"
	"time"

	"github.com/sirupsen/logrus"
)

// SOCKS5 address types per RFC 1928.
const (
	socks5AddrTypeIPv4   = 0x01
	socks5AddrTypeDomain = 0x03
	socks5AddrTypeIPv6   = 0x04
)

// SOCKS5 commands per RFC 1928.
const (
	socks5CmdConnect      = 0x01
	socks5CmdBind         = 0x02
	socks5CmdUDPAssociate = 0x03
)

// SOCKS5 reply codes per RFC 1928.
const (
	socks5ReplySuccess              = 0x00
	socks5ReplyGeneralFailure       = 0x01
	socks5ReplyConnectionNotAllowed = 0x02
	socks5ReplyNetworkUnreachable   = 0x03
	socks5ReplyHostUnreachable      = 0x04
	socks5ReplyConnectionRefused    = 0x05
	socks5ReplyTTLExpired           = 0x06
	socks5ReplyCmdNotSupported      = 0x07
	socks5ReplyAddrTypeNotSupported = 0x08
)

// SOCKS5UDPAssociation represents a UDP ASSOCIATE session with a SOCKS5 proxy.
// It manages the TCP control connection and the UDP relay socket for proxied
// UDP traffic per RFC 1928.
type SOCKS5UDPAssociation struct {
	tcpConn        net.Conn       // TCP control connection to the proxy
	udpConn        net.PacketConn // Local UDP socket for sending/receiving via relay
	relayAddr      net.Addr       // The proxy's UDP relay address
	proxyAddr      string         // Original proxy address for reconnection
	auth           *socks5Auth    // Authentication credentials
	mu             sync.RWMutex   // Protects the connection state
	closed         bool           // Whether the association is closed
	maxFragmentID  uint8          // Current fragment counter (0 = no fragmentation)
	lastActivity   time.Time      // Last packet sent/received time
	keepAliveTimer *time.Timer    // Keeps TCP connection alive
}

// socks5Auth holds SOCKS5 authentication credentials.
type socks5Auth struct {
	username string
	password string
}

// NewSOCKS5UDPAssociation creates a new UDP association with a SOCKS5 proxy.
// It performs the initial negotiation including authentication and the
// UDP ASSOCIATE command, then returns an association ready for use.
func NewSOCKS5UDPAssociation(proxyAddr, username, password string) (*SOCKS5UDPAssociation, error) {
	logrus.WithFields(logrus.Fields{
		"function":   "NewSOCKS5UDPAssociation",
		"proxy_addr": proxyAddr,
	}).Info("Creating new SOCKS5 UDP association")

	// Connect to the SOCKS5 proxy via TCP
	tcpConn, err := net.DialTimeout("tcp", proxyAddr, 10*time.Second)
	if err != nil {
		logrus.WithFields(logrus.Fields{
			"function":   "NewSOCKS5UDPAssociation",
			"proxy_addr": proxyAddr,
			"error":      err.Error(),
		}).Error("Failed to connect to SOCKS5 proxy")
		return nil, fmt.Errorf("failed to connect to SOCKS5 proxy: %w", err)
	}

	var auth *socks5Auth
	if username != "" || password != "" {
		auth = &socks5Auth{username: username, password: password}
	}

	association := &SOCKS5UDPAssociation{
		tcpConn:      tcpConn,
		proxyAddr:    proxyAddr,
		auth:         auth,
		lastActivity: time.Now(),
	}

	// Perform SOCKS5 handshake (method negotiation)
	if err := association.performHandshake(); err != nil {
		tcpConn.Close()
		return nil, fmt.Errorf("SOCKS5 handshake failed: %w", err)
	}

	// Send UDP ASSOCIATE command
	if err := association.sendUDPAssociateCommand(); err != nil {
		tcpConn.Close()
		return nil, fmt.Errorf("UDP ASSOCIATE failed: %w", err)
	}

	// Create local UDP socket for relay communication
	if err := association.createUDPSocket(); err != nil {
		tcpConn.Close()
		return nil, fmt.Errorf("failed to create UDP socket: %w", err)
	}

	// Start keep-alive timer to maintain the TCP control connection
	association.startKeepAlive()

	logrus.WithFields(logrus.Fields{
		"function":   "NewSOCKS5UDPAssociation",
		"proxy_addr": proxyAddr,
		"relay_addr": association.relayAddr.String(),
	}).Info("SOCKS5 UDP association established successfully")

	return association, nil
}

// performHandshake performs the SOCKS5 method negotiation per RFC 1928.
func (a *SOCKS5UDPAssociation) performHandshake() error {
	logrus.WithFields(logrus.Fields{
		"function": "performHandshake",
	}).Debug("Starting SOCKS5 method negotiation")

	// Determine authentication methods to offer
	var methods []byte
	if a.auth != nil {
		// Offer username/password auth (0x02) and no auth (0x00)
		methods = []byte{0x00, 0x02}
	} else {
		// Offer only no auth (0x00)
		methods = []byte{0x00}
	}

	// Send version identifier/method selection message
	// +----+----------+----------+
	// |VER | NMETHODS | METHODS  |
	// +----+----------+----------+
	// | 1  |    1     | 1-255    |
	// +----+----------+----------+
	request := make([]byte, 2+len(methods))
	request[0] = 0x05 // SOCKS version 5
	request[1] = byte(len(methods))
	copy(request[2:], methods)

	if err := a.writeWithTimeout(request); err != nil {
		return fmt.Errorf("failed to send method selection: %w", err)
	}

	// Read server's method selection response
	// +----+--------+
	// |VER | METHOD |
	// +----+--------+
	// | 1  |   1    |
	// +----+--------+
	response := make([]byte, 2)
	if err := a.readWithTimeout(response); err != nil {
		return fmt.Errorf("failed to read method selection: %w", err)
	}

	if response[0] != 0x05 {
		return fmt.Errorf("unsupported SOCKS version: %d", response[0])
	}

	selectedMethod := response[1]
	logrus.WithFields(logrus.Fields{
		"function":        "performHandshake",
		"selected_method": selectedMethod,
	}).Debug("Server selected authentication method")

	switch selectedMethod {
	case 0x00:
		// No authentication required
		return nil
	case 0x02:
		// Username/password authentication
		return a.performUsernamePasswordAuth()
	case 0xFF:
		return errors.New("no acceptable authentication methods")
	default:
		return fmt.Errorf("unsupported authentication method: %d", selectedMethod)
	}
}

// performUsernamePasswordAuth performs RFC 1929 username/password authentication.
func (a *SOCKS5UDPAssociation) performUsernamePasswordAuth() error {
	if a.auth == nil {
		return errors.New("server requires authentication but no credentials provided")
	}

	logrus.WithFields(logrus.Fields{
		"function": "performUsernamePasswordAuth",
	}).Debug("Performing username/password authentication")

	// Send username/password subnegotiation request per RFC 1929
	// +----+------+----------+------+----------+
	// |VER | ULEN |  UNAME   | PLEN |  PASSWD  |
	// +----+------+----------+------+----------+
	// | 1  |  1   | 1 to 255 |  1   | 1 to 255 |
	// +----+------+----------+------+----------+
	ulen := len(a.auth.username)
	plen := len(a.auth.password)
	request := make([]byte, 3+ulen+plen)
	request[0] = 0x01 // Subnegotiation version
	request[1] = byte(ulen)
	copy(request[2:2+ulen], a.auth.username)
	request[2+ulen] = byte(plen)
	copy(request[3+ulen:], a.auth.password)

	if err := a.writeWithTimeout(request); err != nil {
		return fmt.Errorf("failed to send auth request: %w", err)
	}

	// Read authentication response
	// +----+--------+
	// |VER | STATUS |
	// +----+--------+
	// | 1  |   1    |
	// +----+--------+
	response := make([]byte, 2)
	if err := a.readWithTimeout(response); err != nil {
		return fmt.Errorf("failed to read auth response: %w", err)
	}

	if response[1] != 0x00 {
		return errors.New("authentication failed")
	}

	logrus.WithFields(logrus.Fields{
		"function": "performUsernamePasswordAuth",
	}).Debug("Authentication successful")

	return nil
}

// sendUDPAssociateCommand sends the UDP ASSOCIATE command and parses the response.
func (a *SOCKS5UDPAssociation) sendUDPAssociateCommand() error {
	logrus.WithFields(logrus.Fields{
		"function": "sendUDPAssociateCommand",
	}).Debug("Sending UDP ASSOCIATE command")

	// Build UDP ASSOCIATE request
	// +----+-----+-------+------+----------+----------+
	// |VER | CMD |  RSV  | ATYP | DST.ADDR | DST.PORT |
	// +----+-----+-------+------+----------+----------+
	// | 1  |  1  | X'00' |  1   | Variable |    2     |
	// +----+-----+-------+------+----------+----------+
	// For UDP ASSOCIATE, DST.ADDR and DST.PORT specify the expected source
	// of incoming UDP packets. We use 0.0.0.0:0 to indicate any source.
	request := []byte{
		0x05,                   // VER: SOCKS5
		socks5CmdUDPAssociate,  // CMD: UDP ASSOCIATE
		0x00,                   // RSV: Reserved
		socks5AddrTypeIPv4,     // ATYP: IPv4
		0x00, 0x00, 0x00, 0x00, // DST.ADDR: 0.0.0.0
		0x00, 0x00, // DST.PORT: 0
	}

	if err := a.writeWithTimeout(request); err != nil {
		return fmt.Errorf("failed to send UDP ASSOCIATE request: %w", err)
	}

	// Read response
	// +----+-----+-------+------+----------+----------+
	// |VER | REP |  RSV  | ATYP | BND.ADDR | BND.PORT |
	// +----+-----+-------+------+----------+----------+
	// | 1  |  1  | X'00' |  1   | Variable |    2     |
	// +----+-----+-------+------+----------+----------+
	header := make([]byte, 4)
	if err := a.readWithTimeout(header); err != nil {
		return fmt.Errorf("failed to read UDP ASSOCIATE response header: %w", err)
	}

	if header[0] != 0x05 {
		return fmt.Errorf("invalid SOCKS version in response: %d", header[0])
	}

	if header[1] != socks5ReplySuccess {
		return fmt.Errorf("UDP ASSOCIATE failed with reply code: %d", header[1])
	}

	// Parse the bound address (the UDP relay address)
	relayAddr, err := a.readAddress(header[3])
	if err != nil {
		return fmt.Errorf("failed to parse relay address: %w", err)
	}

	a.relayAddr = relayAddr

	logrus.WithFields(logrus.Fields{
		"function":   "sendUDPAssociateCommand",
		"relay_addr": relayAddr.String(),
	}).Info("UDP ASSOCIATE successful, relay address received")

	return nil
}

// readAddress reads a SOCKS5 address from the TCP connection based on address type.
func (a *SOCKS5UDPAssociation) readAddress(addrType byte) (net.Addr, error) {
	switch addrType {
	case socks5AddrTypeIPv4:
		return a.readIPv4Address()
	case socks5AddrTypeIPv6:
		return a.readIPv6Address()
	case socks5AddrTypeDomain:
		return a.readDomainAddress()
	default:
		return nil, fmt.Errorf("unsupported address type: %d", addrType)
	}
}

// readIPv4Address reads a 4-byte IPv4 address plus 2-byte port from the connection.
func (a *SOCKS5UDPAssociation) readIPv4Address() (net.Addr, error) {
	buf := make([]byte, 6)
	if err := a.readWithTimeout(buf); err != nil {
		return nil, err
	}
	ip := net.IP(buf[:4])
	port := binary.BigEndian.Uint16(buf[4:])
	return &net.UDPAddr{IP: ip, Port: int(port)}, nil
}

// readIPv6Address reads a 16-byte IPv6 address plus 2-byte port from the connection.
func (a *SOCKS5UDPAssociation) readIPv6Address() (net.Addr, error) {
	buf := make([]byte, 18)
	if err := a.readWithTimeout(buf); err != nil {
		return nil, err
	}
	ip := net.IP(buf[:16])
	port := binary.BigEndian.Uint16(buf[16:])
	return &net.UDPAddr{IP: ip, Port: int(port)}, nil
}

// readDomainAddress reads a domain name and port, then resolves the domain to an IP.
func (a *SOCKS5UDPAssociation) readDomainAddress() (net.Addr, error) {
	lenBuf := make([]byte, 1)
	if err := a.readWithTimeout(lenBuf); err != nil {
		return nil, err
	}
	domainLen := int(lenBuf[0])
	buf := make([]byte, domainLen+2)
	if err := a.readWithTimeout(buf); err != nil {
		return nil, err
	}
	domain := string(buf[:domainLen])
	port := binary.BigEndian.Uint16(buf[domainLen:])
	ips, err := net.LookupIP(domain)
	if err != nil || len(ips) == 0 {
		return nil, fmt.Errorf("failed to resolve domain %s: %w", domain, err)
	}
	return &net.UDPAddr{IP: ips[0], Port: int(port)}, nil
}

// createUDPSocket creates the local UDP socket for relay communication.
func (a *SOCKS5UDPAssociation) createUDPSocket() error {
	// Create a UDP socket bound to any available port
	udpConn, err := net.ListenPacket("udp", ":0")
	if err != nil {
		return fmt.Errorf("failed to create local UDP socket: %w", err)
	}

	a.udpConn = udpConn

	logrus.WithFields(logrus.Fields{
		"function":   "createUDPSocket",
		"local_addr": udpConn.LocalAddr().String(),
	}).Debug("Local UDP socket created for relay communication")

	return nil
}

// SendUDP sends a UDP datagram through the SOCKS5 proxy to the specified destination.
// The data is encapsulated with the SOCKS5 UDP request header per RFC 1928.
func (a *SOCKS5UDPAssociation) SendUDP(data []byte, destAddr net.Addr) error {
	a.mu.RLock()
	if a.closed {
		a.mu.RUnlock()
		return errors.New("association is closed")
	}
	relayAddr := a.relayAddr
	udpConn := a.udpConn
	a.mu.RUnlock()

	// Build UDP relay request
	// +----+------+------+----------+----------+----------+
	// |RSV | FRAG | ATYP | DST.ADDR | DST.PORT |   DATA   |
	// +----+------+------+----------+----------+----------+
	// | 2  |  1   |  1   | Variable |    2     | Variable |
	// +----+------+------+----------+----------+----------+
	header, err := a.buildUDPHeader(destAddr)
	if err != nil {
		return fmt.Errorf("failed to build UDP header: %w", err)
	}

	packet := make([]byte, len(header)+len(data))
	copy(packet, header)
	copy(packet[len(header):], data)

	n, err := udpConn.WriteTo(packet, relayAddr)
	if err != nil {
		logrus.WithFields(logrus.Fields{
			"function":   "SendUDP",
			"dest_addr":  destAddr.String(),
			"relay_addr": relayAddr.String(),
			"error":      err.Error(),
		}).Error("Failed to send UDP packet via relay")
		return fmt.Errorf("failed to send via relay: %w", err)
	}

	a.mu.Lock()
	a.lastActivity = time.Now()
	a.mu.Unlock()

	logrus.WithFields(logrus.Fields{
		"function":   "SendUDP",
		"dest_addr":  destAddr.String(),
		"bytes_sent": n,
	}).Debug("UDP packet sent via SOCKS5 relay")

	return nil
}

// buildUDPHeader constructs the SOCKS5 UDP relay header for the given destination.
func (a *SOCKS5UDPAssociation) buildUDPHeader(destAddr net.Addr) ([]byte, error) {
	if destAddr == nil {
		return nil, fmt.Errorf("buildUDPHeader: destination address is nil")
	}

	ip, port, err := extractDestIPAndPort(destAddr)
	if err != nil {
		return nil, fmt.Errorf("buildUDPHeader: %w", err)
	}
	return buildSOCKS5UDPRelayHeader(ip, port), nil
}

// extractDestIPAndPort extracts IP and port from a destination address.
func extractDestIPAndPort(destAddr net.Addr) (net.IP, int, error) {
	if destAddr == nil {
		return nil, 0, fmt.Errorf("extractDestIPAndPort: destination address is nil")
	}
	switch addr := destAddr.(type) {
	case *net.UDPAddr:
		return addr.IP, addr.Port, nil
	default:
		return parseIPAndPortFromString(destAddr.String())
	}
}

// parseIPAndPortFromString parses IP and port from a host:port string, resolving hostnames if needed.
func parseIPAndPortFromString(addrStr string) (net.IP, int, error) {
	host, portStr, err := net.SplitHostPort(addrStr)
	if err != nil {
		return nil, 0, fmt.Errorf("unsupported address format: %w", err)
	}
	ip := net.ParseIP(host)
	if ip == nil {
		ips, err := net.LookupIP(host)
		if err != nil || len(ips) == 0 {
			return nil, 0, fmt.Errorf("failed to resolve %s: %w", host, err)
		}
		ip = ips[0]
	}
	var port int
	_, err = fmt.Sscanf(portStr, "%d", &port)
	if err != nil {
		return nil, 0, fmt.Errorf("invalid port: %w", err)
	}
	if port < 0 || port > 65535 {
		return nil, 0, fmt.Errorf("port out of range: %d", port)
	}
	return ip, port, nil
}

// buildSOCKS5UDPRelayHeader builds the SOCKS5 UDP relay header bytes for the given IP and port.
func buildSOCKS5UDPRelayHeader(ip net.IP, port int) []byte {
	if ip4 := ip.To4(); ip4 != nil {
		header := make([]byte, 10) // 2 (RSV) + 1 (FRAG) + 1 (ATYP) + 4 (IP) + 2 (PORT)
		header[3] = socks5AddrTypeIPv4
		copy(header[4:8], ip4)
		binary.BigEndian.PutUint16(header[8:10], uint16(port))
		return header
	}
	header := make([]byte, 22) // 2 (RSV) + 1 (FRAG) + 1 (ATYP) + 16 (IP) + 2 (PORT)
	header[3] = socks5AddrTypeIPv6
	copy(header[4:20], ip.To16())
	binary.BigEndian.PutUint16(header[20:22], uint16(port))
	return header
}

// ReceiveUDP receives a UDP datagram from the SOCKS5 relay.
// It strips the SOCKS5 header and returns the payload along with the source address.
func (a *SOCKS5UDPAssociation) ReceiveUDP(buffer []byte) (int, net.Addr, error) {
	a.mu.RLock()
	if a.closed {
		a.mu.RUnlock()
		return 0, nil, errors.New("association is closed")
	}
	udpConn := a.udpConn
	a.mu.RUnlock()

	// Create a temporary buffer to hold the full relay packet
	tempBuf := make([]byte, len(buffer)+22) // max header size is 22 bytes (IPv6)

	n, _, err := udpConn.ReadFrom(tempBuf)
	if err != nil {
		return 0, nil, fmt.Errorf("failed to read from relay: %w", err)
	}

	if n < 10 { // Minimum header size (IPv4)
		return 0, nil, errors.New("packet too small to contain SOCKS5 header")
	}

	// Parse the source address from the relay header
	sourceAddr, headerLen, err := a.parseUDPHeader(tempBuf[:n])
	if err != nil {
		return 0, nil, fmt.Errorf("failed to parse UDP header: %w", err)
	}

	// Copy payload to the output buffer
	payloadLen := n - headerLen
	if payloadLen > len(buffer) {
		payloadLen = len(buffer)
	}
	copy(buffer, tempBuf[headerLen:headerLen+payloadLen])

	a.mu.Lock()
	a.lastActivity = time.Now()
	a.mu.Unlock()

	logrus.WithFields(logrus.Fields{
		"function":     "ReceiveUDP",
		"source_addr":  sourceAddr.String(),
		"payload_size": payloadLen,
	}).Debug("UDP packet received via SOCKS5 relay")

	return payloadLen, sourceAddr, nil
}

// parseUDPHeader parses the SOCKS5 UDP relay header and returns the source address
// and the header length.
func (a *SOCKS5UDPAssociation) parseUDPHeader(data []byte) (net.Addr, int, error) {
	if len(data) < 4 {
		return nil, 0, errors.New("header too short")
	}

	// Skip RSV (2 bytes) and FRAG (1 byte)
	addrType := data[3]

	switch addrType {
	case socks5AddrTypeIPv4:
		return parseIPv4Header(data)
	case socks5AddrTypeIPv6:
		return parseIPv6Header(data)
	case socks5AddrTypeDomain:
		return parseDomainHeader(data)
	default:
		return nil, 0, fmt.Errorf("unsupported address type: %d", addrType)
	}
}

// parseIPv4Header extracts an IPv4 address from a SOCKS5 UDP header.
func parseIPv4Header(data []byte) (net.Addr, int, error) {
	if len(data) < 10 {
		return nil, 0, errors.New("IPv4 header too short")
	}
	ip := net.IP(data[4:8])
	port := binary.BigEndian.Uint16(data[8:10])
	return &net.UDPAddr{IP: ip, Port: int(port)}, 10, nil
}

// parseIPv6Header extracts an IPv6 address from a SOCKS5 UDP header.
func parseIPv6Header(data []byte) (net.Addr, int, error) {
	if len(data) < 22 {
		return nil, 0, errors.New("IPv6 header too short")
	}
	ip := net.IP(data[4:20])
	port := binary.BigEndian.Uint16(data[20:22])
	return &net.UDPAddr{IP: ip, Port: int(port)}, 22, nil
}

// parseDomainHeader extracts a domain address from a SOCKS5 UDP header.
func parseDomainHeader(data []byte) (net.Addr, int, error) {
	if len(data) < 5 {
		return nil, 0, errors.New("domain header too short")
	}
	domainLen := int(data[4])
	if len(data) < 5+domainLen+2 {
		return nil, 0, errors.New("domain header incomplete")
	}
	domain := string(data[5 : 5+domainLen])
	port := binary.BigEndian.Uint16(data[5+domainLen : 5+domainLen+2])
	addr := resolveDomainToUDPAddr(domain, int(port))
	return addr, 5 + domainLen + 2, nil
}

// resolveDomainToUDPAddr resolves a domain name to a UDPAddr.
func resolveDomainToUDPAddr(domain string, port int) *net.UDPAddr {
	addr := &net.UDPAddr{IP: net.ParseIP(domain), Port: port}
	if addr.IP == nil {
		// Domain didn't parse as IP, resolve it
		if ips, err := net.LookupIP(domain); err == nil && len(ips) > 0 {
			addr.IP = ips[0]
		}
	}
	return addr
}

// startKeepAlive starts a periodic keep-alive mechanism to maintain the TCP
// control connection. SOCKS5 proxies may close idle connections.
func (a *SOCKS5UDPAssociation) startKeepAlive() {
	a.keepAliveTimer = time.AfterFunc(30*time.Second, func() {
		a.mu.RLock()
		closed := a.closed
		tcpConn := a.tcpConn
		a.mu.RUnlock()

		if closed {
			return
		}

		// Check if connection is still alive by setting a deadline
		// and attempting a zero-byte read
		if err := tcpConn.SetReadDeadline(time.Now().Add(1 * time.Second)); err != nil {
			return
		}

		oneByte := make([]byte, 1)
		_, err := tcpConn.Read(oneByte)
		if err != nil && !isTimeoutError(err) {
			// Connection is dead, close the association
			logrus.WithFields(logrus.Fields{
				"function": "startKeepAlive",
				"error":    err.Error(),
			}).Warn("SOCKS5 TCP control connection lost")
			a.Close()
			return
		}

		// Reset deadline and reschedule
		tcpConn.SetReadDeadline(time.Time{})
		a.startKeepAlive()
	})
}

// isTimeoutError checks if an error is a timeout error.
func isTimeoutError(err error) bool {
	if netErr, ok := err.(net.Error); ok && netErr.Timeout() {
		return true
	}
	return false
}

// writeWithTimeout writes data to the TCP connection with a timeout.
func (a *SOCKS5UDPAssociation) writeWithTimeout(data []byte) error {
	if err := a.tcpConn.SetWriteDeadline(time.Now().Add(10 * time.Second)); err != nil {
		return err
	}
	_, err := a.tcpConn.Write(data)
	return err
}

// readWithTimeout reads exactly len(data) bytes from the TCP connection with a timeout.
func (a *SOCKS5UDPAssociation) readWithTimeout(data []byte) error {
	if err := a.tcpConn.SetReadDeadline(time.Now().Add(10 * time.Second)); err != nil {
		return err
	}
	_, err := io.ReadFull(a.tcpConn, data)
	return err
}

// Close terminates the UDP association and releases all resources.
func (a *SOCKS5UDPAssociation) Close() error {
	a.mu.Lock()
	defer a.mu.Unlock()

	if a.closed {
		return nil
	}
	a.closed = true

	logrus.WithFields(logrus.Fields{
		"function": "SOCKS5UDPAssociation.Close",
	}).Info("Closing SOCKS5 UDP association")

	a.stopKeepAliveTimer()
	return a.closeConnections()
}

// stopKeepAliveTimer stops the keep-alive timer if active.
func (a *SOCKS5UDPAssociation) stopKeepAliveTimer() {
	if a.keepAliveTimer != nil {
		a.keepAliveTimer.Stop()
	}
}

// closeConnections closes both UDP and TCP connections, collecting errors.
func (a *SOCKS5UDPAssociation) closeConnections() error {
	var errs []error
	if a.udpConn != nil {
		if err := a.udpConn.Close(); err != nil {
			errs = append(errs, err)
		}
	}
	if a.tcpConn != nil {
		if err := a.tcpConn.Close(); err != nil {
			errs = append(errs, err)
		}
	}
	if len(errs) > 0 {
		return fmt.Errorf("errors closing association: %v", errs)
	}
	return nil
}

// LocalAddr returns the local UDP address used for relay communication.
func (a *SOCKS5UDPAssociation) LocalAddr() net.Addr {
	a.mu.RLock()
	defer a.mu.RUnlock()
	if a.udpConn != nil {
		return a.udpConn.LocalAddr()
	}
	return nil
}

// RelayAddr returns the proxy's UDP relay address.
func (a *SOCKS5UDPAssociation) RelayAddr() net.Addr {
	a.mu.RLock()
	defer a.mu.RUnlock()
	return a.relayAddr
}

// IsClosed returns whether the association has been closed.
func (a *SOCKS5UDPAssociation) IsClosed() bool {
	a.mu.RLock()
	defer a.mu.RUnlock()
	return a.closed
}

// SetReadDeadline sets the read deadline on the UDP socket.
func (a *SOCKS5UDPAssociation) SetReadDeadline(t time.Time) error {
	a.mu.RLock()
	defer a.mu.RUnlock()
	if a.udpConn != nil {
		return a.udpConn.SetReadDeadline(t)
	}
	return errors.New("UDP socket not initialized")
}

// SetWriteDeadline sets the write deadline on the UDP socket.
func (a *SOCKS5UDPAssociation) SetWriteDeadline(t time.Time) error {
	a.mu.RLock()
	defer a.mu.RUnlock()
	if a.udpConn != nil {
		return a.udpConn.SetWriteDeadline(t)
	}
	return errors.New("UDP socket not initialized")
}

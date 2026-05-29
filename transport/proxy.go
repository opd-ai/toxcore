package transport

import (
	"bufio"
	"fmt"
	"net"
	"net/http"
	"net/url"
	"sync"
	"time"

	"github.com/sirupsen/logrus"
	"golang.org/x/net/proxy"
)

// ProxyTransport wraps an existing transport to route traffic through a proxy.
// It supports SOCKS5 and HTTP CONNECT proxies for TCP connections.
// For SOCKS5 proxies with UDP enabled, UDP traffic is also routed through the
// proxy using the SOCKS5 UDP ASSOCIATE command per RFC 1928.
type ProxyTransport struct {
	underlying        Transport
	proxyDialer       proxy.Dialer
	proxyType         string
	proxyAddr         string
	httpProxyURL      *url.URL
	connections       map[string]net.Conn
	mu                sync.RWMutex
	udpAssociation    *SOCKS5UDPAssociation // UDP relay for SOCKS5 proxies
	udpProxyEnabled   bool                  // Whether UDP proxying is enabled
	udpProxyRequested bool                  // Whether UDP proxy was explicitly requested (true even if failed)
	username          string                // Stored for UDP association re-establishment
	// password is retained only when udpProxyEnabled is true; it is required to
	// re-create the SOCKS5 UDP ASSOCIATE if the relay connection drops.
	// When UDP proxying is disabled the field is deliberately left empty.
	password string
}

// ProxyConfig contains configuration for proxy connections.
type ProxyConfig struct {
	Type            string // "socks5" or "http"
	Host            string
	Port            uint16
	Username        string
	Password        string
	UDPProxyEnabled bool // Enable UDP proxying for SOCKS5 (uses UDP ASSOCIATE)
}

// NewProxyTransport wraps a transport to use the specified proxy.
// The underlying transport is used for listening, while outbound connections
// go through the proxy when the transport uses TCP.
// For SOCKS5 proxies, if UDPProxyEnabled is true, UDP traffic will be routed
// through the proxy using the SOCKS5 UDP ASSOCIATE command.
func NewProxyTransport(underlying Transport, config *ProxyConfig) (*ProxyTransport, error) {
	if config == nil {
		return nil, fmt.Errorf("proxy config cannot be nil")
	}

	proxyAddr := fmt.Sprintf("%s:%d", config.Host, config.Port)
	logProxyCreation(config, proxyAddr)

	dialer, httpProxyURL, err := createProxyDialer(config, proxyAddr)
	if err != nil {
		return nil, err
	}

	pt := buildProxyTransport(underlying, config, proxyAddr, dialer, httpProxyURL)
	if err := pt.enableSOCKS5UDPProxy(config.Password); err != nil {
		return nil, err
	}
	logProxyReady(pt, config.Type)
	return pt, nil
}

func logProxyCreation(config *ProxyConfig, proxyAddr string) {
	logrus.WithFields(logrus.Fields{
		"function":          "NewProxyTransport",
		"proxy_type":        config.Type,
		"proxy_addr":        proxyAddr,
		"udp_proxy_enabled": config.UDPProxyEnabled,
	}).Info("Creating proxy transport")
}

// buildProxyTransport constructs the proxy transport state for later setup.
func buildProxyTransport(underlying Transport, config *ProxyConfig, proxyAddr string, dialer proxy.Dialer, httpProxyURL *url.URL) *ProxyTransport {
	udpEnabled := config.UDPProxyEnabled && config.Type == "socks5"
	return &ProxyTransport{
		underlying:        underlying,
		proxyDialer:       dialer,
		proxyType:         config.Type,
		proxyAddr:         proxyAddr,
		httpProxyURL:      httpProxyURL,
		connections:       make(map[string]net.Conn),
		udpProxyEnabled:   udpEnabled,
		udpProxyRequested: udpEnabled,
		username:          config.Username,
	}
}

// enableSOCKS5UDPProxy establishes the SOCKS5 UDP association when requested.
func (t *ProxyTransport) enableSOCKS5UDPProxy(password string) error {
	if !t.udpProxyRequested {
		return nil
	}
	association, err := NewSOCKS5UDPAssociation(t.proxyAddr, t.username, password)
	if err != nil {
		logrus.WithFields(logrus.Fields{
			"function":   "NewProxyTransport",
			"proxy_addr": t.proxyAddr,
			"error":      err.Error(),
		}).Error("Failed to establish SOCKS5 UDP association")
		return fmt.Errorf("UDP proxy explicitly requested for SOCKS5 but failed to establish association: %w", err)
	}
	t.password = password
	t.udpAssociation = association
	logrus.WithFields(logrus.Fields{
		"function":   "NewProxyTransport",
		"proxy_addr": t.proxyAddr,
		"relay_addr": association.RelayAddr().String(),
	}).Info("SOCKS5 UDP association established successfully")
	return nil
}

func logProxyReady(t *ProxyTransport, proxyType string) {
	logrus.WithFields(logrus.Fields{
		"function":         "NewProxyTransport",
		"proxy_type":       proxyType,
		"proxy_addr":       t.proxyAddr,
		"udp_proxy_active": t.udpAssociation != nil,
	}).Info("Proxy transport created successfully")
}

// createProxyDialer creates the appropriate proxy dialer based on configuration.
func createProxyDialer(config *ProxyConfig, proxyAddr string) (proxy.Dialer, *url.URL, error) {
	switch config.Type {
	case "socks5":
		return createSocks5Dialer(config, proxyAddr)
	case "http":
		return createHTTPDialer(config, proxyAddr)
	default:
		return nil, nil, fmt.Errorf("unsupported proxy type: %s (must be 'socks5' or 'http')", config.Type)
	}
}

// createSocks5Dialer creates a SOCKS5 proxy dialer with optional authentication.
func createSocks5Dialer(config *ProxyConfig, proxyAddr string) (proxy.Dialer, *url.URL, error) {
	var auth *proxy.Auth
	if config.Username != "" || config.Password != "" {
		auth = &proxy.Auth{
			User:     config.Username,
			Password: config.Password,
		}
	}

	dialer, err := proxy.SOCKS5("tcp", proxyAddr, auth, proxy.Direct)
	if err != nil {
		logrus.WithFields(logrus.Fields{
			"function":   "NewProxyTransport",
			"proxy_type": config.Type,
			"proxy_addr": proxyAddr,
			"error":      err.Error(),
		}).Error("Failed to create SOCKS5 dialer")
		return nil, nil, fmt.Errorf("failed to create SOCKS5 dialer: %w", err)
	}

	return dialer, nil, nil
}

// createHTTPDialer creates an HTTP CONNECT proxy dialer with optional authentication.
func createHTTPDialer(config *ProxyConfig, proxyAddr string) (proxy.Dialer, *url.URL, error) {
	var userInfo *url.Userinfo
	if config.Username != "" {
		if config.Password != "" {
			userInfo = url.UserPassword(config.Username, config.Password)
		} else {
			userInfo = url.User(config.Username)
		}
	}

	httpProxyURL := &url.URL{
		Scheme: "http",
		Host:   proxyAddr,
		User:   userInfo,
	}

	dialer := &httpProxyDialer{proxyURL: httpProxyURL}

	logrus.WithFields(logrus.Fields{
		"function":   "NewProxyTransport",
		"proxy_type": config.Type,
		"proxy_addr": proxyAddr,
	}).Info("HTTP CONNECT proxy configured")

	return dialer, httpProxyURL, nil
}

// Send sends a packet through the proxy for TCP connections.
// For UDP-based underlying transports with SOCKS5 UDP enabled, routes UDP through
// the proxy using UDP ASSOCIATE. Otherwise, delegates to the underlying transport.
// For TCP-based transports, establishes connections through the configured proxy.
func (t *ProxyTransport) Send(packet *Packet, addr net.Addr) error {
	logrus.WithFields(logrus.Fields{
		"function":    "ProxyTransport.Send",
		"packet_type": packet.PacketType,
		"dest_addr":   addr.String(),
		"proxy_type":  t.proxyType,
	}).Debug("Sending packet via proxy transport")

	// Check if underlying transport is connection-oriented using interface method
	if t.underlying.IsConnectionOriented() {
		return t.sendViaTCPProxy(packet, addr)
	}

	// For connectionless transports, check if we have UDP proxy support
	t.mu.RLock()
	udpAssociation := t.udpAssociation
	udpProxyRequested := t.udpProxyRequested
	t.mu.RUnlock()

	if udpAssociation != nil && !udpAssociation.IsClosed() {
		return t.sendViaUDPProxy(packet, addr)
	}

	// If UDP proxy was explicitly requested but is unavailable, return an error
	// instead of falling back to direct UDP (which would expose the real IP)
	if udpProxyRequested {
		return fmt.Errorf("UDP proxy was explicitly requested but is unavailable or closed")
	}

	// No UDP proxy was requested - use underlying transport for direct send
	return t.underlying.Send(packet, addr)
}

// sendViaUDPProxy sends a packet through the SOCKS5 UDP relay.
func (t *ProxyTransport) sendViaUDPProxy(packet *Packet, addr net.Addr) error {
	udpAssociation := t.getUDPAssociation()
	if udpAssociation == nil {
		return fmt.Errorf("UDP association not available")
	}

	data, err := serializeProxyPacket(packet, "sendViaUDPProxy")
	if err != nil {
		return err
	}
	if err := udpAssociation.SendUDP(data, addr); err != nil {
		return t.retryUDPProxySend(packet, addr, data, udpAssociation, err)
	}
	logUDPSend(packet, addr, len(data))
	return nil
}

func (t *ProxyTransport) getUDPAssociation() *SOCKS5UDPAssociation {
	t.mu.RLock()
	defer t.mu.RUnlock()
	return t.udpAssociation
}

func serializeProxyPacket(packet *Packet, functionName string) ([]byte, error) {
	data, err := packet.Serialize()
	if err != nil {
		logrus.WithFields(logrus.Fields{
			"function":    functionName,
			"packet_type": packet.PacketType,
			"error":       err.Error(),
		}).Error("Failed to serialize packet")
		return nil, err
	}
	return data, nil
}

// retryUDPProxySend re-establishes the UDP association if the relay is closed.
func (t *ProxyTransport) retryUDPProxySend(packet *Packet, addr net.Addr, data []byte, association *SOCKS5UDPAssociation, sendErr error) error {
	if !association.IsClosed() {
		return fmt.Errorf("UDP proxy send failed: %w", sendErr)
	}
	newAssociation, err := t.reestablishUDPAssociation(packet.PacketType)
	if err != nil {
		return err
	}
	if err := newAssociation.SendUDP(data, addr); err != nil {
		return fmt.Errorf("UDP proxy send failed: %w", err)
	}
	logUDPSend(packet, addr, len(data))
	return nil
}

func (t *ProxyTransport) reestablishUDPAssociation(packetType PacketType) (*SOCKS5UDPAssociation, error) {
	t.mu.Lock()
	defer t.mu.Unlock()
	newAssociation, err := NewSOCKS5UDPAssociation(t.proxyAddr, t.username, t.password)
	if err != nil {
		logrus.WithFields(logrus.Fields{
			"function":    "sendViaUDPProxy",
			"packet_type": packetType,
			"error":       err.Error(),
		}).Error("Failed to re-establish UDP association")
		return nil, fmt.Errorf("UDP association failed: %w", err)
	}
	t.udpAssociation = newAssociation
	return newAssociation, nil
}

func logUDPSend(packet *Packet, addr net.Addr, bytesSent int) {
	logrus.WithFields(logrus.Fields{
		"function":    "sendViaUDPProxy",
		"packet_type": packet.PacketType,
		"dest_addr":   addr.String(),
		"bytes_sent":  bytesSent,
	}).Debug("Packet sent via SOCKS5 UDP relay")
}

// sendViaTCPProxy sends a packet through TCP proxy by establishing a proxied connection.
func (t *ProxyTransport) sendViaTCPProxy(packet *Packet, addr net.Addr) error {
	conn, err := t.getOrCreateProxyConnection(addr)
	if err != nil {
		logrus.WithFields(logrus.Fields{
			"function":    "sendViaTCPProxy",
			"packet_type": packet.PacketType,
			"dest_addr":   addr.String(),
			"error":       err.Error(),
		}).Error("Failed to get proxy connection")
		return fmt.Errorf("proxy connection failed: %w", err)
	}

	data, err := packet.Serialize()
	if err != nil {
		logrus.WithFields(logrus.Fields{
			"function":    "sendViaTCPProxy",
			"packet_type": packet.PacketType,
			"error":       err.Error(),
		}).Error("Failed to serialize packet")
		return err
	}

	// Write with timeout
	if err := conn.SetWriteDeadline(time.Now().Add(5 * time.Second)); err != nil {
		return err
	}

	n, err := conn.Write(data)
	if err != nil {
		t.cleanupConnection(addr)
		logrus.WithFields(logrus.Fields{
			"function":    "sendViaTCPProxy",
			"packet_type": packet.PacketType,
			"dest_addr":   addr.String(),
			"error":       err.Error(),
		}).Error("Failed to write to proxy connection")
		return fmt.Errorf("proxy write failed: %w", err)
	}

	logrus.WithFields(logrus.Fields{
		"function":    "sendViaTCPProxy",
		"packet_type": packet.PacketType,
		"dest_addr":   addr.String(),
		"bytes_sent":  n,
	}).Debug("Packet sent via proxy successfully")

	return nil
}

// getOrCreateProxyConnection returns an existing proxy connection or creates a new one.
// A write-lock double-check prevents two concurrent callers from each dialing and
// the second overwriting the first, leaking the first TCP connection (F-TRANS-H3).
func (t *ProxyTransport) getOrCreateProxyConnection(addr net.Addr) (net.Conn, error) {
	addrKey := addr.String()
	if conn := t.getProxyConnection(addrKey); conn != nil {
		return conn, nil
	}
	newConn, err := t.dialProxyConnection(addrKey)
	if err != nil {
		return nil, err
	}
	return t.storeProxyConnection(addrKey, newConn), nil
}

func (t *ProxyTransport) getProxyConnection(addrKey string) net.Conn {
	t.mu.RLock()
	defer t.mu.RUnlock()
	return t.connections[addrKey]
}

// dialProxyConnection opens a new TCP connection through the configured proxy.
func (t *ProxyTransport) dialProxyConnection(addrKey string) (net.Conn, error) {
	logrus.WithFields(logrus.Fields{
		"function":   "getOrCreateProxyConnection",
		"dest_addr":  addrKey,
		"proxy_type": t.proxyType,
		"proxy_addr": t.proxyAddr,
	}).Info("Establishing new connection via proxy")
	newConn, err := t.proxyDialer.Dial("tcp", addrKey)
	if err != nil {
		logrus.WithFields(logrus.Fields{
			"function":   "getOrCreateProxyConnection",
			"dest_addr":  addrKey,
			"proxy_type": t.proxyType,
			"error":      err.Error(),
		}).Error("Failed to dial through proxy")
		return nil, fmt.Errorf("proxy dial failed: %w", err)
	}
	return newConn, nil
}

func (t *ProxyTransport) storeProxyConnection(addrKey string, newConn net.Conn) net.Conn {
	t.mu.Lock()
	defer t.mu.Unlock()
	if existing, raced := t.connections[addrKey]; raced {
		newConn.Close()
		return existing
	}
	t.connections[addrKey] = newConn
	logrus.WithFields(logrus.Fields{
		"function":    "getOrCreateProxyConnection",
		"dest_addr":   addrKey,
		"proxy_type":  t.proxyType,
		"local_addr":  newConn.LocalAddr().String(),
		"remote_addr": newConn.RemoteAddr().String(),
	}).Info("Proxy connection established successfully")
	return newConn
}

// cleanupConnection removes and closes a connection.
func (t *ProxyTransport) cleanupConnection(addr net.Addr) {
	addrKey := addr.String()

	t.mu.Lock()
	conn, exists := t.connections[addrKey]
	if exists {
		delete(t.connections, addrKey)
	}
	t.mu.Unlock()

	if conn != nil {
		conn.Close()
	}
}

// Close closes the proxy transport and underlying transport.
func (t *ProxyTransport) Close() error {
	logrus.WithFields(logrus.Fields{
		"function":   "ProxyTransport.Close",
		"proxy_type": t.proxyType,
		"proxy_addr": t.proxyAddr,
	}).Info("Closing proxy transport")

	// Close UDP association if present
	t.mu.Lock()
	if t.udpAssociation != nil {
		if err := t.udpAssociation.Close(); err != nil {
			logrus.WithFields(logrus.Fields{
				"function": "ProxyTransport.Close",
				"error":    err.Error(),
			}).Warn("Error closing UDP association")
		}
		t.udpAssociation = nil
	}

	// Close all proxy connections
	for _, conn := range t.connections {
		conn.Close()
	}
	t.connections = make(map[string]net.Conn)
	t.mu.Unlock()

	return t.underlying.Close()
}

// LocalAddr returns the local address from the underlying transport.
func (t *ProxyTransport) LocalAddr() net.Addr {
	return t.underlying.LocalAddr()
}

// RegisterHandler registers a packet handler with the underlying transport.
func (t *ProxyTransport) RegisterHandler(packetType PacketType, handler PacketHandler) {
	logrus.WithFields(logrus.Fields{
		"function":    "ProxyTransport.RegisterHandler",
		"packet_type": packetType,
		"proxy_type":  t.proxyType,
	}).Debug("Registering handler via proxy transport")

	t.underlying.RegisterHandler(packetType, handler)
}

// IsConnectionOriented delegates to the underlying transport.
func (t *ProxyTransport) IsConnectionOriented() bool {
	return t.underlying.IsConnectionOriented()
}

// DialProxy establishes a connection to a remote address via the proxy.
// This is useful for TCP connections that need to go through the proxy.
func (t *ProxyTransport) DialProxy(address string) (net.Conn, error) {
	t.mu.RLock()
	dialer := t.proxyDialer
	proxyType := t.proxyType
	proxyAddr := t.proxyAddr
	t.mu.RUnlock()

	logrus.WithFields(logrus.Fields{
		"function":   "ProxyTransport.DialProxy",
		"address":    address,
		"proxy_type": proxyType,
		"proxy_addr": proxyAddr,
	}).Debug("Dialing via proxy")

	conn, err := dialer.Dial("tcp", address)
	if err != nil {
		logrus.WithFields(logrus.Fields{
			"function":   "ProxyTransport.DialProxy",
			"address":    address,
			"proxy_type": proxyType,
			"error":      err.Error(),
		}).Error("Failed to dial via proxy")
		return nil, fmt.Errorf("proxy dial failed: %w", err)
	}

	logrus.WithFields(logrus.Fields{
		"function":    "ProxyTransport.DialProxy",
		"address":     address,
		"proxy_type":  proxyType,
		"local_addr":  conn.LocalAddr().String(),
		"remote_addr": conn.RemoteAddr().String(),
	}).Info("Proxy connection established")

	return conn, nil
}

// GetProxyDialer returns the underlying proxy dialer for advanced usage.
func (t *ProxyTransport) GetProxyDialer() proxy.Dialer {
	t.mu.RLock()
	defer t.mu.RUnlock()
	return t.proxyDialer
}

// IsUDPProxyEnabled returns whether UDP proxying is active.
func (t *ProxyTransport) IsUDPProxyEnabled() bool {
	t.mu.RLock()
	defer t.mu.RUnlock()
	return t.udpAssociation != nil && !t.udpAssociation.IsClosed()
}

// GetUDPAssociation returns the SOCKS5 UDP association for advanced usage.
// Returns nil if UDP proxying is not enabled.
func (t *ProxyTransport) GetUDPAssociation() *SOCKS5UDPAssociation {
	t.mu.RLock()
	defer t.mu.RUnlock()
	return t.udpAssociation
}

// httpProxyDialer implements the proxy.Dialer interface for HTTP CONNECT proxies.
type httpProxyDialer struct {
	proxyURL *url.URL
}

// Dial connects to the address via HTTP CONNECT proxy.
func (d *httpProxyDialer) Dial(network, addr string) (net.Conn, error) {
	if network != "tcp" {
		return nil, fmt.Errorf("HTTP CONNECT proxy only supports TCP, got: %s", network)
	}

	proxyConn, err := net.DialTimeout("tcp", d.proxyURL.Host, 10*time.Second)
	if err != nil {
		return nil, fmt.Errorf("failed to connect to proxy: %w", err)
	}

	connectReq := d.createConnectRequest(addr)

	if err := d.sendConnectRequest(proxyConn, connectReq); err != nil {
		proxyConn.Close()
		return nil, err
	}

	if err := d.readConnectResponse(proxyConn, connectReq); err != nil {
		proxyConn.Close()
		return nil, err
	}

	return proxyConn, nil
}

// createConnectRequest builds an HTTP CONNECT request with authentication.
func (d *httpProxyDialer) createConnectRequest(addr string) *http.Request {
	connectReq := &http.Request{
		Method: "CONNECT",
		URL:    &url.URL{Opaque: addr},
		Host:   addr,
		Header: make(http.Header),
	}

	if d.proxyURL.User != nil {
		username := d.proxyURL.User.Username()
		password, _ := d.proxyURL.User.Password()
		// SetBasicAuth with empty password is valid for HTTP proxies that require
		// only a username; the boolean return of Password() is intentionally ignored
		// here because HTTP Basic Auth transmits an empty password rather than omitting it.
		connectReq.SetBasicAuth(username, password)
	}

	return connectReq
}

// sendConnectRequest writes the CONNECT request to the proxy connection.
func (d *httpProxyDialer) sendConnectRequest(conn net.Conn, req *http.Request) error {
	if err := req.Write(conn); err != nil {
		return fmt.Errorf("failed to write CONNECT request: %w", err)
	}
	return nil
}

// readConnectResponse reads and validates the proxy server's CONNECT response.
func (d *httpProxyDialer) readConnectResponse(conn net.Conn, req *http.Request) error {
	if err := conn.SetReadDeadline(time.Now().Add(10 * time.Second)); err != nil {
		return fmt.Errorf("failed to set read deadline: %w", err)
	}

	resp, err := http.ReadResponse(bufio.NewReader(conn), req)
	if err != nil {
		return fmt.Errorf("failed to read CONNECT response: %w", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		return fmt.Errorf("proxy returned non-200 status: %s", resp.Status)
	}

	return nil
}

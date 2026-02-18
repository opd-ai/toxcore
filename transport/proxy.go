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
// Note: UDP traffic is not proxied (passed through to underlying transport).
type ProxyTransport struct {
	underlying   Transport
	proxyDialer  proxy.Dialer
	proxyType    string
	proxyAddr    string
	httpProxyURL *url.URL
	connections  map[string]net.Conn
	mu           sync.RWMutex
}

// ProxyConfig contains configuration for proxy connections.
type ProxyConfig struct {
	Type     string // "socks5" or "http"
	Host     string
	Port     uint16
	Username string
	Password string
}

// NewProxyTransport wraps a transport to use the specified proxy.
// The underlying transport is used for listening, while outbound connections
// go through the proxy when the transport uses TCP.
func NewProxyTransport(underlying Transport, config *ProxyConfig) (*ProxyTransport, error) {
	if config == nil {
		return nil, fmt.Errorf("proxy config cannot be nil")
	}

	proxyAddr := fmt.Sprintf("%s:%d", config.Host, config.Port)

	logrus.WithFields(logrus.Fields{
		"function":   "NewProxyTransport",
		"proxy_type": config.Type,
		"proxy_addr": proxyAddr,
	}).Info("Creating proxy transport")

	var dialer proxy.Dialer
	var httpProxyURL *url.URL
	var err error

	switch config.Type {
	case "socks5":
		// Create SOCKS5 dialer with optional authentication
		var auth *proxy.Auth
		if config.Username != "" || config.Password != "" {
			auth = &proxy.Auth{
				User:     config.Username,
				Password: config.Password,
			}
		}

		dialer, err = proxy.SOCKS5("tcp", proxyAddr, auth, proxy.Direct)
		if err != nil {
			logrus.WithFields(logrus.Fields{
				"function":   "NewProxyTransport",
				"proxy_type": config.Type,
				"proxy_addr": proxyAddr,
				"error":      err.Error(),
			}).Error("Failed to create SOCKS5 dialer")
			return nil, fmt.Errorf("failed to create SOCKS5 dialer: %w", err)
		}

	case "http":
		// Create HTTP CONNECT proxy URL
		scheme := "http"
		var userInfo *url.Userinfo
		if config.Username != "" {
			if config.Password != "" {
				userInfo = url.UserPassword(config.Username, config.Password)
			} else {
				userInfo = url.User(config.Username)
			}
		}

		httpProxyURL = &url.URL{
			Scheme: scheme,
			Host:   proxyAddr,
			User:   userInfo,
		}

		// Use HTTP proxy via custom dialer
		dialer = &httpProxyDialer{proxyURL: httpProxyURL}

		logrus.WithFields(logrus.Fields{
			"function":   "NewProxyTransport",
			"proxy_type": config.Type,
			"proxy_addr": proxyAddr,
		}).Info("HTTP CONNECT proxy configured")

	default:
		return nil, fmt.Errorf("unsupported proxy type: %s (must be 'socks5' or 'http')", config.Type)
	}

	logrus.WithFields(logrus.Fields{
		"function":   "NewProxyTransport",
		"proxy_type": config.Type,
		"proxy_addr": proxyAddr,
	}).Info("Proxy transport created successfully")

	return &ProxyTransport{
		underlying:   underlying,
		proxyDialer:  dialer,
		proxyType:    config.Type,
		proxyAddr:    proxyAddr,
		httpProxyURL: httpProxyURL,
		connections:  make(map[string]net.Conn),
	}, nil
}

// Send sends a packet through the proxy for TCP connections.
// For UDP-based underlying transports, delegates to the underlying transport.
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

	// For connectionless transports, delegate to underlying transport
	// Note: Full UDP proxy support would require SOCKS5 UDP association
	// WARNING: UDP traffic bypasses the proxy and may leak the user's real IP address
	logrus.WithFields(logrus.Fields{
		"function":    "ProxyTransport.Send",
		"packet_type": packet.PacketType,
		"dest_addr":   addr.String(),
		"proxy_type":  t.proxyType,
	}).Warn("UDP traffic bypassing proxy - sent directly without proxy protection (real IP may be exposed)")

	return t.underlying.Send(packet, addr)
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
func (t *ProxyTransport) getOrCreateProxyConnection(addr net.Addr) (net.Conn, error) {
	addrKey := addr.String()

	t.mu.RLock()
	conn, exists := t.connections[addrKey]
	t.mu.RUnlock()

	if exists {
		return conn, nil
	}

	// Create new connection through proxy
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

	t.mu.Lock()
	t.connections[addrKey] = newConn
	t.mu.Unlock()

	logrus.WithFields(logrus.Fields{
		"function":    "getOrCreateProxyConnection",
		"dest_addr":   addrKey,
		"proxy_type":  t.proxyType,
		"local_addr":  newConn.LocalAddr().String(),
		"remote_addr": newConn.RemoteAddr().String(),
	}).Info("Proxy connection established successfully")

	return newConn, nil
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

	// Close all proxy connections
	t.mu.Lock()
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

// httpProxyDialer implements the proxy.Dialer interface for HTTP CONNECT proxies.
type httpProxyDialer struct {
	proxyURL *url.URL
}

// Dial connects to the address via HTTP CONNECT proxy.
func (d *httpProxyDialer) Dial(network, addr string) (net.Conn, error) {
	if network != "tcp" {
		return nil, fmt.Errorf("HTTP CONNECT proxy only supports TCP, got: %s", network)
	}

	// Connect to proxy server
	proxyConn, err := net.DialTimeout("tcp", d.proxyURL.Host, 10*time.Second)
	if err != nil {
		return nil, fmt.Errorf("failed to connect to proxy: %w", err)
	}

	// Send CONNECT request
	connectReq := &http.Request{
		Method: "CONNECT",
		URL:    &url.URL{Opaque: addr},
		Host:   addr,
		Header: make(http.Header),
	}

	// Add proxy authentication if present
	if d.proxyURL.User != nil {
		username := d.proxyURL.User.Username()
		password, _ := d.proxyURL.User.Password()
		connectReq.SetBasicAuth(username, password)
	}

	// Write CONNECT request
	if err := connectReq.Write(proxyConn); err != nil {
		proxyConn.Close()
		return nil, fmt.Errorf("failed to write CONNECT request: %w", err)
	}

	// Read CONNECT response
	if err := proxyConn.SetReadDeadline(time.Now().Add(10 * time.Second)); err != nil {
		proxyConn.Close()
		return nil, fmt.Errorf("failed to set read deadline: %w", err)
	}

	resp, err := http.ReadResponse(bufio.NewReader(proxyConn), connectReq)
	if err != nil {
		proxyConn.Close()
		return nil, fmt.Errorf("failed to read CONNECT response: %w", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		proxyConn.Close()
		return nil, fmt.Errorf("proxy returned non-200 status: %s", resp.Status)
	}

	return proxyConn, nil
}

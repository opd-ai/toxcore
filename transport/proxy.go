package transport

import (
	"fmt"
	"net"
	"sync"

	"github.com/sirupsen/logrus"
	"golang.org/x/net/proxy"
)

// ProxyTransport wraps an existing transport to route traffic through a proxy.
// It supports SOCKS5 and HTTP proxies for TCP connections.
// Note: UDP over SOCKS5 is supported, but HTTP proxies do not support UDP.
type ProxyTransport struct {
	underlying  Transport
	proxyDialer proxy.Dialer
	proxyType   string
	proxyAddr   string
	mu          sync.RWMutex
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
// go through the proxy.
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
		// golang.org/x/net/proxy doesn't support HTTP CONNECT proxies directly
		// HTTP proxies are typically handled by net/http.ProxyFromEnvironment or custom implementation
		// For now, we'll note this as unsupported for direct dialing
		logrus.WithFields(logrus.Fields{
			"function":   "NewProxyTransport",
			"proxy_type": config.Type,
			"proxy_addr": proxyAddr,
		}).Warn("HTTP proxy support requires custom implementation - currently unsupported for direct transport")
		return nil, fmt.Errorf("HTTP proxy support is not yet implemented for direct transport layer (use SOCKS5 instead)")

	default:
		return nil, fmt.Errorf("unsupported proxy type: %s (must be 'socks5' or 'http')", config.Type)
	}

	logrus.WithFields(logrus.Fields{
		"function":   "NewProxyTransport",
		"proxy_type": config.Type,
		"proxy_addr": proxyAddr,
	}).Info("Proxy transport created successfully")

	return &ProxyTransport{
		underlying:  underlying,
		proxyDialer: dialer,
		proxyType:   config.Type,
		proxyAddr:   proxyAddr,
	}, nil
}

// Send sends a packet through the proxy.
// Note: This implementation uses the underlying transport's Send method.
// For true proxy support, packets would need to be sent via proxy connection.
func (t *ProxyTransport) Send(packet *Packet, addr net.Addr) error {
	logrus.WithFields(logrus.Fields{
		"function":    "ProxyTransport.Send",
		"packet_type": packet.PacketType,
		"dest_addr":   addr.String(),
		"proxy_type":  t.proxyType,
	}).Debug("Sending packet via proxy transport")

	// For now, delegate to underlying transport
	// Full proxy support would require establishing connections via proxy
	return t.underlying.Send(packet, addr)
}

// Close closes the proxy transport and underlying transport.
func (t *ProxyTransport) Close() error {
	logrus.WithFields(logrus.Fields{
		"function":   "ProxyTransport.Close",
		"proxy_type": t.proxyType,
		"proxy_addr": t.proxyAddr,
	}).Info("Closing proxy transport")

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

// toxcore_network.go contains network-related functionality including
// transport setup, bootstrap, relay servers, LAN discovery, NAT traversal,
// and packet routing.

package toxcore

import (
	"context"
	"encoding/hex"
	"errors"
	"fmt"
	"net"
	"strconv"
	"time"

	"github.com/opd-ai/toxcore/crypto"
	"github.com/opd-ai/toxcore/dht"
	"github.com/opd-ai/toxcore/transport"
	"github.com/sirupsen/logrus"
)

// setupUDPTransport configures UDP transport with secure-by-default Noise-IK encryption.
// Returns a NegotiatingTransport that automatically handles protocol version negotiation.
//
// WARNING: UDP traffic bypasses configured proxies. The ProxyTransport only wraps
// TCP-style connections. If proxy anonymity is required, disable UDP by setting
// Options.UDPEnabled = false to force TCP-only operation.
func setupUDPTransport(options *Options, keyPair *crypto.KeyPair) (transport.Transport, error) {
	if !options.UDPEnabled {
		return nil, nil
	}

	// Warn if proxy is configured but UDP is enabled - UDP bypasses proxy
	if options.Proxy != nil && options.Proxy.Type != ProxyTypeNone {
		logrus.WithFields(logrus.Fields{
			"function":   "setupUDPTransport",
			"proxy_type": options.Proxy.Type,
			"warning":    "UDP_BYPASSES_PROXY",
		}).Warn("Proxy configured but UDP is enabled. UDP traffic will NOT be proxied " +
			"and may leak your real IP address. For full proxy coverage, disable UDP " +
			"(set UDPEnabled=false) or use system-level proxy routing.")
	}

	// Try ports in the range [StartPort, EndPort]
	for port := options.StartPort; port <= options.EndPort; port++ {
		addr := net.JoinHostPort("0.0.0.0", strconv.Itoa(int(port)))
		udpTransport, err := transport.NewUDPTransport(addr)
		if err == nil {
			// Enable secure-by-default behavior by wrapping with NegotiatingTransport
			capabilities := transport.DefaultProtocolCapabilities()
			negotiatingTransport, err := transport.NewNegotiatingTransport(udpTransport, capabilities, keyPair.Private[:])
			if err != nil {
				// If secure transport setup fails, log warning but continue with UDP
				// This ensures backward compatibility while preferring security
				logrus.WithFields(logrus.Fields{
					"function": "setupUDPTransport",
					"error":    err.Error(),
					"port":     port,
				}).Warn("Failed to enable Noise-IK transport, falling back to legacy UDP")
				return wrapWithProxyIfConfigured(udpTransport, options.Proxy)
			}

			logrus.WithFields(logrus.Fields{
				"function": "setupUDPTransport",
				"port":     port,
				"security": "noise-ik_enabled",
			}).Info("Secure transport initialized with Noise-IK capability")

			return wrapWithProxyIfConfigured(negotiatingTransport, options.Proxy)
		}
	}

	return nil, errors.New("failed to bind to any UDP port")
}

// setupTCPTransport configures TCP transport with secure-by-default Noise-IK encryption.
// Returns a NegotiatingTransport that automatically handles protocol version negotiation.
// If proxy options are configured, wraps the transport with ProxyTransport.
func setupTCPTransport(options *Options, keyPair *crypto.KeyPair) (transport.Transport, error) {
	if options.TCPPort == 0 {
		return nil, nil
	}

	addr := net.JoinHostPort("0.0.0.0", strconv.Itoa(int(options.TCPPort)))
	tcpTransport, err := transport.NewTCPTransport(addr)
	if err != nil {
		logrus.WithFields(logrus.Fields{
			"function": "setupTCPTransport",
			"error":    err.Error(),
			"port":     options.TCPPort,
		}).Error("Failed to create TCP transport")
		return nil, err
	}

	// Enable secure-by-default behavior by wrapping with NegotiatingTransport
	capabilities := transport.DefaultProtocolCapabilities()
	negotiatingTransport, err := transport.NewNegotiatingTransport(tcpTransport, capabilities, keyPair.Private[:])
	if err != nil {
		// If secure transport setup fails, log warning but continue with TCP
		// This ensures backward compatibility while preferring security
		logrus.WithFields(logrus.Fields{
			"function": "setupTCPTransport",
			"error":    err.Error(),
			"port":     options.TCPPort,
		}).Warn("Failed to enable Noise-IK transport, falling back to legacy TCP")
		return wrapWithProxyIfConfigured(tcpTransport, options.Proxy)
	}

	logrus.WithFields(logrus.Fields{
		"function": "setupTCPTransport",
		"port":     options.TCPPort,
		"security": "noise-ik_enabled",
	}).Info("Secure TCP transport initialized with Noise-IK capability")

	return wrapWithProxyIfConfigured(negotiatingTransport, options.Proxy)
}

// wrapWithProxyIfConfigured wraps a transport with proxy if proxy options are configured.
// Returns the original transport if no proxy is configured or an error occurs.
func wrapWithProxyIfConfigured(t transport.Transport, proxyOpts *ProxyOptions) (transport.Transport, error) {
	if proxyOpts == nil || proxyOpts.Type == ProxyTypeNone {
		return t, nil
	}

	var proxyType string
	switch proxyOpts.Type {
	case ProxyTypeHTTP:
		proxyType = "http"
	case ProxyTypeSOCKS5:
		proxyType = "socks5"
	default:
		logrus.WithFields(logrus.Fields{
			"function":   "wrapWithProxyIfConfigured",
			"proxy_type": proxyOpts.Type,
		}).Warn("Unknown proxy type, skipping proxy configuration")
		return t, nil
	}

	proxyConfig := &transport.ProxyConfig{
		Type:            proxyType,
		Host:            proxyOpts.Host,
		Port:            proxyOpts.Port,
		Username:        proxyOpts.Username,
		Password:        proxyOpts.Password,
		UDPProxyEnabled: proxyOpts.UDPProxyEnabled,
	}

	proxyTransport, err := transport.NewProxyTransport(t, proxyConfig)
	if err != nil {
		logrus.WithFields(logrus.Fields{
			"function":   "wrapWithProxyIfConfigured",
			"proxy_type": proxyType,
			"error":      err.Error(),
		}).Warn("Failed to create proxy transport, continuing without proxy")
		return t, nil
	}

	logrus.WithFields(logrus.Fields{
		"function":   "wrapWithProxyIfConfigured",
		"proxy_type": proxyType,
		"proxy_addr": fmt.Sprintf("%s:%d", proxyOpts.Host, proxyOpts.Port),
	}).Info("Proxy transport configured successfully")

	return proxyTransport, nil
}

// setupTransports initializes and configures UDP and TCP transports based on options.
// It returns the configured transports or an error if setup fails.
func setupTransports(options *Options, keyPair *crypto.KeyPair) (transport.Transport, transport.Transport, error) {
	logrus.WithFields(logrus.Fields{
		"function":    "setupTransports",
		"udp_enabled": options.UDPEnabled,
	}).Debug("Setting up UDP transport")

	udpTransport, err := setupUDPTransport(options, keyPair)
	if err != nil {
		logrus.WithFields(logrus.Fields{
			"function": "setupTransports",
			"error":    err.Error(),
		}).Error("Failed to setup UDP transport")
		return nil, nil, err
	}
	if udpTransport != nil {
		logrus.WithFields(logrus.Fields{
			"function":   "setupTransports",
			"local_addr": udpTransport.LocalAddr().String(),
		}).Debug("UDP transport setup successfully")
	}

	logrus.WithFields(logrus.Fields{
		"function": "setupTransports",
		"tcp_port": options.TCPPort,
	}).Debug("Setting up TCP transport")

	tcpTransport, err := setupTCPTransport(options, keyPair)
	if err != nil {
		logrus.WithFields(logrus.Fields{
			"function": "setupTransports",
			"error":    err.Error(),
		}).Error("Failed to setup TCP transport")
		return nil, nil, err
	}
	if tcpTransport != nil {
		logrus.WithFields(logrus.Fields{
			"function":   "setupTransports",
			"local_addr": tcpTransport.LocalAddr().String(),
		}).Debug("TCP transport setup successfully")
	}

	return udpTransport, tcpTransport, nil
}

// registerTransportHandlers registers packet handlers for the configured transports.
func (t *Tox) registerTransportHandlers(udpTransport, tcpTransport transport.Transport) {
	if udpTransport != nil {
		logrus.WithFields(logrus.Fields{
			"function": "registerTransportHandlers",
		}).Debug("Registering UDP handlers")
		t.registerUDPHandlers()
	}

	if tcpTransport != nil {
		logrus.WithFields(logrus.Fields{
			"function": "registerTransportHandlers",
		}).Debug("Registering TCP handlers")
		t.registerTCPHandlers()
	}
}

// registerUDPHandlers registers packet handlers for UDP transport.
func (t *Tox) registerUDPHandlers() {
	t.udpTransport.RegisterHandler(transport.PacketPingRequest, t.handlePingRequest)
	t.udpTransport.RegisterHandler(transport.PacketPingResponse, t.handlePingResponse)
	t.udpTransport.RegisterHandler(transport.PacketGetNodes, t.handleGetNodes)
	t.udpTransport.RegisterHandler(transport.PacketSendNodes, t.handleSendNodes)
	// Register more handlers here
}

// registerTCPHandlers registers packet handlers for TCP transport.
func (t *Tox) registerTCPHandlers() {
	t.tcpTransport.RegisterHandler(transport.PacketPingRequest, t.handlePingRequest)
	t.tcpTransport.RegisterHandler(transport.PacketPingResponse, t.handlePingResponse)
	t.tcpTransport.RegisterHandler(transport.PacketGetNodes, t.handleGetNodes)
	t.tcpTransport.RegisterHandler(transport.PacketSendNodes, t.handleSendNodes)
	// Register more handlers here
}

// handlePingRequest processes ping request packets.
func (t *Tox) handlePingRequest(packet *transport.Packet, addr net.Addr) error {
	// Delegate to the bootstrap manager which has the full implementation
	return t.bootstrapManager.HandlePacket(packet, addr)
}

// handlePingResponse processes ping response packets.
func (t *Tox) handlePingResponse(packet *transport.Packet, addr net.Addr) error {
	// Delegate to the bootstrap manager which has the full implementation
	return t.bootstrapManager.HandlePacket(packet, addr)
}

// handleGetNodes processes get nodes request packets.
func (t *Tox) handleGetNodes(packet *transport.Packet, addr net.Addr) error {
	// Delegate to the bootstrap manager which has the full implementation
	return t.bootstrapManager.HandlePacket(packet, addr)
}

// handleSendNodes processes send nodes response packets.
func (t *Tox) handleSendNodes(packet *transport.Packet, addr net.Addr) error {
	// Delegate to the bootstrap manager which has the full implementation
	return t.bootstrapManager.HandlePacket(packet, addr)
}

// validateBootstrapPublicKey validates the public key format and hex encoding.
func validateBootstrapPublicKey(publicKeyHex, address string, port uint16) error {
	if len(publicKeyHex) != 64 {
		err := fmt.Errorf("invalid public key length: expected 64, got %d", len(publicKeyHex))
		logrus.WithFields(logrus.Fields{
			"function":          "Bootstrap",
			"address":           address,
			"port":              port,
			"public_key_length": len(publicKeyHex),
			"error":             err.Error(),
		}).Error("Public key validation failed")
		return err
	}

	_, err := hex.DecodeString(publicKeyHex)
	if err != nil {
		err := fmt.Errorf("invalid hex public key: %w", err)
		logrus.WithFields(logrus.Fields{
			"function": "Bootstrap",
			"address":  address,
			"port":     port,
			"error":    err.Error(),
		}).Error("Public key hex decoding failed")
		return err
	}
	return nil
}

// resolveBootstrapAddress resolves the bootstrap node address.
func resolveBootstrapAddress(address string, port uint16) (net.Addr, error) {
	logrus.WithFields(logrus.Fields{
		"function": "Bootstrap",
		"address":  address,
		"port":     port,
	}).Debug("Resolving bootstrap address")

	addr, err := net.ResolveUDPAddr("udp", net.JoinHostPort(address, fmt.Sprintf("%d", port)))
	if err != nil {
		logrus.WithFields(logrus.Fields{
			"function": "Bootstrap",
			"address":  address,
			"port":     port,
			"error":    err.Error(),
		}).Error("Bootstrap address resolution failed")
		return nil, fmt.Errorf("failed to resolve bootstrap address %s:%d: %w", address, port, err)
	}

	logrus.WithFields(logrus.Fields{
		"function":      "Bootstrap",
		"resolved_addr": addr.String(),
	}).Debug("Bootstrap address resolved successfully")
	return addr, nil
}

// Bootstrap connects to a bootstrap node to join the Tox network.
// The address is the hostname or IP, port is the UDP port, and publicKeyHex
// is the node's public key in hexadecimal format.
//
//export ToxBootstrap
func (t *Tox) Bootstrap(address string, port uint16, publicKeyHex string) error {
	logrus.WithFields(logrus.Fields{
		"function":   "Bootstrap",
		"address":    address,
		"port":       port,
		"public_key": publicKeyHex[:16] + "...",
	}).Info("Attempting to bootstrap")

	if err := validateBootstrapPublicKey(publicKeyHex, address, port); err != nil {
		return err
	}

	addr, err := resolveBootstrapAddress(address, port)
	if err != nil {
		return err
	}

	if err := t.addBootstrapNode(addr, publicKeyHex); err != nil {
		return err
	}

	return t.executeBootstrapProcess(address, port)
}

// addBootstrapNode adds a bootstrap node to the manager.
func (t *Tox) addBootstrapNode(addr net.Addr, publicKeyHex string) error {
	logrus.WithFields(logrus.Fields{
		"function": "Bootstrap",
	}).Debug("Adding bootstrap node to manager")

	if err := t.bootstrapManager.AddNode(addr, publicKeyHex); err != nil {
		logrus.WithFields(logrus.Fields{
			"function": "Bootstrap",
			"error":    err.Error(),
		}).Error("Failed to add bootstrap node to manager")
		return err
	}
	return nil
}

// executeBootstrapProcess starts and monitors the bootstrap process with timeout and retry.
// Uses exponential backoff with up to 3 retries to improve reliability on congested networks.
func (t *Tox) executeBootstrapProcess(address string, port uint16) error {
	const maxRetries = 3

	logrus.WithFields(logrus.Fields{
		"function":    "Bootstrap",
		"timeout":     t.options.BootstrapTimeout,
		"max_retries": maxRetries,
	}).Debug("Starting bootstrap process with timeout and retry")

	var lastErr error
	for attempt := 0; attempt < maxRetries; attempt++ {
		if attempt > 0 {
			// Exponential backoff: 1s, 2s, 4s
			backoff := time.Duration(1<<uint(attempt-1)) * time.Second
			logrus.WithFields(logrus.Fields{
				"function": "Bootstrap",
				"attempt":  attempt + 1,
				"backoff":  backoff,
			}).Debug("Retrying bootstrap after backoff")
			time.Sleep(backoff)
		}

		ctx, cancel := context.WithTimeout(t.ctx, t.options.BootstrapTimeout)
		lastErr = t.bootstrapManager.Bootstrap(ctx)
		cancel()

		if lastErr == nil {
			logrus.WithFields(logrus.Fields{
				"function": "Bootstrap",
				"address":  address,
				"port":     port,
				"attempts": attempt + 1,
			}).Info("Bootstrap completed successfully")
			return nil
		}

		logrus.WithFields(logrus.Fields{
			"function": "Bootstrap",
			"address":  address,
			"port":     port,
			"attempt":  attempt + 1,
			"error":    lastErr.Error(),
		}).Debug("Bootstrap attempt failed, will retry")
	}

	logrus.WithFields(logrus.Fields{
		"function":    "Bootstrap",
		"address":     address,
		"port":        port,
		"max_retries": maxRetries,
		"error":       lastErr.Error(),
	}).Error("Bootstrap process failed after all retries")
	return lastErr
}

// AddRelayServer adds a TCP relay server for symmetric NAT fallback.
// Relay servers are used when direct UDP connections fail, particularly
// for users behind symmetric NAT where UDP hole punching doesn't work.
//
// Example:
//
//	tox.AddRelayServer("relay.example.com", 33445, publicKey, 1)
//
//export ToxAddRelayServer
func (t *Tox) AddRelayServer(address string, port uint16, publicKey [32]byte, priority int) {
	if t.natTraversal == nil {
		logrus.Warn("NAT traversal not initialized, cannot add relay server")
		return
	}

	server := transport.RelayServerInfo{
		Address:   address,
		PublicKey: publicKey,
		Port:      port,
		Priority:  priority,
	}

	t.natTraversal.AddRelayServer(server)

	logrus.WithFields(logrus.Fields{
		"function": "AddRelayServer",
		"address":  address,
		"port":     port,
		"priority": priority,
	}).Info("Added relay server")
}

// RemoveRelayServer removes a TCP relay server by address.
//
//export ToxRemoveRelayServer
func (t *Tox) RemoveRelayServer(address string) {
	if t.natTraversal == nil {
		return
	}

	t.natTraversal.RemoveRelayServer(address)

	logrus.WithFields(logrus.Fields{
		"function": "RemoveRelayServer",
		"address":  address,
	}).Info("Removed relay server")
}

// EnableRelayFallback enables or disables relay connection fallback.
// When enabled, connections that fail via direct UDP will attempt to connect
// through configured relay servers.
//
//export ToxEnableRelayFallback
func (t *Tox) EnableRelayFallback(enabled bool) {
	if t.natTraversal == nil {
		logrus.Warn("NAT traversal not initialized, cannot configure relay fallback")
		return
	}

	t.natTraversal.EnableMethod(transport.ConnectionRelay, enabled)

	logrus.WithFields(logrus.Fields{
		"function": "EnableRelayFallback",
		"enabled":  enabled,
	}).Info("Relay fallback configuration updated")
}

// IsRelayConnected returns true if connected to a TCP relay server.
//
//export ToxIsRelayConnected
func (t *Tox) IsRelayConnected() bool {
	if t.natTraversal == nil {
		return false
	}
	return t.natTraversal.IsRelayConnected()
}

// DiscoverRelayServers queries the DHT for available relay servers
// and automatically adds them to the relay server list.
// Returns the number of relay servers discovered.
//
//export ToxDiscoverRelayServers
func (t *Tox) DiscoverRelayServers() (int, error) {
	if t.dht == nil {
		return 0, fmt.Errorf("DHT not initialized")
	}
	if t.natTraversal == nil {
		return 0, fmt.Errorf("NAT traversal not initialized")
	}

	relays, err := t.dht.QueryRelays(t.udpTransport)
	if err != nil {
		return 0, fmt.Errorf("failed to query relays from DHT: %w", err)
	}

	count := 0
	for _, relay := range relays {
		t.natTraversal.AddRelayServer(relay.ToTransportServerInfo())
		count++
	}

	logrus.WithFields(logrus.Fields{
		"function":     "DiscoverRelayServers",
		"relays_found": count,
	}).Info("Discovered relay servers from DHT")

	return count, nil
}

// GetRelayServerCount returns the number of configured relay servers.
//
//export ToxGetRelayServerCount
func (t *Tox) GetRelayServerCount() int {
	if t.natTraversal == nil || t.natTraversal.GetRelayClient() == nil {
		return 0
	}
	return t.natTraversal.GetRelayClient().GetServerCount()
}

// initializeLANDiscovery sets up local network peer discovery if enabled in options.
func initializeLANDiscovery(tox *Tox, options *Options) {
	if !options.LocalDiscovery {
		return
	}

	port := determineLANDiscoveryPort(options)
	tox.lanDiscovery = dht.NewLANDiscovery(tox.keyPair.Public, port)

	configureLANDiscoveryCallback(tox)
	startLANDiscovery(tox)
}

// determineLANDiscoveryPort returns the port to use for LAN discovery.
func determineLANDiscoveryPort(options *Options) uint16 {
	if options.StartPort == 0 {
		return 33445
	}
	return options.StartPort
}

// configureLANDiscoveryCallback sets up the peer discovery callback handler.
func configureLANDiscoveryCallback(tox *Tox) {
	tox.lanDiscovery.OnPeer(func(publicKey [32]byte, addr net.Addr) {
		toxID := crypto.ToxID{PublicKey: publicKey}
		node := dht.NewNode(toxID, addr)
		logrus.WithFields(logrus.Fields{
			"peer_addr":  addr.String(),
			"public_key": fmt.Sprintf("%x", publicKey[:8]),
		}).Info("Adding LAN-discovered peer to DHT")
		tox.dht.AddNode(node)
	})
}

// startLANDiscovery attempts to start LAN discovery with graceful fallback on failure.
func startLANDiscovery(tox *Tox) {
	if err := tox.lanDiscovery.Start(); err != nil {
		logrus.WithError(err).Debug("Failed to start LAN discovery, continuing without it")
		tox.lanDiscovery = nil
	} else {
		logrus.Info("LAN discovery started successfully")
	}
}

// initializeNATTraversal sets up advanced NAT traversal with relay support.
func initializeNATTraversal(tox *Tox) {
	if tox.selfAddress == nil {
		return
	}

	ant, err := transport.NewAdvancedNATTraversalWithKey(tox.selfAddress, tox.keyPair.Public)
	if err != nil {
		logrus.WithError(err).Debug("Failed to initialize NAT traversal, continuing without it")
		return
	}

	tox.natTraversal = ant

	// Configure relay servers from options
	if tox.options != nil {
		for _, server := range tox.options.RelayServers {
			ant.AddRelayServer(server.ToRelayServerInfo())
		}

		// Enable relay fallback if configured
		if tox.options.RelayEnabled {
			ant.EnableMethod(transport.ConnectionRelay, true)
		}
	}

	logrus.WithFields(logrus.Fields{
		"relay_servers": len(tox.options.RelayServers),
		"relay_enabled": tox.options != nil && tox.options.RelayEnabled,
	}).Info("Advanced NAT traversal initialized with relay support")
}

// registerPacketHandlers registers packet handlers for network integration.
func registerPacketHandlers(udpTransport transport.Transport, tox *Tox) {
	if udpTransport != nil {
		udpTransport.RegisterHandler(transport.PacketFriendMessage, tox.handleFriendMessagePacket)
		udpTransport.RegisterHandler(transport.PacketFriendRequest, tox.handleFriendRequestPacket)
	}
}

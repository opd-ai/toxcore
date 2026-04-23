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
	"github.com/opd-ai/toxcore/interfaces"
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

// resolveFriendAddress determines the network address for a friend using DHT lookup.
func (t *Tox) resolveFriendAddress(friend *Friend) (net.Addr, error) {
	t.dhtMutex.RLock()
	dht := t.dht
	t.dhtMutex.RUnlock()

	if dht == nil {
		return nil, fmt.Errorf("DHT not available for address resolution")
	}

	// Create ToxID from friend's public key for DHT lookup
	friendToxID := crypto.ToxID{
		PublicKey: friend.PublicKey,
		Nospam:    [4]byte{}, // Unknown nospam, but DHT uses public key for routing
		Checksum:  [2]byte{}, // Checksum not needed for DHT lookup
	}

	// Find closest nodes to the friend in our routing table
	closestNodes := dht.FindClosestNodes(friendToxID, 1)
	if len(closestNodes) > 0 && closestNodes[0].Address != nil {
		return closestNodes[0].Address, nil
	}

	return nil, fmt.Errorf("failed to resolve network address for friend via DHT lookup")
}

// resolveFriendIDFromAddress attempts to find a friend ID from a network address.
// This performs a reverse lookup through the DHT to find which friend is associated
// with the given address. Returns an error if no friend is found.
func (t *Tox) resolveFriendIDFromAddress(addr net.Addr) (uint32, error) {
	if t.dht == nil {
		return 0, fmt.Errorf("DHT not available for reverse address resolution")
	}

	// Search through DHT nodes to find one matching this address
	// and then check if that public key belongs to a friend
	nodes := t.dht.GetAllNodes()
	for _, node := range nodes {
		if node.Address != nil && node.Address.String() == addr.String() {
			// Found a matching node, check if this public key is a friend
			friendID, exists := t.getFriendIDByPublicKey(node.ID.PublicKey)
			if exists {
				return friendID, nil
			}
		}
	}

	return 0, fmt.Errorf("no friend found for address: %s", addr.String())
}

// sendPacketToTarget transmits a packet to the specified network address using the UDP transport.
func (t *Tox) sendPacketToTarget(packet *transport.Packet, targetAddr net.Addr) error {
	if t.udpTransport == nil {
		return fmt.Errorf("no transport available")
	}

	err := t.udpTransport.Send(packet, targetAddr)
	if err != nil {
		return fmt.Errorf("failed to send packet: %w", err)
	}

	return nil
}

// sendPacketToFriend resolves a friend's address and sends a packet to them.
// This is a convenience method that combines address resolution with packet transmission.
func (t *Tox) sendPacketToFriend(friendID uint32, friend *Friend, data []byte, packetType transport.PacketType) error {
	// Resolve friend's network address
	friendAddr, err := t.resolveFriendAddress(friend)
	if err != nil {
		return fmt.Errorf("failed to resolve friend address: %w", err)
	}

	// Check if transport is available
	if t.udpTransport == nil {
		return fmt.Errorf("no transport available")
	}

	// Create transport packet
	transportPacket := &transport.Packet{
		PacketType: packetType,
		Data:       data,
	}

	// Send packet to friend
	if err := t.udpTransport.Send(transportPacket, friendAddr); err != nil {
		return fmt.Errorf("failed to send packet to friend: %w", err)
	}

	return nil
}

// validateFriendConnection validates that a friend exists and is connected.
// Returns the friend object if validation passes, otherwise returns an error.
func (t *Tox) validateFriendConnection(friendID uint32) (*Friend, error) {
	return t.validateFriendOnline(friendID, "friend is not connected")
}

// simulatePacketDelivery simulates packet delivery for testing purposes.
// DEPRECATED: This method is deprecated in favor of the new packet delivery interface.
// Use packetDelivery.DeliverPacket() instead.
// In a real implementation, this would go through the transport layer.
func (t *Tox) simulatePacketDelivery(friendID uint32, packet []byte) {
	logrus.Warn("SIMULATION FUNCTION - NOT A REAL OPERATION")
	logrus.WithFields(logrus.Fields{
		"function":    "simulatePacketDelivery",
		"friend_id":   friendID,
		"packet_size": len(packet),
		"deprecated":  true,
	}).Warn("Using deprecated simulatePacketDelivery - consider migrating to packet delivery interface")

	// Use the new packet delivery interface if available.
	if d := t.loadDelivery(); d != nil {
		err := d.DeliverPacket(friendID, packet)
		if err != nil {
			logrus.WithFields(logrus.Fields{
				"function":  "simulatePacketDelivery",
				"friend_id": friendID,
				"error":     err.Error(),
			}).Error("Packet delivery failed through interface")
		}
		return
	}

	// Fallback to old simulation behavior.
	logrus.WithFields(logrus.Fields{
		"function":    "simulatePacketDelivery",
		"friend_id":   friendID,
		"packet_size": len(packet),
	}).Info("Simulating packet delivery (fallback)")

	// For testing purposes, we'll just process the packet directly.
	// In production, this would involve actual network transmission.
	logrus.WithFields(logrus.Fields{
		"friend_id":   friendID,
		"packet_size": len(packet),
	}).Debug("Processing packet directly for simulation")

	t.processIncomingPacket(packet, nil)

	logrus.WithFields(logrus.Fields{
		"friend_id":   friendID,
		"packet_size": len(packet),
	}).Debug("Packet simulation completed")
}

// Packet Delivery Interface Management

// SetPacketDeliveryMode switches between simulation and real packet delivery modes.
func (t *Tox) SetPacketDeliveryMode(useSimulation bool) error {
	logrus.WithFields(logrus.Fields{
		"function":       "SetPacketDeliveryMode",
		"use_simulation": useSimulation,
		"current_mode":   t.IsPacketDeliverySimulation(),
	}).Info("Switching packet delivery mode")

	if err := t.validateDeliveryFactory(); err != nil {
		return err
	}

	t.switchDeliveryFactory(useSimulation)

	newDelivery := t.createPacketDelivery(useSimulation)
	t.storeDelivery(newDelivery)

	logrus.WithFields(logrus.Fields{
		"function":   "SetPacketDeliveryMode",
		"new_mode":   t.IsPacketDeliverySimulation(),
		"successful": true,
	}).Info("Packet delivery mode switched successfully")

	return nil
}

// validateDeliveryFactory checks if the delivery factory is properly initialized.
func (t *Tox) validateDeliveryFactory() error {
	if t.deliveryFactory == nil {
		return fmt.Errorf("delivery factory not initialized")
	}
	return nil
}

// switchDeliveryFactory switches the factory mode between simulation and real delivery.
func (t *Tox) switchDeliveryFactory(useSimulation bool) {
	if useSimulation {
		t.deliveryFactory.SwitchToSimulation()
	} else {
		t.deliveryFactory.SwitchToReal()
	}
}

// createPacketDelivery creates the appropriate packet delivery based on the mode.
func (t *Tox) createPacketDelivery(useSimulation bool) interfaces.IPacketDelivery {
	if t.udpTransport != nil && !useSimulation {
		return t.createRealPacketDelivery()
	}
	return t.deliveryFactory.CreateSimulationForTesting()
}

// createRealPacketDelivery attempts to create real packet delivery with fallback to simulation.
func (t *Tox) createRealPacketDelivery() interfaces.IPacketDelivery {
	underlyingUDP := t.extractUnderlyingUDPTransport()
	if underlyingUDP == nil {
		return t.deliveryFactory.CreateSimulationForTesting()
	}

	networkTransport := transport.NewNetworkTransportAdapter(underlyingUDP)
	newDelivery, err := t.deliveryFactory.CreatePacketDelivery(networkTransport)
	if err != nil {
		logrus.WithFields(logrus.Fields{
			"function": "createRealPacketDelivery",
			"error":    err.Error(),
		}).Error("Failed to create real packet delivery, falling back to simulation")
		return t.deliveryFactory.CreateSimulationForTesting()
	}

	return newDelivery
}

// extractUnderlyingUDPTransport extracts the underlying UDP transport from wrapper types.
func (t *Tox) extractUnderlyingUDPTransport() *transport.UDPTransport {
	if negotiatingTransport, ok := t.udpTransport.(*transport.NegotiatingTransport); ok {
		if udp, ok := negotiatingTransport.GetUnderlying().(*transport.UDPTransport); ok {
			return udp
		}
	} else if udp, ok := t.udpTransport.(*transport.UDPTransport); ok {
		return udp
	}
	return nil
}

// GetPacketDeliveryStats returns statistics about packet delivery.
// Deprecated: Use GetPacketDeliveryTypedStats() for type-safe access.
func (t *Tox) GetPacketDeliveryStats() map[string]interface{} {
	logrus.Warn("Tox.GetPacketDeliveryStats() is deprecated and will be removed in v2.0.0; use GetPacketDeliveryTypedStats() instead")
	stats := t.GetPacketDeliveryTypedStats()
	return map[string]interface{}{
		"is_simulation":      stats.IsSimulation,
		"friend_count":       stats.FriendCount,
		"packets_sent":       stats.PacketsSent,
		"packets_delivered":  stats.PacketsDelivered,
		"packets_failed":     stats.PacketsFailed,
		"bytes_sent":         stats.BytesSent,
		"average_latency_ms": stats.AverageLatencyMs,
		// Backward compatible keys for legacy code.
		"total_friends":         stats.FriendCount,
		"total_deliveries":      int(stats.PacketsDelivered),
		"successful_deliveries": int(stats.PacketsDelivered),
		"failed_deliveries":     int(stats.PacketsFailed),
	}
}

// GetPacketDeliveryTypedStats returns type-safe statistics about packet delivery.
func (t *Tox) GetPacketDeliveryTypedStats() interfaces.PacketDeliveryStats {
	d := t.loadDelivery()
	if d == nil {
		return interfaces.PacketDeliveryStats{
			IsSimulation: true,
		}
	}

	return d.GetTypedStats()
}

// IsPacketDeliverySimulation returns true if currently using simulation.
func (t *Tox) IsPacketDeliverySimulation() bool {
	d := t.loadDelivery()
	if d == nil {
		return true // Default to simulation if not initialized.
	}
	return d.IsSimulation()
}

// SetPacketDelivery replaces the active packet delivery implementation.
//
// This method allows external consumers to inject a custom [interfaces.IPacketDelivery]
// implementation, making the abstraction a true plug-in point. Use cases include:
//   - Custom transport backends (e.g. encrypted overlays, metrics decorators)
//   - Testing with purpose-built delivery stubs
//   - Integration harnesses that route packets through custom middleware
//
// The provided delivery must not be nil. The existing delivery is replaced
// atomically; any in-flight deliveries via the old implementation may complete
// independently.
//
// Example:
//
//	type loggingDelivery struct { inner interfaces.IPacketDelivery }
//	// … implement IPacketDelivery forwarding to inner …
//	if err := tox.SetPacketDelivery(&loggingDelivery{inner: tox.GetPacketDelivery()}); err != nil {
//	    log.Fatalf("inject delivery: %v", err)
//	}
func (t *Tox) SetPacketDelivery(delivery interfaces.IPacketDelivery) error {
	if delivery == nil {
		return fmt.Errorf("packet delivery cannot be nil")
	}
	logrus.WithFields(logrus.Fields{
		"function":   "SetPacketDelivery",
		"is_sim_old": t.IsPacketDeliverySimulation(),
		"is_sim_new": delivery.IsSimulation(),
	}).Info("Installing custom packet delivery implementation")
	t.storeDelivery(delivery)
	return nil
}

// GetPacketDelivery returns the active packet delivery implementation.
//
// The returned value is the live implementation; callers should not cache it
// across calls to [SetPacketDelivery] or [SetPacketDeliveryMode].
func (t *Tox) GetPacketDelivery() interfaces.IPacketDelivery {
	return t.loadDelivery()
}

// loadDelivery returns the current packet delivery implementation under a read lock.
func (t *Tox) loadDelivery() interfaces.IPacketDelivery {
	t.deliveryMu.RLock()
	defer t.deliveryMu.RUnlock()
	return t.packetDelivery
}

// storeDelivery replaces the packet delivery implementation under a write lock.
func (t *Tox) storeDelivery(d interfaces.IPacketDelivery) {
	t.deliveryMu.Lock()
	defer t.deliveryMu.Unlock()
	t.packetDelivery = d
}

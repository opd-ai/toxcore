package dht

import (
	"errors"
	"fmt"
	"net"
	"strconv"
	"time"

	"github.com/opd-ai/toxcore/crypto"
	"github.com/opd-ai/toxcore/transport"
	"github.com/sirupsen/logrus"
)

// Add after the existing Bootstrap method

// HandlePacket processes incoming DHT packets, particularly responses from bootstrap nodes.
// This method now includes version negotiation support for multi-network compatibility.
//
//export ToxDHTHandlePacket
func (bm *BootstrapManager) HandlePacket(packet *transport.Packet, senderAddr net.Addr) error {
	switch packet.PacketType {
	case transport.PacketVersionNegotiation:
		return bm.handleVersionNegotiationPacket(packet, senderAddr)
	case transport.PacketNoiseHandshake:
		return bm.handleVersionedHandshakePacket(packet, senderAddr)
	case transport.PacketSendNodes:
		return bm.handleSendNodesPacket(packet, senderAddr)
	case transport.PacketPingRequest:
		return bm.handlePingPacket(packet, senderAddr)
	case transport.PacketPingResponse:
		return bm.handlePingResponsePacket(packet, senderAddr)
	case transport.PacketGetNodes:
		return bm.handleGetNodesPacket(packet, senderAddr)
	default:
		return fmt.Errorf("unsupported packet type: %d", packet.PacketType)
	}
}

// handleSendNodesPacket processes a send_nodes response from a bootstrap node.
// This method now includes version-aware parsing for multi-network support.
func (bm *BootstrapManager) handleSendNodesPacket(packet *transport.Packet, senderAddr net.Addr) error {
	if err := bm.validateSendNodesPacket(packet); err != nil {
		return err
	}

	senderPK, senderID := bm.extractSenderInfo(packet)
	bm.processSender(senderID, senderAddr)
	bm.markBootstrapNodeSuccess(senderPK)

	numNodes := int(packet.Data[32])
	if numNodes < 0 {
		return fmt.Errorf("send_nodes packet contains invalid node count (received %d nodes)", numNodes)
	}

	// If numNodes is 0, that's valid - the sender has no nodes to share
	// We still processed the sender successfully above
	if numNodes > 0 {
		return bm.processReceivedNodesWithVersionDetection(packet, numNodes, senderAddr)
	}

	return nil // Successfully handled packet with 0 nodes
}

// handleVersionNegotiationPacket processes version negotiation requests from peers.
// This enables protocol capability discovery and ensures compatibility.
func (bm *BootstrapManager) handleVersionNegotiationPacket(packet *transport.Packet, senderAddr net.Addr) error {
	if !bm.enableVersioned || bm.handshakeManager == nil {
		// Version negotiation not supported, ignore packet
		logrus.WithFields(logrus.Fields{
			"function": "handleVersionNegotiationPacket",
			"address":  senderAddr.String(),
		}).Debug("Version negotiation not enabled, ignoring packet")
		return nil
	}

	logrus.WithFields(logrus.Fields{
		"function": "handleVersionNegotiationPacket",
		"address":  senderAddr.String(),
	}).Debug("Processing version negotiation request")

	// For now, we'll log that we received a version negotiation packet
	// In a complete implementation, this would parse the request and respond
	// with our supported protocol versions

	// TODO: Implement actual version negotiation protocol parsing
	// For now, indicate successful processing
	return nil
}

// handleVersionedHandshakePacket processes versioned handshake packets from peers.
// This enables secure channel establishment with protocol version agreement.
func (bm *BootstrapManager) handleVersionedHandshakePacket(packet *transport.Packet, senderAddr net.Addr) error {
	if !bm.enableVersioned || bm.handshakeManager == nil {
		// Versioned handshakes not supported, ignore packet
		logrus.WithFields(logrus.Fields{
			"function": "handleVersionedHandshakePacket",
			"address":  senderAddr.String(),
		}).Debug("Versioned handshakes not enabled, ignoring packet")
		return nil
	}

	logrus.WithFields(logrus.Fields{
		"function": "handleVersionedHandshakePacket",
		"address":  senderAddr.String(),
	}).Debug("Processing versioned handshake request")

	// Parse the versioned handshake request
	request, err := transport.ParseVersionedHandshakeRequest(packet.Data)
	if err != nil {
		logrus.WithFields(logrus.Fields{
			"function": "handleVersionedHandshakePacket",
			"address":  senderAddr.String(),
			"error":    err.Error(),
		}).Warn("Failed to parse versioned handshake request")
		return fmt.Errorf("invalid versioned handshake request: %w", err)
	}

	// Handle the handshake request
	response, err := bm.handshakeManager.HandleHandshakeRequest(request, senderAddr)
	if err != nil {
		logrus.WithFields(logrus.Fields{
			"function": "handleVersionedHandshakePacket",
			"address":  senderAddr.String(),
			"error":    err.Error(),
		}).Warn("Failed to handle versioned handshake request")
		return fmt.Errorf("handshake processing failed: %w", err)
	}

	// Send the handshake response
	responseData, err := transport.SerializeVersionedHandshakeResponse(response)
	if err != nil {
		return fmt.Errorf("failed to serialize handshake response: %w", err)
	}

	responsePacket := &transport.Packet{
		PacketType: transport.PacketNoiseHandshake,
		Data:       responseData,
	}

	err = bm.transport.Send(responsePacket, senderAddr)
	if err != nil {
		return fmt.Errorf("failed to send handshake response: %w", err)
	}

	logrus.WithFields(logrus.Fields{
		"function":       "handleVersionedHandshakePacket",
		"address":        senderAddr.String(),
		"agreed_version": response.AgreedVersion,
	}).Info("Versioned handshake completed successfully")

	// Record the negotiated protocol version for future communications
	bm.SetPeerProtocolVersion(senderAddr, response.AgreedVersion)

	return nil
}

// validateSendNodesPacket checks if the packet has valid structure and minimum size.
func (bm *BootstrapManager) validateSendNodesPacket(packet *transport.Packet) error {
	if len(packet.Data) < 33 { // Minimum size: sender's public key (32) + num_nodes (1)
		return errors.New("invalid send_nodes packet: too short")
	}
	return nil
}

// extractSenderInfo extracts the sender's public key and creates a ToxID.
func (bm *BootstrapManager) extractSenderInfo(packet *transport.Packet) ([32]byte, *crypto.ToxID) {
	var senderPK [32]byte
	copy(senderPK[:], packet.Data[:32])

	// Create nospam (zeros for DHT nodes)
	var nospam [4]byte
	senderID := crypto.NewToxID(senderPK, nospam)

	return senderPK, senderID
}

// processSender updates the routing table with the sender's information.
func (bm *BootstrapManager) processSender(senderID *crypto.ToxID, senderAddr net.Addr) {
	senderNode := NewNode(*senderID, senderAddr)
	senderNode.Update(StatusGood)
	bm.routingTable.AddNode(senderNode)
}

// markBootstrapNodeSuccess marks matching bootstrap nodes as successful.
func (bm *BootstrapManager) markBootstrapNodeSuccess(senderPK [32]byte) {
	bm.mu.Lock()
	defer bm.mu.Unlock()

	for _, node := range bm.nodes {
		if node.PublicKey == senderPK {
			node.Success = true
			node.LastUsed = time.Now()
		}
	}
}

// processReceivedNodesWithVersionDetection parses and adds received nodes using version-aware parsing.
// This method replaces processReceivedNodes() with multi-network and protocol version support.
func (bm *BootstrapManager) processReceivedNodesWithVersionDetection(packet *transport.Packet, numNodes int, senderAddr net.Addr) error {
	context := bm.initializeVersionDetectionContext(packet, numNodes, senderAddr)

	// Process each node using the version-aware parser
	for i := 0; i < numNodes; i++ {
		if !bm.processNodeWithErrorHandling(context, i) {
			break // Stop processing if we can't recover from error
		}
	}

	return nil
}

// initializeVersionDetectionContext sets up the processing context for version-aware node parsing.
// It detects the protocol version, selects the appropriate parser, and initializes processing state.
func (bm *BootstrapManager) initializeVersionDetectionContext(packet *transport.Packet, numNodes int, senderAddr net.Addr) *nodeProcessingContext {
	protocolVersion := bm.detectProtocolVersionFromPacket(packet, senderAddr)

	logrus.WithFields(logrus.Fields{
		"function":         "processReceivedNodesWithVersionDetection",
		"sender":           senderAddr.String(),
		"protocol_version": protocolVersion,
		"num_nodes":        numNodes,
	}).Debug("Processing received nodes with version detection")

	parser := bm.parser.SelectParser(protocolVersion)

	var nospam [4]byte // Create nospam (zeros for DHT nodes)

	return &nodeProcessingContext{
		packet:          packet,
		parser:          parser,
		protocolVersion: protocolVersion,
		nospam:          nospam,
		offset:          33, // Skip sender PK and numNodes byte
	}
}

// processNodeWithErrorHandling processes a single node entry with comprehensive error handling.
// Returns true if processing should continue, false if the packet should be abandoned.
func (bm *BootstrapManager) processNodeWithErrorHandling(context *nodeProcessingContext, nodeIndex int) bool {
	entry, nextOffset, err := context.parser.ParseNodeEntry(context.packet.Data, context.offset)
	if err != nil {
		return bm.handleNodeParsingError(context, nodeIndex, err)
	}

	// Convert to DHT node and add to routing table
	if err := bm.processNodeEntryVersionAware(entry, context.nospam); err != nil {
		logrus.WithFields(logrus.Fields{
			"function":   "processReceivedNodesWithVersionDetection",
			"node_index": nodeIndex,
			"error":      err.Error(),
		}).Warn("Failed to process node entry, skipping")
		// Continue with next node
	}

	context.offset = nextOffset
	return true
}

// handleNodeParsingError handles parsing errors and determines recovery strategy.
// Returns true if processing should continue, false if the packet should be abandoned.
func (bm *BootstrapManager) handleNodeParsingError(context *nodeProcessingContext, nodeIndex int, err error) bool {
	logrus.WithFields(logrus.Fields{
		"function":   "processReceivedNodesWithVersionDetection",
		"node_index": nodeIndex,
		"offset":     context.offset,
		"error":      err.Error(),
	}).Warn("Failed to parse node entry, skipping")

	// Skip this node but continue processing others
	// For legacy format, advance by fixed size; for extended, we can't easily recover
	if context.protocolVersion == transport.ProtocolLegacy {
		context.offset += 50 // Legacy format: 32+16+2 bytes
		return true
	} else {
		// For extended format, we can't reliably advance without parsing
		// Skip the rest of the packet
		return false
	}
}

// nodeProcessingContext holds the state for processing nodes with version detection.
type nodeProcessingContext struct {
	packet          *transport.Packet
	parser          transport.PacketParser // Parser interface from transport package
	protocolVersion transport.ProtocolVersion
	nospam          [4]byte
	offset          int
}

// processNodeEntryVersionAware processes a single parsed node entry with address type detection.
// This method replaces the original processNodeEntry with enhanced version awareness and multi-network support.
func (bm *BootstrapManager) processNodeEntryVersionAware(entry *transport.NodeEntry, nospam [4]byte) error {
	// Convert to net.Addr for address type detection
	addr := entry.Address.ToNetAddr()

	// Detect and validate address type
	addrType, err := bm.addressDetector.DetectAddressType(addr)
	if err != nil {
		return fmt.Errorf("address type detection failed for %s: %w", addr.String(), err)
	}

	// Validate that the address type is supported and routable
	if !bm.addressDetector.ValidateAddressType(addrType) {
		return fmt.Errorf("unsupported address type %s for address %s", addrType.String(), addr.String())
	}

	if !bm.addressDetector.IsRoutableAddress(addrType) {
		return fmt.Errorf("address type %s is not routable for address %s", addrType.String(), addr.String())
	}

	// Update address type statistics
	bm.addressStats.IncrementCount(addrType)

	// Convert the transport.NodeEntry to a DHT Node
	newNode, err := bm.convertNodeEntryToNode(entry, nospam)
	if err != nil {
		return fmt.Errorf("failed to convert node entry to DHT node: %w", err)
	}

	// Add the node to the routing table
	bm.routingTable.AddNode(newNode)

	logrus.WithFields(logrus.Fields{
		"function":              "processNodeEntryVersionAware",
		"node_id":               newNode.ID.String()[:16] + "...",
		"address":               newNode.Address.String(),
		"detected_type":         addrType.String(),
		"transport_type":        entry.Address.Type.String(),
		"network":               addr.Network(),
		"routable":              bm.addressDetector.IsRoutableAddress(addrType),
		"total_nodes_processed": bm.addressStats.TotalCount,
	}).Debug("Successfully processed node entry with address type detection")

	return nil
}

// detectProtocolVersionFromPacket determines the protocol version used by the sender.
// This enables backward compatibility while supporting new protocol features.
func (bm *BootstrapManager) detectProtocolVersionFromPacket(packet *transport.Packet, senderAddr net.Addr) transport.ProtocolVersion {
	// Check if we have any record of previous version negotiation with this peer
	// For now, use packet structure analysis

	// If the packet contains only legacy-sized node entries, assume legacy
	dataLen := len(packet.Data) - 33 // Subtract header (32-byte PK + 1-byte count)
	numNodes := int(packet.Data[32])

	if numNodes > 0 {
		legacyNodeSize := 50 // 32-byte pubkey + 16-byte IP + 2-byte port
		expectedLegacySize := numNodes * legacyNodeSize

		if dataLen == expectedLegacySize {
			logrus.WithFields(logrus.Fields{
				"function":        "detectProtocolVersionFromPacket",
				"sender":          senderAddr.String(),
				"data_length":     dataLen,
				"expected_legacy": expectedLegacySize,
			}).Debug("Detected legacy protocol format")
			return transport.ProtocolLegacy
		}
	}

	// For packets that don't match legacy format exactly, assume extended format
	// In a complete implementation, this would check for version negotiation state
	logrus.WithFields(logrus.Fields{
		"function":    "detectProtocolVersionFromPacket",
		"sender":      senderAddr.String(),
		"data_length": dataLen,
	}).Debug("Detected extended protocol format")
	return transport.ProtocolNoiseIK
}

// processReceivedNodes parses and adds received nodes to the routing table.
//
// Deprecated: Use processReceivedNodesWithVersionDetection() instead.
// This method will be removed in a future version as it does not support
// multi-network addressing or protocol version negotiation.
func (bm *BootstrapManager) processReceivedNodes(packet *transport.Packet, numNodes int) error {
	const nodeEntrySize = 32 + 16 + 2 // public key (32) + IP (16) + port (2)
	offset := 33                      // Skip sender PK and numNodes byte

	// Check packet length
	if len(packet.Data) < offset+numNodes*nodeEntrySize {
		return errors.New("invalid send_nodes packet: truncated node data")
	}

	// Create nospam (zeros for DHT nodes)
	var nospam [4]byte

	// Process each node
	for i := 0; i < numNodes; i++ {
		nodeOffset := offset + i*nodeEntrySize

		if err := bm.processNodeEntry(packet.Data, nodeOffset, nospam); err != nil {
			continue // Skip invalid nodes but continue processing others
		}
	}

	return nil
}

// processNodeEntry processes a single node entry from the packet data.
// Updated to use the new multi-network parser system instead of parseAddressFromPacket().
func (bm *BootstrapManager) processNodeEntry(data []byte, nodeOffset int, nospam [4]byte) error {
	// Parse the node entry using the new multi-network parser
	entry, _, err := bm.parseNodeEntry(data, nodeOffset)
	if err != nil {
		return fmt.Errorf("failed to parse node entry at offset %d: %w", nodeOffset, err)
	}

	// Convert the transport.NodeEntry to a DHT Node
	newNode, err := bm.convertNodeEntryToNode(entry, nospam)
	if err != nil {
		return fmt.Errorf("failed to convert node entry to DHT node: %w", err)
	}

	// Add the node to the routing table
	bm.routingTable.AddNode(newNode)

	return nil
}

// parseAddressFromPacket extracts and converts address information from packet data.
// Returns a net.Addr instead of separate IP and port to maintain abstraction.
//
// Deprecated: Use parseNodeEntry() instead. This function prevents support for
// alternative network types (.onion, .i2p, .nym, .loki) and will be removed
// in a future version. The new parseNodeEntry() method supports multi-network
// addressing through the PacketParser interface.
func (bm *BootstrapManager) parseAddressFromPacket(data []byte, nodeOffset int) net.Addr {
	// Extract IP and port
	var ip [16]byte
	copy(ip[:], data[nodeOffset+32:nodeOffset+48])

	port := uint16(data[nodeOffset+48])<<8 | uint16(data[nodeOffset+49])

	// **RED FLAG - NEEDS ARCHITECTURAL REDESIGN**
	// This address parsing logic prevents future network type support (.onion, .i2p, etc.)
	// TODO: Redesign to work without address format assumptions
	var hostStr string
	if ip[0] == 0 && ip[1] == 0 && ip[2] == 0 && ip[3] == 0 &&
		ip[4] == 0 && ip[5] == 0 && ip[6] == 0 && ip[7] == 0 &&
		ip[8] == 0 && ip[9] == 0 && ip[10] == 0xff && ip[11] == 0xff {
		// IPv4 address formatting - ARCHITECTURAL REDESIGN NEEDED
		hostStr = fmt.Sprintf("%d.%d.%d.%d", ip[12], ip[13], ip[14], ip[15])
	} else {
		// IPv6 address formatting - ARCHITECTURAL REDESIGN NEEDED
		hostStr = fmt.Sprintf("%02x%02x:%02x%02x:%02x%02x:%02x%02x:%02x%02x:%02x%02x:%02x%02x:%02x%02x",
			ip[0], ip[1], ip[2], ip[3], ip[4], ip[5], ip[6], ip[7],
			ip[8], ip[9], ip[10], ip[11], ip[12], ip[13], ip[14], ip[15])
	}

	// Create address string and resolve it to get a net.Addr interface
	addrStr := net.JoinHostPort(hostStr, strconv.Itoa(int(port)))
	addr, err := net.ResolveUDPAddr("udp", addrStr)
	if err != nil {
		// Fallback: create a minimal net.Addr implementation
		return &simpleAddr{network: "udp", address: addrStr}
	}
	return addr
}

// simpleAddr is a minimal implementation of net.Addr for fallback cases
type simpleAddr struct {
	network string
	address string
}

func (s *simpleAddr) Network() string {
	return s.network
}

func (s *simpleAddr) String() string {
	return s.address
}

// handlePingPacket processes a ping request from another node.
func (bm *BootstrapManager) handlePingPacket(packet *transport.Packet, senderAddr net.Addr) error {
	// Create ping response packet
	responsePacket := &transport.Packet{
		PacketType: transport.PacketPingResponse,
		Data:       packet.Data, // Echo back the ping data
	}

	// Send response
	return bm.transport.Send(responsePacket, senderAddr)
}

// handlePingResponsePacket processes a ping response from another node.
func (bm *BootstrapManager) handlePingResponsePacket(packet *transport.Packet, senderAddr net.Addr) error {
	if len(packet.Data) < 32 { // Minimum size: sender's public key
		return errors.New("invalid ping response packet: too short")
	}

	// Extract sender's public key
	var senderPK [32]byte
	copy(senderPK[:], packet.Data[:32])

	// Create nospam (zeros for DHT nodes)
	var nospam [4]byte
	senderID := crypto.NewToxID(senderPK, nospam)

	// Update sender in routing table as good
	senderNode := NewNode(*senderID, senderAddr)
	senderNode.Update(StatusGood)
	bm.routingTable.AddNode(senderNode)

	return nil
}

// Add this method to the BootstrapManager struct

// handleGetNodesPacket processes a get_nodes request packet and responds with
// the closest known nodes to the requested target. This is the core DHT functionality
// that enables node discovery and network topology building.
// Now includes version-aware response formatting.
func (bm *BootstrapManager) handleGetNodesPacket(packet *transport.Packet, senderAddr net.Addr) error {
	senderPK, targetPK, err := bm.validateAndExtractKeys(packet)
	if err != nil {
		return err
	}

	senderID, targetID := bm.createToxIDs(senderPK, targetPK)
	bm.updateSenderInRoutingTable(senderID, senderAddr)

	closestNodes := bm.findClosestNodes(targetID)

	// Detect the protocol version to use for the response
	responseVersion := bm.determineResponseProtocolVersion(senderAddr)

	responseData := bm.buildVersionedResponseData(closestNodes, responseVersion)

	return bm.sendNodesResponse(responseData, senderAddr)
}

// determineResponseProtocolVersion determines which protocol version to use for responses.
// This considers the sender's capabilities and our configuration.
func (bm *BootstrapManager) determineResponseProtocolVersion(senderAddr net.Addr) transport.ProtocolVersion {
	// Check if we have explicitly negotiated a version with this peer
	bm.versionMu.RLock()
	negotiatedVersion, hasNegotiatedVersion := bm.peerVersions[senderAddr.String()]
	bm.versionMu.RUnlock()

	if hasNegotiatedVersion {
		logrus.WithFields(logrus.Fields{
			"function": "determineResponseProtocolVersion",
			"sender":   senderAddr.String(),
			"version":  negotiatedVersion,
		}).Debug("Using negotiated protocol version for response")
		return negotiatedVersion
	}

	// If no negotiated version, fall back to capability detection
	if bm.enableVersioned {
		// If we support versioned handshakes, we can try the extended format
		// This assumes peers that support versioned handshakes also support extended node formats
		logrus.WithFields(logrus.Fields{
			"function": "determineResponseProtocolVersion",
			"sender":   senderAddr.String(),
		}).Debug("Using extended protocol for response (no negotiated version)")
		return transport.ProtocolNoiseIK
	}

	logrus.WithFields(logrus.Fields{
		"function": "determineResponseProtocolVersion",
		"sender":   senderAddr.String(),
	}).Debug("Using legacy protocol for response")
	return transport.ProtocolLegacy
}

// buildVersionedResponseData constructs the response packet data using version-aware formatting with address type detection.
// This replaces buildResponseData() with multi-network support and address filtering.
func (bm *BootstrapManager) buildVersionedResponseData(closestNodes []*Node, protocolVersion transport.ProtocolVersion) []byte {
	// Select appropriate parser for the protocol version
	parser := bm.parser.SelectParser(protocolVersion)

	// Filter nodes based on address type support for the target protocol
	filteredNodes := bm.filterCompatibleNodes(closestNodes, parser, protocolVersion)

	// Build response with header and serialize nodes
	responseData := bm.createResponseHeader(filteredNodes)
	responseData = bm.serializeFilteredNodes(responseData, filteredNodes, parser)

	bm.logResponseSummary(protocolVersion, len(closestNodes), len(filteredNodes), len(responseData))
	return responseData
}

// filterCompatibleNodes filters nodes based on address type support for the target protocol.
func (bm *BootstrapManager) filterCompatibleNodes(closestNodes []*Node, parser transport.PacketParser, protocolVersion transport.ProtocolVersion) []*Node {
	var filteredNodes []*Node
	supportedTypes := parser.SupportedAddressTypes()
	supportedTypeMap := bm.createSupportedTypeMap(supportedTypes)

	for _, node := range closestNodes {
		if bm.isNodeCompatible(node, supportedTypeMap, protocolVersion) {
			filteredNodes = append(filteredNodes, node)
		}
	}

	return filteredNodes
}

// createSupportedTypeMap creates a map of supported address types for efficient lookup.
func (bm *BootstrapManager) createSupportedTypeMap(supportedTypes []transport.AddressType) map[transport.AddressType]bool {
	supportedTypeMap := make(map[transport.AddressType]bool)
	for _, addrType := range supportedTypes {
		supportedTypeMap[addrType] = true
	}
	return supportedTypeMap
}

// isNodeCompatible checks if a node is compatible with the protocol and address requirements.
func (bm *BootstrapManager) isNodeCompatible(node *Node, supportedTypeMap map[transport.AddressType]bool, protocolVersion transport.ProtocolVersion) bool {
	// Detect address type for each node
	addrType, err := bm.addressDetector.DetectAddressType(node.Address)
	if err != nil {
		logrus.WithFields(logrus.Fields{
			"function": "isNodeCompatible",
			"node_id":  node.ID.String()[:16] + "...",
			"address":  node.Address.String(),
			"error":    err.Error(),
		}).Warn("Failed to detect address type, skipping node")
		return false
	}

	// Check if this address type is supported by the target protocol
	if !supportedTypeMap[addrType] {
		logrus.WithFields(logrus.Fields{
			"function":         "isNodeCompatible",
			"node_id":          node.ID.String()[:16] + "...",
			"address":          node.Address.String(),
			"address_type":     addrType.String(),
			"protocol_version": protocolVersion,
		}).Debug("Address type not supported by target protocol, skipping node")
		return false
	}

	// Validate that the address is routable
	if !bm.addressDetector.IsRoutableAddress(addrType) {
		logrus.WithFields(logrus.Fields{
			"function":     "isNodeCompatible",
			"node_id":      node.ID.String()[:16] + "...",
			"address":      node.Address.String(),
			"address_type": addrType.String(),
		}).Debug("Address type not routable, skipping node")
		return false
	}

	return true
}

// createResponseHeader creates the initial response data with sender public key and node count.
func (bm *BootstrapManager) createResponseHeader(filteredNodes []*Node) []byte {
	responseData := make([]byte, 33) // Start with header
	copy(responseData[:32], bm.selfID.PublicKey[:])
	responseData[32] = byte(len(filteredNodes))
	return responseData
}

// serializeFilteredNodes serializes each filtered node and appends to response data.
func (bm *BootstrapManager) serializeFilteredNodes(responseData []byte, filteredNodes []*Node, parser transport.PacketParser) []byte {
	for _, node := range filteredNodes {
		// Convert DHT Node to transport.NodeEntry
		entry, err := bm.convertNodeToNodeEntry(node)
		if err != nil {
			logrus.WithError(err).WithField("node", node.ID.String()).
				Warn("Failed to convert node to entry, skipping")
			continue
		}

		// Serialize the node entry
		serialized, err := parser.SerializeNodeEntry(entry)
		if err != nil {
			logrus.WithError(err).WithField("node", node.ID.String()).
				Warn("Failed to serialize node entry, skipping")
			continue
		}

		// Append to response data
		responseData = append(responseData, serialized...)
	}

	return responseData
}

// logResponseSummary logs a summary of the response building process.
func (bm *BootstrapManager) logResponseSummary(protocolVersion transport.ProtocolVersion, originalNodes, filteredNodes, responseSize int) {
	logrus.WithFields(logrus.Fields{
		"function":         "buildVersionedResponseData",
		"protocol_version": protocolVersion,
		"original_nodes":   originalNodes,
		"filtered_nodes":   filteredNodes,
		"response_size":    responseSize,
	}).Debug("Built versioned response data with address type filtering")
}

// validateAndExtractKeys validates the packet format and extracts sender and target public keys.
func (bm *BootstrapManager) validateAndExtractKeys(packet *transport.Packet) ([32]byte, [32]byte, error) {
	// Packet format: [sender_pk(32 bytes)][target_pk(32 bytes)]
	if len(packet.Data) < 64 {
		return [32]byte{}, [32]byte{}, errors.New("invalid get_nodes packet: too short")
	}

	var senderPK, targetPK [32]byte
	copy(senderPK[:], packet.Data[:32])
	copy(targetPK[:], packet.Data[32:64])

	return senderPK, targetPK, nil
}

// createToxIDs creates Tox IDs for sender and target from their public keys.
func (bm *BootstrapManager) createToxIDs(senderPK, targetPK [32]byte) (*crypto.ToxID, *crypto.ToxID) {
	var nospam [4]byte
	senderID := crypto.NewToxID(senderPK, nospam)
	targetID := crypto.NewToxID(targetPK, nospam)
	return senderID, targetID
}

// updateSenderInRoutingTable adds or updates the sender node in the routing table.
func (bm *BootstrapManager) updateSenderInRoutingTable(senderID *crypto.ToxID, senderAddr net.Addr) {
	senderNode := NewNode(*senderID, senderAddr)
	senderNode.Update(StatusGood)
	bm.routingTable.AddNode(senderNode)
}

// findClosestNodes retrieves the closest nodes to the target from the routing table.
func (bm *BootstrapManager) findClosestNodes(targetID *crypto.ToxID) []*Node {
	const maxNodesToSend = 4 // Typical DHT value
	return bm.routingTable.FindClosestNodes(*targetID, maxNodesToSend)
}

// buildResponseData constructs the response packet data with our public key and node entries.
//
// Deprecated: Use buildVersionedResponseData() instead. This method will be removed
// in a future version as it does not support multi-network addressing or protocol
// version negotiation.
func (bm *BootstrapManager) buildResponseData(closestNodes []*Node) []byte {
	// Format: [sender_pk(32 bytes)][num_nodes(1 byte)][node_entries(50 bytes each)]
	responseSize := 32 + 1 + (len(closestNodes) * (32 + 16 + 2))
	responseData := make([]byte, responseSize)

	// Add our public key
	copy(responseData[:32], bm.selfID.PublicKey[:])

	// Add number of nodes
	responseData[32] = byte(len(closestNodes))

	// Add node entries
	offset := 33
	for _, node := range closestNodes {
		offset = bm.encodeNodeEntry(responseData, offset, node)
	}

	return responseData
}

// encodeNodeEntry encodes a single node entry into the response data at the given offset.
// Updated to use the new multi-network serialization system instead of formatIPAddress().
func (bm *BootstrapManager) encodeNodeEntry(responseData []byte, offset int, node *Node) int {
	// Convert DHT Node to transport.NodeEntry
	entry, err := bm.convertNodeToNodeEntry(node)
	if err != nil {
		// Log error and skip this node entry
		logrus.WithError(err).WithField("node", node.ID.String()).
			Warn("Failed to convert node to entry, skipping")
		return offset
	}

	// Serialize the node entry using the new multi-network system
	serialized, err := bm.serializeNodeEntry(entry)
	if err != nil {
		// Log error and skip this node entry
		logrus.WithError(err).WithField("node", node.ID.String()).
			Warn("Failed to serialize node entry, skipping")
		return offset
	}

	// Copy the serialized data to the response buffer
	if len(responseData) >= offset+len(serialized) {
		copy(responseData[offset:offset+len(serialized)], serialized)
		return offset + len(serialized)
	} else {
		// Not enough space in response buffer
		logrus.WithField("node", node.ID.String()).WithField("needed", len(serialized)).
			WithField("available", len(responseData)-offset).
			Warn("Insufficient space in response buffer for node entry")
		return offset
	}
}

// formatIPAddress converts a network address to a byte representation
//
// Deprecated: Use serializeNodeEntry() instead. This function prevents support for
// alternative network types (.onion, .i2p, .nym, .loki) and will be removed
// in a future version. The new serializeNodeEntry() method supports multi-network
// addressing through the PacketParser interface.
//
// **RED FLAG - NEEDS ARCHITECTURAL REDESIGN**
// This function attempts to parse IP addresses from net.Addr which prevents
// compatibility with alternative network types (.onion, .b32.i2p, .nym, .loki).
// Consider redesigning the protocol to work with opaque address identifiers
// or passing address type information through a separate channel.
func (bm *BootstrapManager) formatIPAddress(addr net.Addr) []byte {
	ip := make([]byte, 16)

	// **REDESIGN NEEDED**: This address parsing prevents future network type support
	// For now, we'll try basic string parsing as a temporary measure
	addrStr := addr.String()
	host, _, err := net.SplitHostPort(addrStr)
	if err != nil {
		host = addrStr
	}

	if parsedIP := net.ParseIP(host); parsedIP != nil {
		if ipv4 := parsedIP.To4(); ipv4 != nil {
			// IPv4-mapped IPv6 address format
			ip[10] = 0xff
			ip[11] = 0xff
			copy(ip[12:16], ipv4)
		} else {
			// IPv6 address
			copy(ip, parsedIP.To16())
		}
	}
	// For non-IP addresses, returns zero bytes - caller must handle this case
	return ip
}

// sendNodesResponse creates and sends the send_nodes response packet.
func (bm *BootstrapManager) sendNodesResponse(responseData []byte, senderAddr net.Addr) error {
	responsePacket := &transport.Packet{
		PacketType: transport.PacketSendNodes,
		Data:       responseData,
	}
	return bm.transport.Send(responsePacket, senderAddr)
}

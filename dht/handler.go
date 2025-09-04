package dht

import (
	"errors"
	"fmt"
	"net"
	"time"

	"github.com/opd-ai/toxcore/crypto"
	"github.com/opd-ai/toxcore/transport"
)

// Add after the existing Bootstrap method

// HandlePacket processes incoming DHT packets, particularly responses from bootstrap nodes.
//
//export ToxDHTHandlePacket
func (bm *BootstrapManager) HandlePacket(packet *transport.Packet, senderAddr net.Addr) error {
	switch packet.PacketType {
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
func (bm *BootstrapManager) handleSendNodesPacket(packet *transport.Packet, senderAddr net.Addr) error {
	if err := bm.validateSendNodesPacket(packet); err != nil {
		return err
	}

	senderPK, senderID := bm.extractSenderInfo(packet)
	bm.processSender(senderID, senderAddr)
	bm.markBootstrapNodeSuccess(senderPK)

	numNodes := int(packet.Data[32])
	if numNodes <= 0 {
		return fmt.Errorf("send_nodes packet contains no nodes (received %d nodes)", numNodes)
	}

	return bm.processReceivedNodes(packet, numNodes)
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

// processReceivedNodes parses and adds received nodes to the routing table.
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
func (bm *BootstrapManager) processNodeEntry(data []byte, nodeOffset int, nospam [4]byte) error {
	// Extract node public key
	var nodePK [32]byte
	copy(nodePK[:], data[nodeOffset:nodeOffset+32])

	// Extract and parse IP and port
	ipAddr, port := bm.parseIPAndPort(data, nodeOffset)

	// Create network address - for DHT wire protocol, we know these are IP:Port
	addr := bm.createNetworkAddress(ipAddr, port)

	// Create node ID and add to routing table
	nodeID := crypto.NewToxID(nodePK, nospam)
	newNode := NewNode(*nodeID, addr)
	bm.routingTable.AddNode(newNode)

	return nil
}

// parseIPAndPort extracts and converts IP address and port from packet data.
func (bm *BootstrapManager) parseIPAndPort(data []byte, nodeOffset int) (net.IP, uint16) {
	// Extract IP and port
	var ip [16]byte
	copy(ip[:], data[nodeOffset+32:nodeOffset+48])

	port := uint16(data[nodeOffset+48])<<8 | uint16(data[nodeOffset+49])

	// Create IP address
	var ipAddr net.IP
	if ip[0] == 0 && ip[1] == 0 && ip[2] == 0 && ip[3] == 0 &&
		ip[4] == 0 && ip[5] == 0 && ip[6] == 0 && ip[7] == 0 &&
		ip[8] == 0 && ip[9] == 0 && ip[10] == 0xff && ip[11] == 0xff {
		// IPv4 address
		ipAddr = net.IP(ip[12:16])
	} else {
		// IPv6 address
		ipAddr = net.IP(ip[:])
	}

	return ipAddr, port
}

// createNetworkAddress creates a net.Addr from IP and port
// For DHT wire protocol compatibility, this creates UDP addresses
func (bm *BootstrapManager) createNetworkAddress(ip net.IP, port uint16) net.Addr {
	return &net.UDPAddr{
		IP:   ip,
		Port: int(port),
	}
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
func (bm *BootstrapManager) handleGetNodesPacket(packet *transport.Packet, senderAddr net.Addr) error {
	senderPK, targetPK, err := bm.validateAndExtractKeys(packet)
	if err != nil {
		return err
	}

	senderID, targetID := bm.createToxIDs(senderPK, targetPK)
	bm.updateSenderInRoutingTable(senderID, senderAddr)

	closestNodes := bm.findClosestNodes(targetID)
	responseData := bm.buildResponseData(closestNodes)

	return bm.sendNodesResponse(responseData, senderAddr)
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
func (bm *BootstrapManager) encodeNodeEntry(responseData []byte, offset int, node *Node) int {
	// Add node public key
	copy(responseData[offset:offset+32], node.PublicKey[:])
	offset += 32

	// Add node IP (padded to 16 bytes)
	ip := bm.formatIPAddress(node.Address)
	copy(responseData[offset:offset+16], ip)
	offset += 16

	// Add node port
	_, port := node.IPPort()
	responseData[offset] = byte(port >> 8)     // Port high byte
	responseData[offset+1] = byte(port & 0xff) // Port low byte

	return offset + 2
}

// formatIPAddress converts a network address to a byte representation
func (bm *BootstrapManager) formatIPAddress(addr net.Addr) []byte {
	ip := make([]byte, 16)

	// Extract IP address from interface using address parsing
	ipAddr := bm.extractIPFromAddr(addr)
	if ipAddr != nil {
		if ipv4 := ipAddr.To4(); ipv4 != nil {
			// IPv4-mapped IPv6 address format
			ip[10] = 0xff
			ip[11] = 0xff
			copy(ip[12:16], ipv4)
		} else {
			// IPv6 address
			copy(ip, ipAddr.To16())
		}
	}
	return ip
}

// extractIPFromAddr extracts IP address from a net.Addr interface
// by parsing the string representation. Returns nil for non-IP addresses.
func (bm *BootstrapManager) extractIPFromAddr(addr net.Addr) net.IP {
	host, _, err := net.SplitHostPort(addr.String())
	if err != nil {
		// If split fails, try parsing the entire string as an IP
		if ip := net.ParseIP(addr.String()); ip != nil {
			return ip
		}
		// For non-IP addresses (.onion, .b32.i2p, etc.), return nil
		// The caller should handle this case appropriately
		return nil
	}
	return net.ParseIP(host)
}

// sendNodesResponse creates and sends the send_nodes response packet.
func (bm *BootstrapManager) sendNodesResponse(responseData []byte, senderAddr net.Addr) error {
	responsePacket := &transport.Packet{
		PacketType: transport.PacketSendNodes,
		Data:       responseData,
	}
	return bm.transport.Send(responsePacket, senderAddr)
}

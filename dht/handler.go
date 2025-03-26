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
	if len(packet.Data) < 33 { // Minimum size: sender's public key (32) + num_nodes (1)
		return errors.New("invalid send_nodes packet: too short")
	}

	// Extract sender's public key
	var senderPK [32]byte
	copy(senderPK[:], packet.Data[:32])

	// Create nospam (zeros for DHT nodes)
	var nospam [4]byte
	senderID := crypto.NewToxID(senderPK, nospam)

	// Update sender in routing table
	senderNode := NewNode(*senderID, senderAddr)
	senderNode.Update(StatusGood)
	bm.routingTable.AddNode(senderNode)

	// Mark bootstrap node as successful if it matches
	bm.mu.Lock()
	for _, node := range bm.nodes {
		if node.PublicKey == senderPK {
			node.Success = true
			node.LastUsed = time.Now()
		}
	}
	bm.mu.Unlock()

	// Parse received nodes
	numNodes := int(packet.Data[32])
	if numNodes <= 0 {
		return nil // No nodes included
	}

	// Each node entry consists of: public key (32) + IP (16) + port (2)
	const nodeEntrySize = 32 + 16 + 2
	offset := 33 // Skip sender PK and numNodes byte

	// Check packet length
	if len(packet.Data) < offset+numNodes*nodeEntrySize {
		return errors.New("invalid send_nodes packet: truncated node data")
	}

	// Process each node
	for i := 0; i < numNodes; i++ {
		nodeOffset := offset + i*nodeEntrySize

		// Extract node public key
		var nodePK [32]byte
		copy(nodePK[:], packet.Data[nodeOffset:nodeOffset+32])

		// Extract IP and port
		var ip [16]byte
		copy(ip[:], packet.Data[nodeOffset+32:nodeOffset+48])

		port := uint16(packet.Data[nodeOffset+48])<<8 | uint16(packet.Data[nodeOffset+49])

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

		// Create UDP address
		nodeAddr := &net.UDPAddr{
			IP:   ipAddr,
			Port: int(port),
		}

		// Create node ID
		nodeID := crypto.NewToxID(nodePK, nospam)

		// Create and add node to routing table
		newNode := NewNode(*nodeID, nodeAddr)
		bm.routingTable.AddNode(newNode)
	}

	return nil
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

// handleGetNodesPacket processes a get_nodes request from another node.
// When a node asks us for nodes close to a target, we look up in our routing table
// and respond with the closest nodes we know about.
func (bm *BootstrapManager) handleGetNodesPacket(packet *transport.Packet, senderAddr net.Addr) error {
    // Packet format: [sender_pk(32 bytes)][target_pk(32 bytes)]
    if len(packet.Data) < 64 {
        return errors.New("invalid get_nodes packet: too short")
    }
    
    // Extract sender's public key
    var senderPK [32]byte
    copy(senderPK[:], packet.Data[:32])
    
    // Extract target public key (node they're searching for)
    var targetPK [32]byte
    copy(targetPK[:], packet.Data[32:64])
    
    // Create sender's Tox ID
    var nospam [4]byte
    senderID := crypto.NewToxID(senderPK, nospam)
    
    // Create target Tox ID for search
    targetID := crypto.NewToxID(targetPK, nospam)
    
    // Update sender in routing table
    senderNode := NewNode(*senderID, senderAddr)
    senderNode.Update(StatusGood)
    bm.routingTable.AddNode(senderNode)
    
    // Find closest nodes to target
    const maxNodesToSend = 4 // Typical DHT value
    closestNodes := bm.routingTable.FindClosestNodes(*targetID, maxNodesToSend)
    
    // Prepare response packet
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
        // Add node public key
        copy(responseData[offset:offset+32], node.PublicKey[:])
        offset += 32
        
        // Add node IP (padded to 16 bytes)
        ip := make([]byte, 16)
        switch addr := node.Address.(type) {
        case *net.UDPAddr:
            if ipv4 := addr.IP.To4(); ipv4 != nil {
                // IPv4-mapped IPv6 address format
                ip[10] = 0xff
                ip[11] = 0xff
                copy(ip[12:16], ipv4)
            } else {
                // IPv6 address
                copy(ip, addr.IP.To16())
            }
        }
        copy(responseData[offset:offset+16], ip)
        offset += 16
        
        // Add node port
        _, port := node.IPPort()
        responseData[offset] = byte(port >> 8)     // Port high byte
        responseData[offset+1] = byte(port & 0xff) // Port low byte
        offset += 2
    }
    
    // Create and send send_nodes response packet
    responsePacket := &transport.Packet{
        PacketType: transport.PacketSendNodes,
        Data:       responseData,
    }
    
    return bm.transport.Send(responsePacket, senderAddr)
}

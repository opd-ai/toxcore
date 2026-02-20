// Package dht implements parser integration for multi-network support.
//
// This file implements the replacement methods for the RED FLAG functions
// parseAddressFromPacket() and formatIPAddress() as part of Phase 2.1
// of the architectural redesign plan.
package dht

import (
	"errors"
	"fmt"

	"github.com/opd-ai/toxcore/crypto"
	"github.com/opd-ai/toxcore/transport"
)

// parseNodeEntry replaces parseAddressFromPacket() with multi-network support.
// This method uses the PacketParser interface to support both legacy IP formats
// and extended formats for .onion, .i2p, .nym, and .loki addresses.
func (bm *BootstrapManager) parseNodeEntry(data []byte, offset int) (*transport.NodeEntry, int, error) {
	// Auto-detect the protocol version based on packet structure
	parser := bm.detectParserForPacket(data, offset)

	// Parse the node entry using the appropriate parser
	entry, nextOffset, err := parser.ParseNodeEntry(data, offset)
	if err != nil {
		return nil, offset, fmt.Errorf("failed to parse node entry: %w", err)
	}

	return entry, nextOffset, nil
}

// serializeNodeEntry replaces formatIPAddress() with multi-network serialization.
// This method uses the PacketParser interface to serialize node entries
// for both legacy and extended address formats.
func (bm *BootstrapManager) serializeNodeEntry(entry *transport.NodeEntry) ([]byte, error) {
	if entry == nil {
		return nil, errors.New("node entry cannot be nil")
	}

	if entry.Address == nil {
		return nil, errors.New("node entry address cannot be nil")
	}

	// Select the appropriate parser based on address type
	parser := bm.parser.SelectParserForAddressType(entry.Address.Type)

	// Serialize the node entry using the selected parser
	data, err := parser.SerializeNodeEntry(entry)
	if err != nil {
		return nil, fmt.Errorf("failed to serialize node entry: %w", err)
	}

	return data, nil
}

// detectParserForPacket automatically detects which parser to use based on packet structure.
// This provides backward compatibility by analyzing the packet format.
// hasMinimumPacketSize checks if data has enough bytes for basic analysis.
func hasMinimumPacketSize(data []byte, offset int) bool {
	return len(data) >= offset+34
}

// isLegacyIPv4Mapped checks if the data contains an IPv4-mapped IPv6 address indicating legacy format.
func isLegacyIPv4Mapped(data []byte, ipStart int) bool {
	return len(data) >= ipStart+16 && data[ipStart+10] == 0xff && data[ipStart+11] == 0xff
}

// detectLegacyFormat checks if the packet matches the legacy 50-byte structure.
func (bm *BootstrapManager) detectLegacyFormat(data []byte, offset int) (transport.PacketParser, bool) {
	if len(data) >= offset+50 {
		ipStart := offset + 32
		if isLegacyIPv4Mapped(data, ipStart) {
			return bm.parser.SelectParser(transport.ProtocolLegacy), true
		}
	}
	return nil, false
}

// detectExtendedFormat checks if the packet matches the extended format with type field.
func (bm *BootstrapManager) detectExtendedFormat(data []byte, offset int) (transport.PacketParser, bool) {
	if len(data) >= offset+35 {
		addressType := transport.AddressType(data[offset+32])
		switch addressType {
		case transport.AddressTypeOnion, transport.AddressTypeI2P,
			transport.AddressTypeNym, transport.AddressTypeLoki,
			transport.AddressTypeIPv4, transport.AddressTypeIPv6:
			return bm.parser.SelectParser(transport.ProtocolNoiseIK), true
		}
	}
	return nil, false
}

func (bm *BootstrapManager) detectParserForPacket(data []byte, offset int) transport.PacketParser {
	if !hasMinimumPacketSize(data, offset) {
		return bm.parser.SelectParser(transport.ProtocolLegacy)
	}

	if parser, found := bm.detectLegacyFormat(data, offset); found {
		return parser
	}

	if parser, found := bm.detectExtendedFormat(data, offset); found {
		return parser
	}

	return bm.parser.SelectParser(transport.ProtocolLegacy)
}

// convertNodeEntryToNode converts a transport.NodeEntry to a DHT Node.
// This bridges the new multi-network system with the existing DHT code.
func (bm *BootstrapManager) convertNodeEntryToNode(entry *transport.NodeEntry, nospam [4]byte) (*Node, error) {
	if entry == nil {
		return nil, errors.New("node entry cannot be nil")
	}

	if entry.Address == nil {
		return nil, errors.New("node entry address cannot be nil")
	}

	// Convert NetworkAddress to net.Addr for compatibility with existing Node structure
	addr := entry.Address.ToNetAddr()

	// Create ToxID from public key
	nodeID := crypto.NewToxID(entry.PublicKey, nospam)

	// Create and return the DHT node
	node := NewNode(*nodeID, addr)
	node.LastSeen = entry.LastSeen

	return node, nil
}

// convertNodeToNodeEntry converts a DHT Node to a transport.NodeEntry.
// This allows existing DHT nodes to be serialized using the new format.
func (bm *BootstrapManager) convertNodeToNodeEntry(node *Node) (*transport.NodeEntry, error) {
	if node == nil {
		return nil, errors.New("node cannot be nil")
	}

	// Convert net.Addr to NetworkAddress
	netAddr, err := transport.ConvertNetAddrToNetworkAddress(node.Address)
	if err != nil {
		return nil, fmt.Errorf("failed to convert node address: %w", err)
	}

	entry := &transport.NodeEntry{
		PublicKey: node.PublicKey,
		Address:   netAddr,
		LastSeen:  node.LastSeen,
	}

	return entry, nil
}

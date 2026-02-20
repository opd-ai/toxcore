// Package transport implements network transport for the Tox protocol.
//
// This file implements a STUN (Session Traversal Utilities for NAT) client
// for accurate public IP address detection through external STUN servers.
package transport

import (
	"context"
	"crypto/rand"
	"encoding/binary"
	"errors"
	"fmt"
	"net"
	"time"
)

// STUN protocol constants as defined in RFC 5389
const (
	stunMagicCookie = 0x2112A442
	stunHeaderSize  = 20

	// STUN message types
	stunBindingRequest  = 0x0001
	stunBindingResponse = 0x0101
	stunBindingError    = 0x0111

	// STUN attribute types
	stunAttrMappedAddress    = 0x0001
	stunAttrXorMappedAddress = 0x0020
	stunAttrErrorCode        = 0x0009
)

// STUNClient provides STUN-based public IP discovery functionality
type STUNClient struct {
	servers []string
	timeout time.Duration
}

// NewSTUNClient creates a new STUN client with default public STUN servers
func NewSTUNClient() *STUNClient {
	return &STUNClient{
		servers: []string{
			"stun.l.google.com:19302",
			"stun1.l.google.com:19302",
			"stun.stunprotocol.org:3478",
			"stun.cloudflare.com:3478",
		},
		timeout: 5 * time.Second,
	}
}

// DiscoverPublicAddress discovers the public IP address using STUN protocol
func (sc *STUNClient) DiscoverPublicAddress(ctx context.Context, localAddr net.Addr) (net.Addr, error) {
	if localAddr == nil {
		return nil, errors.New("local address cannot be nil")
	}

	// Try each STUN server until one succeeds
	var lastErr error
	for _, server := range sc.servers {
		addr, err := sc.querySTUNServer(ctx, server, localAddr)
		if err == nil {
			return addr, nil
		}
		lastErr = err

		// Check if context was cancelled
		select {
		case <-ctx.Done():
			return nil, ctx.Err()
		default:
		}
	}

	return nil, fmt.Errorf("all STUN servers failed, last error: %w", lastErr)
}

// querySTUNServer queries a specific STUN server for public address mapping
func (sc *STUNClient) querySTUNServer(ctx context.Context, server string, localAddr net.Addr) (net.Addr, error) {
	if err := checkContextCancellation(ctx); err != nil {
		return nil, err
	}

	conn, err := sc.dialSTUNServer(ctx, server)
	if err != nil {
		return nil, err
	}
	defer conn.Close()

	sc.setConnectionDeadline(ctx, conn)

	transactionID, err := generateTransactionID()
	if err != nil {
		return nil, err
	}

	if err := sc.sendBindingRequest(conn, transactionID); err != nil {
		return nil, err
	}

	return sc.receiveBindingResponse(conn, transactionID)
}

// checkContextCancellation verifies the context is not cancelled before proceeding.
func checkContextCancellation(ctx context.Context) error {
	select {
	case <-ctx.Done():
		return ctx.Err()
	default:
		return nil
	}
}

// dialSTUNServer creates a UDP connection to the STUN server.
func (sc *STUNClient) dialSTUNServer(ctx context.Context, server string) (net.Conn, error) {
	dialer := &net.Dialer{Timeout: sc.timeout}
	conn, err := dialer.DialContext(ctx, "udp", server)
	if err != nil {
		return nil, fmt.Errorf("failed to connect to STUN server %s: %w", server, err)
	}
	return conn, nil
}

// setConnectionDeadline sets read/write deadlines on the connection based on context.
func (sc *STUNClient) setConnectionDeadline(ctx context.Context, conn net.Conn) {
	deadline, ok := ctx.Deadline()
	if ok {
		conn.SetDeadline(deadline)
	} else {
		conn.SetDeadline(time.Now().Add(sc.timeout))
	}
}

// generateTransactionID creates a random 96-bit transaction ID for STUN.
func generateTransactionID() ([]byte, error) {
	transactionID := make([]byte, 12)
	if _, err := rand.Read(transactionID); err != nil {
		return nil, fmt.Errorf("failed to generate transaction ID: %w", err)
	}
	return transactionID, nil
}

// sendBindingRequest sends a STUN binding request to the server.
func (sc *STUNClient) sendBindingRequest(conn net.Conn, transactionID []byte) error {
	request := sc.buildBindingRequest(transactionID)
	if _, err := conn.Write(request); err != nil {
		return fmt.Errorf("failed to send STUN request: %w", err)
	}
	return nil
}

// receiveBindingResponse reads and parses the STUN binding response.
func (sc *STUNClient) receiveBindingResponse(conn net.Conn, transactionID []byte) (net.Addr, error) {
	response := make([]byte, 1024)
	n, err := conn.Read(response)
	if err != nil {
		return nil, fmt.Errorf("failed to read STUN response: %w", err)
	}

	return sc.parseBindingResponse(response[:n], transactionID)
}

// buildBindingRequest constructs a STUN binding request packet
func (sc *STUNClient) buildBindingRequest(transactionID []byte) []byte {
	packet := make([]byte, stunHeaderSize)

	// Message type (binding request)
	binary.BigEndian.PutUint16(packet[0:2], stunBindingRequest)

	// Message length (0 for basic binding request)
	binary.BigEndian.PutUint16(packet[2:4], 0)

	// Magic cookie
	binary.BigEndian.PutUint32(packet[4:8], stunMagicCookie)

	// Transaction ID (12 bytes)
	copy(packet[8:20], transactionID)

	return packet
}

// parseBindingResponse parses a STUN binding response and extracts the mapped address
func (sc *STUNClient) parseBindingResponse(response, expectedTransactionID []byte) (net.Addr, error) {
	if err := sc.validateResponseLength(response); err != nil {
		return nil, err
	}

	if err := sc.validateMessageType(response); err != nil {
		return nil, err
	}

	if err := sc.validateMagicCookie(response); err != nil {
		return nil, err
	}

	if err := sc.validateTransactionID(response, expectedTransactionID); err != nil {
		return nil, err
	}

	return sc.extractAndParseAttributes(response, expectedTransactionID)
}

// validateResponseLength checks if the response has minimum required length.
func (sc *STUNClient) validateResponseLength(response []byte) error {
	if len(response) < stunHeaderSize {
		return errors.New("STUN response too short")
	}
	return nil
}

// validateMessageType verifies the STUN message type is a binding response.
func (sc *STUNClient) validateMessageType(response []byte) error {
	messageType := binary.BigEndian.Uint16(response[0:2])
	if messageType == stunBindingError {
		return errors.New("STUN server returned error response")
	}
	if messageType != stunBindingResponse {
		return fmt.Errorf("unexpected STUN message type: 0x%04x", messageType)
	}
	return nil
}

// validateMagicCookie checks if the magic cookie matches expected value.
func (sc *STUNClient) validateMagicCookie(response []byte) error {
	magicCookie := binary.BigEndian.Uint32(response[4:8])
	if magicCookie != stunMagicCookie {
		return errors.New("invalid STUN magic cookie")
	}
	return nil
}

// validateTransactionID verifies the transaction ID matches expected value.
func (sc *STUNClient) validateTransactionID(response, expectedTransactionID []byte) error {
	responseTransactionID := response[8:20]
	for i := 0; i < 12; i++ {
		if responseTransactionID[i] != expectedTransactionID[i] {
			return errors.New("STUN transaction ID mismatch")
		}
	}
	return nil
}

// extractAndParseAttributes extracts attributes section and parses it.
func (sc *STUNClient) extractAndParseAttributes(response, expectedTransactionID []byte) (net.Addr, error) {
	messageLength := binary.BigEndian.Uint16(response[2:4])
	attributesStart := stunHeaderSize
	attributesEnd := attributesStart + int(messageLength)

	if len(response) < attributesEnd {
		return nil, errors.New("STUN response truncated")
	}

	return sc.parseAttributes(response[attributesStart:attributesEnd], expectedTransactionID)
}

// parseAttributes parses STUN attributes to extract the mapped address
func (sc *STUNClient) parseAttributes(attributes, transactionID []byte) (net.Addr, error) {
	offset := 0

	for offset < len(attributes) {
		if offset+4 > len(attributes) {
			break
		}

		attrType := binary.BigEndian.Uint16(attributes[offset : offset+2])
		attrLength := binary.BigEndian.Uint16(attributes[offset+2 : offset+4])
		offset += 4

		if offset+int(attrLength) > len(attributes) {
			break
		}

		attrValue := attributes[offset : offset+int(attrLength)]

		switch attrType {
		case stunAttrXorMappedAddress:
			// Prefer XOR-mapped address (RFC 5389)
			return sc.parseXorMappedAddress(attrValue, transactionID)
		case stunAttrMappedAddress:
			// Fallback to regular mapped address
			return sc.parseMappedAddress(attrValue)
		}

		// Move to next attribute (with padding to 4-byte boundary)
		offset += int(attrLength)
		if offset%4 != 0 {
			offset += 4 - (offset % 4)
		}
	}

	return nil, errors.New("no mapped address found in STUN response")
}

// parseXorMappedAddress parses XOR-MAPPED-ADDRESS attribute
func (sc *STUNClient) parseXorMappedAddress(attrValue, transactionID []byte) (net.Addr, error) {
	if len(attrValue) < 8 {
		return nil, errors.New("XOR-mapped address too short")
	}

	family := binary.BigEndian.Uint16(attrValue[0:2])
	xorPort := binary.BigEndian.Uint16(attrValue[2:4])

	// XOR the port with the magic cookie
	port := xorPort ^ uint16(stunMagicCookie>>16)

	switch family {
	case 0x01: // IPv4
		if len(attrValue) < 8 {
			return nil, errors.New("IPv4 XOR-mapped address too short")
		}
		xorAddress := binary.BigEndian.Uint32(attrValue[4:8])
		// XOR the address with the magic cookie
		address := xorAddress ^ stunMagicCookie
		ip := net.IPv4(byte(address>>24), byte(address>>16), byte(address>>8), byte(address))
		return &net.UDPAddr{IP: ip, Port: int(port)}, nil

	case 0x02: // IPv6
		if len(attrValue) < 20 {
			return nil, errors.New("IPv6 XOR-mapped address too short")
		}
		ip := make(net.IP, 16)
		// XOR with magic cookie + transaction ID
		xorKey := make([]byte, 16)
		binary.BigEndian.PutUint32(xorKey[0:4], stunMagicCookie)
		copy(xorKey[4:16], transactionID)

		for i := 0; i < 16; i++ {
			ip[i] = attrValue[4+i] ^ xorKey[i]
		}
		return &net.UDPAddr{IP: ip, Port: int(port)}, nil
	}

	return nil, fmt.Errorf("unsupported address family: %d", family)
}

// parseMappedAddress parses MAPPED-ADDRESS attribute (legacy)
func (sc *STUNClient) parseMappedAddress(attrValue []byte) (net.Addr, error) {
	if len(attrValue) < 8 {
		return nil, errors.New("mapped address too short")
	}

	family := binary.BigEndian.Uint16(attrValue[0:2])
	port := binary.BigEndian.Uint16(attrValue[2:4])

	switch family {
	case 0x01: // IPv4
		if len(attrValue) < 8 {
			return nil, errors.New("IPv4 mapped address too short")
		}
		ip := net.IP(attrValue[4:8])
		return &net.UDPAddr{IP: ip, Port: int(port)}, nil

	case 0x02: // IPv6
		if len(attrValue) < 20 {
			return nil, errors.New("IPv6 mapped address too short")
		}
		ip := net.IP(attrValue[4:20])
		return &net.UDPAddr{IP: ip, Port: int(port)}, nil
	}

	return nil, fmt.Errorf("unsupported address family: %d", family)
}

// SetServers allows customizing the STUN servers list
func (sc *STUNClient) SetServers(servers []string) {
	sc.servers = make([]string, len(servers))
	copy(sc.servers, servers)
}

// SetTimeout sets the timeout for STUN operations
func (sc *STUNClient) SetTimeout(timeout time.Duration) {
	sc.timeout = timeout
}

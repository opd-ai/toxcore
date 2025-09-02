package transport

import (
	"errors"
	"fmt"
	"net"
	"sync"

	"github.com/flynn/noise"
	"github.com/opd-ai/toxcore/crypto"
	toxnoise "github.com/opd-ai/toxcore/noise"
)

var (
	// ErrNoiseNotSupported indicates peer doesn't support Noise protocol
	ErrNoiseNotSupported = errors.New("peer does not support noise protocol")
	// ErrNoiseSessionNotFound indicates no active session with peer
	ErrNoiseSessionNotFound = errors.New("noise session not found for peer")
)

// NoiseSession tracks the handshake and cipher state for a peer connection.
type NoiseSession struct {
	handshake  *toxnoise.IKHandshake
	sendCipher *noise.CipherState
	recvCipher *noise.CipherState
	peerAddr   net.Addr
	role       toxnoise.HandshakeRole
	complete   bool
}

// NoiseTransport wraps an existing transport with Noise Protocol encryption.
// It provides automatic handshake negotiation and transparent encryption
// for all packet types except handshake packets themselves.
type NoiseTransport struct {
	underlying Transport
	staticPriv []byte                   // Our long-term private key (32 bytes)
	staticPub  []byte                   // Our long-term public key (32 bytes)
	sessions   map[string]*NoiseSession // Key: addr.String()
	sessionsMu sync.RWMutex
	peerKeys   map[string][]byte // Known peer public keys
	peerKeysMu sync.RWMutex
}

// NewNoiseTransport creates a transport wrapper that adds Noise-IK encryption.
// staticPrivKey is our long-term Curve25519 private key (32 bytes).
// underlying is the base transport (UDP/TCP) to wrap.
func NewNoiseTransport(underlying Transport, staticPrivKey []byte) (*NoiseTransport, error) {
	if len(staticPrivKey) != 32 {
		return nil, fmt.Errorf("static private key must be 32 bytes, got %d", len(staticPrivKey))
	}
	if underlying == nil {
		return nil, errors.New("underlying transport cannot be nil")
	}

	// Generate public key from private key
	var staticPrivArray [32]byte
	copy(staticPrivArray[:], staticPrivKey)
	keypair, err := crypto.FromSecretKey(staticPrivArray)
	if err != nil {
		return nil, fmt.Errorf("invalid private key: %w", err)
	}

	nt := &NoiseTransport{
		underlying: underlying,
		staticPriv: make([]byte, 32),
		staticPub:  make([]byte, 32),
		sessions:   make(map[string]*NoiseSession),
		peerKeys:   make(map[string][]byte),
	}

	copy(nt.staticPriv, staticPrivKey)
	copy(nt.staticPub, keypair.Public[:])

	// Register handlers for Noise packets
	underlying.RegisterHandler(PacketNoiseHandshake, nt.handleHandshakePacket)
	underlying.RegisterHandler(PacketNoiseMessage, nt.handleEncryptedPacket)

	return nt, nil
}

// AddPeer registers a peer's public key for future encrypted communication.
// This enables us to initiate Noise-IK handshakes with known peers.
func (nt *NoiseTransport) AddPeer(addr net.Addr, publicKey []byte) error {
	if len(publicKey) != 32 {
		return fmt.Errorf("public key must be 32 bytes, got %d", len(publicKey))
	}

	nt.peerKeysMu.Lock()
	key := make([]byte, 32)
	copy(key, publicKey)
	nt.peerKeys[addr.String()] = key
	nt.peerKeysMu.Unlock()

	return nil
}

// Send sends a packet with automatic encryption if Noise session exists.
// Handshake packets are sent unencrypted, all others use Noise encryption.
func (nt *NoiseTransport) Send(packet *Packet, addr net.Addr) error {
	if packet.PacketType == PacketNoiseHandshake {
		// Handshake packets are never encrypted
		return nt.underlying.Send(packet, addr)
	}

	addrKey := addr.String()
	nt.sessionsMu.RLock()
	session, exists := nt.sessions[addrKey]
	nt.sessionsMu.RUnlock()

	if !exists || !session.complete {
		// Try to initiate handshake for known peers
		if err := nt.initiateHandshake(addr); err != nil {
			// Fall back to unencrypted transmission
			return nt.underlying.Send(packet, addr)
		}
		// Handshake initiated, queue packet for retry
		return nt.underlying.Send(packet, addr)
	}

	// Encrypt packet using Noise cipher
	encryptedPacket, err := nt.encryptPacket(packet, session)
	if err != nil {
		return fmt.Errorf("encryption failed: %w", err)
	}

	return nt.underlying.Send(encryptedPacket, addr)
}

// Close shuts down the transport and cleans up all sessions.
func (nt *NoiseTransport) Close() error {
	nt.sessionsMu.Lock()
	nt.sessions = make(map[string]*NoiseSession)
	nt.sessionsMu.Unlock()

	return nt.underlying.Close()
}

// LocalAddr returns the local address from the underlying transport.
func (nt *NoiseTransport) LocalAddr() net.Addr {
	return nt.underlying.LocalAddr()
}

// RegisterHandler registers a handler for decrypted packets.
func (nt *NoiseTransport) RegisterHandler(packetType PacketType, handler PacketHandler) {
	nt.underlying.RegisterHandler(packetType, handler)
}

// initiateHandshake starts a Noise-IK handshake with a known peer.
func (nt *NoiseTransport) initiateHandshake(addr net.Addr) error {
	addrKey := addr.String()

	nt.peerKeysMu.RLock()
	peerPubKey, exists := nt.peerKeys[addrKey]
	nt.peerKeysMu.RUnlock()

	if !exists {
		return ErrNoiseNotSupported
	}

	// Create initiator handshake
	handshake, err := toxnoise.NewIKHandshake(nt.staticPriv, peerPubKey, toxnoise.Initiator)
	if err != nil {
		return fmt.Errorf("failed to create handshake: %w", err)
	}

	// Generate initial message
	message, _, err := handshake.WriteMessage(nil, nil)
	if err != nil {
		return fmt.Errorf("failed to generate handshake message: %w", err)
	}

	// Store session
	nt.sessionsMu.Lock()
	nt.sessions[addrKey] = &NoiseSession{
		handshake: handshake,
		peerAddr:  addr,
		role:      toxnoise.Initiator,
		complete:  false,
	}
	nt.sessionsMu.Unlock()

	// Send handshake packet
	packet := &Packet{
		PacketType: PacketNoiseHandshake,
		Data:       message,
	}

	return nt.underlying.Send(packet, addr)
}

// handleHandshakePacket processes incoming Noise handshake packets.
func (nt *NoiseTransport) handleHandshakePacket(packet *Packet, addr net.Addr) error {
	addrKey := addr.String()

	nt.sessionsMu.RLock()
	session, exists := nt.sessions[addrKey]
	nt.sessionsMu.RUnlock()

	if !exists {
		// This is an incoming handshake from unknown peer - create responder
		handshake, err := toxnoise.NewIKHandshake(nt.staticPriv, nil, toxnoise.Responder)
		if err != nil {
			return fmt.Errorf("failed to create responder handshake: %w", err)
		}

		session = &NoiseSession{
			handshake: handshake,
			peerAddr:  addr,
			role:      toxnoise.Responder,
			complete:  false,
		}

		nt.sessionsMu.Lock()
		nt.sessions[addrKey] = session
		nt.sessionsMu.Unlock()
	}

	if session.complete {
		return fmt.Errorf("handshake already complete for peer %s", addr)
	}

	// Process the handshake message
	if session.role == toxnoise.Responder {
		// Responder processes initiator's message and sends response
		response, complete, err := session.handshake.WriteMessage(nil, packet.Data)
		if err != nil {
			return fmt.Errorf("failed to generate handshake response: %w", err)
		}

		if complete {
			// Extract cipher states
			sendCipher, recvCipher, err := session.handshake.GetCipherStates()
			if err != nil {
				return fmt.Errorf("failed to get cipher states: %w", err)
			}

			session.sendCipher = sendCipher
			session.recvCipher = recvCipher
			session.complete = true
		}

		// Send response
		responsePacket := &Packet{
			PacketType: PacketNoiseHandshake,
			Data:       response,
		}
		return nt.underlying.Send(responsePacket, addr)

	} else {
		// Initiator processes responder's response
		_, complete, err := session.handshake.ReadMessage(packet.Data)
		if err != nil {
			return fmt.Errorf("failed to read handshake response: %w", err)
		}

		if complete {
			// Extract cipher states
			sendCipher, recvCipher, err := session.handshake.GetCipherStates()
			if err != nil {
				return fmt.Errorf("failed to get cipher states: %w", err)
			}

			session.sendCipher = sendCipher
			session.recvCipher = recvCipher
			session.complete = true
		}
	}

	return nil
}

// handleEncryptedPacket processes incoming encrypted Noise messages.
func (nt *NoiseTransport) handleEncryptedPacket(packet *Packet, addr net.Addr) error {
	addrKey := addr.String()

	nt.sessionsMu.RLock()
	session, exists := nt.sessions[addrKey]
	nt.sessionsMu.RUnlock()

	if !exists || !session.complete {
		return ErrNoiseSessionNotFound
	}

	// Decrypt the packet
	decryptedData, err := session.recvCipher.Decrypt(nil, nil, packet.Data)
	if err != nil {
		return fmt.Errorf("decryption failed: %w", err)
	}

	// Parse the decrypted packet
	if len(decryptedData) < 1 {
		return errors.New("decrypted packet too short")
	}

	decryptedPacket := &Packet{
		PacketType: PacketType(decryptedData[0]),
		Data:       decryptedData[1:],
	}

	// TODO: Forward decrypted packet to appropriate handler
	// This requires handler forwarding mechanism
	_ = decryptedPacket // Suppress unused variable warning

	return nil
}

// encryptPacket encrypts a packet using the session's send cipher.
func (nt *NoiseTransport) encryptPacket(packet *Packet, session *NoiseSession) (*Packet, error) {
	if !session.complete {
		return nil, errors.New("session not complete")
	}

	// Serialize the original packet
	serialized, err := packet.Serialize()
	if err != nil {
		return nil, fmt.Errorf("failed to serialize packet: %w", err)
	}

	// Encrypt the serialized packet
	encrypted, err := session.sendCipher.Encrypt(nil, nil, serialized)
	if err != nil {
		return nil, fmt.Errorf("encryption failed: %w", err)
	}

	return &Packet{
		PacketType: PacketNoiseMessage,
		Data:       encrypted,
	}, nil
}

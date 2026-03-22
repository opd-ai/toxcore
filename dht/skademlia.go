// Package dht implements the Distributed Hash Table for the Tox protocol.
//
// This file implements S/Kademlia extensions for Sybil resistance through
// cryptographic proof-of-work on node IDs. The implementation follows the
// S/Kademlia paper by Baumgart and Mies (2007).
//
// Key concepts:
//   - Static crypto puzzle: Node ID = hash(PublicKey || Nonce) must have
//     a configurable number of leading zero bits
//   - This makes mass node creation computationally expensive for attackers
//   - Honest nodes solve the puzzle once during identity generation
package dht

import (
	"crypto/sha256"
	"crypto/subtle"
	"encoding/binary"
	"errors"
	"sync/atomic"
	"time"

	"github.com/opd-ai/toxcore/crypto"
)

const (
	// DefaultPoWDifficulty is the number of leading zero bits required in the
	// node ID hash. Higher values increase security but also honest node setup time.
	// 16 bits = ~65K hash attempts on average, takes <1 second on modern hardware.
	// 20 bits = ~1M hash attempts on average, takes ~5 seconds.
	DefaultPoWDifficulty = 16

	// MinPoWDifficulty is the minimum acceptable difficulty.
	MinPoWDifficulty = 8

	// MaxPoWDifficulty is the maximum difficulty to prevent DoS via excessive computation.
	MaxPoWDifficulty = 32

	// ProofNonceSize is the size of the nonce used in proof-of-work.
	ProofNonceSize = 8

	// ProofSignatureSize is the Ed25519 signature size.
	ProofSignatureSize = 64

	// DefaultProofCacheMaxSize is the default maximum number of verified proofs to cache.
	DefaultProofCacheMaxSize = 1024
)

var (
	// ErrInvalidProof indicates that a node's proof-of-work is invalid.
	ErrInvalidProof = errors.New("invalid node ID proof-of-work")

	// ErrInsufficientDifficulty indicates the proof doesn't meet the required difficulty.
	ErrInsufficientDifficulty = errors.New("proof-of-work difficulty insufficient")

	// ErrProofRequired indicates that a proof is required but not provided.
	ErrProofRequired = errors.New("node ID proof required")

	// ErrInvalidSignature indicates the proof signature is invalid.
	ErrInvalidSignature = errors.New("invalid proof signature")
)

// NodeIDProof represents the cryptographic proof that a node legitimately
// owns its node ID. It contains:
//   - The nonce used to solve the proof-of-work puzzle
//   - A signature proving ownership of the private key
//   - The computed hash that demonstrates the PoW solution
type NodeIDProof struct {
	// Nonce is the value found that makes hash(PublicKey || Nonce) have
	// the required number of leading zero bits.
	Nonce [ProofNonceSize]byte

	// Signature is an Ed25519 signature over (PublicKey || Nonce) proving
	// the node controls the corresponding private key.
	Signature [ProofSignatureSize]byte

	// Difficulty is the number of leading zero bits this proof satisfies.
	// Stored to allow verification without recounting.
	Difficulty uint8

	// Timestamp is when the proof was generated. Nodes may reject proofs
	// older than a certain age to prevent replay attacks with pre-computed IDs.
	Timestamp time.Time
}

// SKademliaConfig holds configuration for S/Kademlia extensions.
type SKademliaConfig struct {
	// RequireProofs determines whether nodes must provide valid proofs to be added.
	// When false (default), proofs are verified if present but not required.
	// This allows gradual migration of existing networks.
	RequireProofs bool

	// MinDifficulty is the minimum acceptable proof-of-work difficulty.
	MinDifficulty uint8

	// MaxProofAge is the maximum age of a proof before it's considered stale.
	// Zero means no age limit.
	MaxProofAge time.Duration
}

// DefaultSKademliaConfig returns the default S/Kademlia configuration.
func DefaultSKademliaConfig() *SKademliaConfig {
	return &SKademliaConfig{
		RequireProofs: false, // Backward compatible by default
		MinDifficulty: DefaultPoWDifficulty,
		MaxProofAge:   0, // No age limit by default
	}
}

// GenerateNodeIDProof solves the proof-of-work puzzle for the given key pair.
// It finds a nonce such that hash(publicKey || nonce) has at least 'difficulty'
// leading zero bits, then signs the result to prove key ownership.
//
// This is a CPU-intensive operation. For difficulty=16, expect ~65K iterations
// taking less than 1 second on modern hardware.
func GenerateNodeIDProof(publicKey [32]byte, privateKey [64]byte, difficulty uint8) (*NodeIDProof, error) {
	if difficulty < MinPoWDifficulty {
		difficulty = MinPoWDifficulty
	}
	if difficulty > MaxPoWDifficulty {
		difficulty = MaxPoWDifficulty
	}

	proof := &NodeIDProof{
		Difficulty: difficulty,
		Timestamp:  time.Now().UTC(),
	}

	// Prepare the data buffer: publicKey (32) + nonce (8)
	data := make([]byte, 32+ProofNonceSize)
	copy(data[:32], publicKey[:])

	// Find a nonce that produces a hash with enough leading zeros
	var nonce uint64
	for {
		binary.BigEndian.PutUint64(data[32:], nonce)

		hash := sha256.Sum256(data)
		if countLeadingZeroBits(hash[:]) >= int(difficulty) {
			// Found valid nonce
			binary.BigEndian.PutUint64(proof.Nonce[:], nonce)
			break
		}
		nonce++
	}

	// Sign the proof: Ed25519(privateKey, publicKey || nonce)
	signature, err := crypto.SignWithPrivateKey(privateKey, data)
	if err != nil {
		return nil, err
	}
	copy(proof.Signature[:], signature[:])

	return proof, nil
}

// GenerateNodeIDProofWithCancel is like GenerateNodeIDProof but can be
// cancelled via a stop channel. Returns nil, nil if cancelled.
func GenerateNodeIDProofWithCancel(publicKey [32]byte, privateKey [64]byte, difficulty uint8, stop <-chan struct{}) (*NodeIDProof, error) {
	difficulty = clampDifficulty(difficulty)

	proof := &NodeIDProof{
		Difficulty: difficulty,
		Timestamp:  time.Now().UTC(),
	}

	data := make([]byte, 32+ProofNonceSize)
	copy(data[:32], publicKey[:])

	nonce, cancelled := findValidNonce(data, difficulty, stop)
	if cancelled {
		return nil, nil
	}

	binary.BigEndian.PutUint64(proof.Nonce[:], nonce)
	return signProof(proof, privateKey, data)
}

// clampDifficulty ensures difficulty is within valid bounds.
func clampDifficulty(difficulty uint8) uint8 {
	if difficulty < MinPoWDifficulty {
		return MinPoWDifficulty
	}
	if difficulty > MaxPoWDifficulty {
		return MaxPoWDifficulty
	}
	return difficulty
}

// findValidNonce searches for a nonce that produces a hash with enough leading zeros.
// Returns the nonce and whether the search was cancelled.
func findValidNonce(data []byte, difficulty uint8, stop <-chan struct{}) (uint64, bool) {
	var nonce uint64
	checkInterval := uint64(10000) // Check cancel every 10K iterations

	for {
		if nonce%checkInterval == 0 && isCancelled(stop) {
			return 0, true
		}

		binary.BigEndian.PutUint64(data[32:], nonce)
		hash := sha256.Sum256(data)

		if countLeadingZeroBits(hash[:]) >= int(difficulty) {
			return nonce, false
		}
		nonce++
	}
}

// isCancelled checks if the stop channel has been closed.
func isCancelled(stop <-chan struct{}) bool {
	select {
	case <-stop:
		return true
	default:
		return false
	}
}

// signProof signs the proof data and populates the signature.
func signProof(proof *NodeIDProof, privateKey [64]byte, data []byte) (*NodeIDProof, error) {
	signature, err := crypto.SignWithPrivateKey(privateKey, data)
	if err != nil {
		return nil, err
	}
	copy(proof.Signature[:], signature[:])
	return proof, nil
}

// VerifyNodeIDProof verifies that a proof is valid for the given public key
// and meets the minimum difficulty requirement.
func VerifyNodeIDProof(publicKey [32]byte, proof *NodeIDProof, minDifficulty uint8) error {
	if proof == nil {
		return ErrProofRequired
	}

	if proof.Difficulty < minDifficulty {
		return ErrInsufficientDifficulty
	}

	// Reconstruct the data that was signed: publicKey || nonce
	data := make([]byte, 32+ProofNonceSize)
	copy(data[:32], publicKey[:])
	copy(data[32:], proof.Nonce[:])

	// Verify the hash has sufficient leading zeros
	hash := sha256.Sum256(data)
	actualDifficulty := countLeadingZeroBits(hash[:])
	if actualDifficulty < int(minDifficulty) {
		return ErrInvalidProof
	}

	// Verify the signature proves ownership of the private key
	valid, err := crypto.VerifySignature(publicKey, data, proof.Signature)
	if err != nil {
		return err
	}
	if !valid {
		return ErrInvalidSignature
	}

	return nil
}

// VerifyNodeIDProofWithConfig verifies a proof with additional configuration.
func VerifyNodeIDProofWithConfig(publicKey [32]byte, proof *NodeIDProof, config *SKademliaConfig) error {
	if config == nil {
		config = DefaultSKademliaConfig()
	}

	if proof == nil {
		if config.RequireProofs {
			return ErrProofRequired
		}
		return nil // Proof not required and not provided
	}

	// Check proof age if configured
	if config.MaxProofAge > 0 {
		proofAge := time.Since(proof.Timestamp)
		if proofAge > config.MaxProofAge {
			return ErrInvalidProof
		}
	}

	return VerifyNodeIDProof(publicKey, proof, config.MinDifficulty)
}

// countLeadingZeroBits counts the number of leading zero bits in a byte slice.
func countLeadingZeroBits(data []byte) int {
	zeros := 0
	for _, b := range data {
		if b == 0 {
			zeros += 8
			continue
		}
		// Count leading zeros in this byte
		for i := 7; i >= 0; i-- {
			if (b>>i)&1 == 0 {
				zeros++
			} else {
				return zeros
			}
		}
	}
	return zeros
}

// ComputeNodeIDHash computes the hash used for proof-of-work verification.
// This can be used to inspect what hash a proof produces.
func ComputeNodeIDHash(publicKey [32]byte, nonce [ProofNonceSize]byte) [32]byte {
	data := make([]byte, 32+ProofNonceSize)
	copy(data[:32], publicKey[:])
	copy(data[32:], nonce[:])
	return sha256.Sum256(data)
}

// EstimatePoWTime estimates the time to generate a proof at the given difficulty.
// Returns the expected number of hash operations and estimated time.
func EstimatePoWTime(difficulty uint8, hashesPerSecond uint64) (expectedHashes uint64, estimatedDuration time.Duration) {
	if hashesPerSecond == 0 {
		hashesPerSecond = 1000000 // Default: 1M hashes/sec on modern CPU
	}
	expectedHashes = 1 << difficulty
	estimatedDuration = time.Duration(expectedHashes/hashesPerSecond) * time.Second
	return expectedHashes, estimatedDuration
}

// NodeWithProof extends a Node with its S/Kademlia proof.
type NodeWithProof struct {
	*Node
	Proof *NodeIDProof
}

// SKademliaRoutingTable wraps a RoutingTable with S/Kademlia proof verification.
type SKademliaRoutingTable struct {
	*RoutingTable
	config      *SKademliaConfig
	proofCache  map[[32]byte]*NodeIDProof // Cache of verified proofs
	verifyStats SKademliaStats
}

// SKademliaStats tracks S/Kademlia verification statistics.
type SKademliaStats struct {
	NodesVerified uint64
	NodesRejected uint64
	ProofsMissing uint64
	ProofsInvalid uint64
	ProofsExpired uint64
	CacheHits     uint64
	CacheMisses   uint64
}

// NewSKademliaRoutingTable creates a routing table with S/Kademlia extensions.
func NewSKademliaRoutingTable(selfID crypto.ToxID, maxBucketSize int, config *SKademliaConfig) *SKademliaRoutingTable {
	if config == nil {
		config = DefaultSKademliaConfig()
	}
	return &SKademliaRoutingTable{
		RoutingTable: NewRoutingTable(selfID, maxBucketSize),
		config:       config,
		proofCache:   make(map[[32]byte]*NodeIDProof),
	}
}

// AddNodeWithProof adds a node to the routing table after verifying its proof.
func (srt *SKademliaRoutingTable) AddNodeWithProof(node *Node, proof *NodeIDProof) (bool, error) {
	// Check proof cache first
	srt.mu.Lock()
	cachedProof, cached := srt.proofCache[node.PublicKey]
	if cached {
		atomic.AddUint64(&srt.verifyStats.CacheHits, 1)
		srt.mu.Unlock()
		// Use cached proof if node didn't provide one
		if proof == nil {
			proof = cachedProof
		}
	} else {
		atomic.AddUint64(&srt.verifyStats.CacheMisses, 1)
		srt.mu.Unlock()
	}

	// Verify the proof
	err := VerifyNodeIDProofWithConfig(node.PublicKey, proof, srt.config)
	if err != nil {
		atomic.AddUint64(&srt.verifyStats.NodesRejected, 1)
		switch err {
		case ErrProofRequired:
			atomic.AddUint64(&srt.verifyStats.ProofsMissing, 1)
		case ErrInvalidProof, ErrInvalidSignature, ErrInsufficientDifficulty:
			atomic.AddUint64(&srt.verifyStats.ProofsInvalid, 1)
		}
		return false, err
	}

	// Cache valid proof
	if proof != nil && !cached {
		srt.mu.Lock()
		srt.proofCache[node.PublicKey] = proof
		srt.mu.Unlock()
	}

	atomic.AddUint64(&srt.verifyStats.NodesVerified, 1)
	return srt.RoutingTable.AddNode(node), nil
}

// GetStats returns S/Kademlia verification statistics.
func (srt *SKademliaRoutingTable) GetStats() SKademliaStats {
	return SKademliaStats{
		NodesVerified: atomic.LoadUint64(&srt.verifyStats.NodesVerified),
		NodesRejected: atomic.LoadUint64(&srt.verifyStats.NodesRejected),
		ProofsMissing: atomic.LoadUint64(&srt.verifyStats.ProofsMissing),
		ProofsInvalid: atomic.LoadUint64(&srt.verifyStats.ProofsInvalid),
		ProofsExpired: atomic.LoadUint64(&srt.verifyStats.ProofsExpired),
		CacheHits:     atomic.LoadUint64(&srt.verifyStats.CacheHits),
		CacheMisses:   atomic.LoadUint64(&srt.verifyStats.CacheMisses),
	}
}

// ClearProofCache clears the proof verification cache.
func (srt *SKademliaRoutingTable) ClearProofCache() {
	srt.mu.Lock()
	defer srt.mu.Unlock()
	srt.proofCache = make(map[[32]byte]*NodeIDProof)
}

// SetConfig updates the S/Kademlia configuration.
func (srt *SKademliaRoutingTable) SetConfig(config *SKademliaConfig) {
	if config != nil {
		srt.config = config
	}
}

// GetConfig returns the current S/Kademlia configuration.
func (srt *SKademliaRoutingTable) GetConfig() *SKademliaConfig {
	return srt.config
}

// ValidateExistingNodes validates all nodes in the routing table against cached proofs.
// Returns the count of nodes without valid proofs. Useful when enabling RequireProofs
// on an existing network.
func (srt *SKademliaRoutingTable) ValidateExistingNodes() (validCount, invalidCount int) {
	allNodes := srt.GetAllNodes()
	for _, node := range allNodes {
		srt.mu.RLock()
		proof := srt.proofCache[node.PublicKey]
		srt.mu.RUnlock()

		err := VerifyNodeIDProofWithConfig(node.PublicKey, proof, srt.config)
		if err == nil {
			validCount++
		} else {
			invalidCount++
		}
	}
	return validCount, invalidCount
}

// CompareProofDifficulty compares two proofs by difficulty.
// Returns -1 if a < b, 0 if a == b, 1 if a > b.
// A nil proof is considered to have difficulty 0.
func CompareProofDifficulty(a, b *NodeIDProof) int {
	var aDiff, bDiff uint8
	if a != nil {
		aDiff = a.Difficulty
	}
	if b != nil {
		bDiff = b.Difficulty
	}

	if aDiff < bDiff {
		return -1
	} else if aDiff > bDiff {
		return 1
	}
	return 0
}

// ConstantTimeCompare performs constant-time comparison of two proofs to prevent timing attacks.
func (p *NodeIDProof) ConstantTimeCompare(other *NodeIDProof) bool {
	if p == nil || other == nil {
		return p == other
	}

	nonceMatch := subtle.ConstantTimeCompare(p.Nonce[:], other.Nonce[:]) == 1
	sigMatch := subtle.ConstantTimeCompare(p.Signature[:], other.Signature[:]) == 1
	diffMatch := p.Difficulty == other.Difficulty

	return nonceMatch && sigMatch && diffMatch
}

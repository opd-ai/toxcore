package dht

import (
	"sync"
	"testing"
	"time"

	"github.com/opd-ai/toxcore/crypto"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestCountLeadingZeroBits(t *testing.T) {
	tests := []struct {
		name     string
		data     []byte
		expected int
	}{
		{"all zeros", []byte{0x00, 0x00, 0x00, 0x00}, 32},
		{"first bit set", []byte{0x80, 0x00}, 0},
		{"second bit set", []byte{0x40, 0x00}, 1},
		{"8 leading zeros", []byte{0x00, 0x80}, 8},
		{"16 leading zeros", []byte{0x00, 0x00, 0x80}, 16},
		{"5 leading zeros", []byte{0x04}, 5},
		{"mixed", []byte{0x00, 0x00, 0x01}, 23},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := countLeadingZeroBits(tt.data)
			assert.Equal(t, tt.expected, result)
		})
	}
}

func TestGenerateNodeIDProof(t *testing.T) {
	// Generate a key pair for testing
	privateKey, publicKey, err := crypto.GenerateEd25519KeyPair()
	require.NoError(t, err)

	t.Run("generates valid proof at minimum difficulty", func(t *testing.T) {
		proof, err := GenerateNodeIDProof(publicKey, privateKey, MinPoWDifficulty)
		require.NoError(t, err)
		require.NotNil(t, proof)

		assert.Equal(t, uint8(MinPoWDifficulty), proof.Difficulty)
		assert.False(t, proof.Timestamp.IsZero())

		// Verify the proof is valid
		err = VerifyNodeIDProof(publicKey, proof, MinPoWDifficulty)
		assert.NoError(t, err)
	})

	t.Run("proof hash has required leading zeros", func(t *testing.T) {
		difficulty := uint8(12)
		proof, err := GenerateNodeIDProof(publicKey, privateKey, difficulty)
		require.NoError(t, err)

		hash := ComputeNodeIDHash(publicKey, proof.Nonce)
		zeros := countLeadingZeroBits(hash[:])
		assert.GreaterOrEqual(t, zeros, int(difficulty))
	})

	t.Run("clamps difficulty to valid range", func(t *testing.T) {
		// Below minimum should be clamped up
		proof, err := GenerateNodeIDProof(publicKey, privateKey, 2)
		require.NoError(t, err)
		assert.Equal(t, uint8(MinPoWDifficulty), proof.Difficulty)

		// Note: We don't test above maximum here because generating a proof
		// at MaxPoWDifficulty=32 would take billions of hash operations.
		// The clamping logic is verified by code inspection instead.
	})
}

func TestGenerateNodeIDProofWithCancel(t *testing.T) {
	privateKey, publicKey, err := crypto.GenerateEd25519KeyPair()
	require.NoError(t, err)

	t.Run("can be cancelled", func(t *testing.T) {
		stop := make(chan struct{})

		// Start proof generation and cancel immediately
		go func() {
			time.Sleep(10 * time.Millisecond)
			close(stop)
		}()

		// Use a moderately high difficulty that gives time to cancel
		// but doesn't risk completing too fast
		proof, err := GenerateNodeIDProofWithCancel(publicKey, privateKey, 24, stop)
		assert.Nil(t, proof)
		assert.NoError(t, err) // Returns nil, nil when cancelled
	})

	t.Run("completes normally if not cancelled", func(t *testing.T) {
		stop := make(chan struct{})
		// Don't close stop - let it complete normally

		proof, err := GenerateNodeIDProofWithCancel(publicKey, privateKey, MinPoWDifficulty, stop)
		require.NoError(t, err)
		require.NotNil(t, proof)

		err = VerifyNodeIDProof(publicKey, proof, MinPoWDifficulty)
		assert.NoError(t, err)
	})
}

func TestVerifyNodeIDProof(t *testing.T) {
	privateKey, publicKey, err := crypto.GenerateEd25519KeyPair()
	require.NoError(t, err)

	validProof, err := GenerateNodeIDProof(publicKey, privateKey, DefaultPoWDifficulty)
	require.NoError(t, err)

	t.Run("accepts valid proof", func(t *testing.T) {
		err := VerifyNodeIDProof(publicKey, validProof, DefaultPoWDifficulty)
		assert.NoError(t, err)
	})

	t.Run("rejects nil proof", func(t *testing.T) {
		err := VerifyNodeIDProof(publicKey, nil, DefaultPoWDifficulty)
		assert.Equal(t, ErrProofRequired, err)
	})

	t.Run("rejects insufficient difficulty", func(t *testing.T) {
		lowDiffProof, err := GenerateNodeIDProof(publicKey, privateKey, MinPoWDifficulty)
		require.NoError(t, err)

		err = VerifyNodeIDProof(publicKey, lowDiffProof, MinPoWDifficulty+5)
		assert.Equal(t, ErrInsufficientDifficulty, err)
	})

	t.Run("rejects tampered nonce", func(t *testing.T) {
		tamperedProof := *validProof
		tamperedProof.Nonce[0] ^= 0xFF // Flip bits in nonce

		err := VerifyNodeIDProof(publicKey, &tamperedProof, DefaultPoWDifficulty)
		// Could be either ErrInvalidProof (hash check fails) or ErrInvalidSignature
		assert.Error(t, err)
	})

	t.Run("rejects wrong public key", func(t *testing.T) {
		_, wrongPublicKey, err := crypto.GenerateEd25519KeyPair()
		require.NoError(t, err)

		err = VerifyNodeIDProof(wrongPublicKey, validProof, DefaultPoWDifficulty)
		assert.Error(t, err)
	})
}

func TestVerifyNodeIDProofWithConfig(t *testing.T) {
	privateKey, publicKey, err := crypto.GenerateEd25519KeyPair()
	require.NoError(t, err)

	t.Run("allows missing proof when not required", func(t *testing.T) {
		config := &SKademliaConfig{
			RequireProofs: false,
			MinDifficulty: DefaultPoWDifficulty,
		}

		err := VerifyNodeIDProofWithConfig(publicKey, nil, config)
		assert.NoError(t, err)
	})

	t.Run("rejects missing proof when required", func(t *testing.T) {
		config := &SKademliaConfig{
			RequireProofs: true,
			MinDifficulty: DefaultPoWDifficulty,
		}

		err := VerifyNodeIDProofWithConfig(publicKey, nil, config)
		assert.Equal(t, ErrProofRequired, err)
	})

	t.Run("rejects expired proof when age limit set", func(t *testing.T) {
		proof, err := GenerateNodeIDProof(publicKey, privateKey, MinPoWDifficulty)
		require.NoError(t, err)

		// Set timestamp in the past
		proof.Timestamp = time.Now().Add(-2 * time.Hour)

		config := &SKademliaConfig{
			RequireProofs: true,
			MinDifficulty: MinPoWDifficulty,
			MaxProofAge:   1 * time.Hour,
		}

		err = VerifyNodeIDProofWithConfig(publicKey, proof, config)
		assert.Equal(t, ErrInvalidProof, err)
	})
}

func TestSKademliaRoutingTable(t *testing.T) {
	selfPrivateKey, selfPublicKey, err := crypto.GenerateEd25519KeyPair()
	require.NoError(t, err)
	_ = selfPrivateKey // Not used directly

	selfID := crypto.ToxID{PublicKey: selfPublicKey}

	t.Run("creates with default config", func(t *testing.T) {
		srt := NewSKademliaRoutingTable(selfID, 8, nil)
		require.NotNil(t, srt)
		assert.NotNil(t, srt.config)
		assert.False(t, srt.config.RequireProofs)
	})

	t.Run("accepts node with valid proof", func(t *testing.T) {
		config := &SKademliaConfig{
			RequireProofs: true,
			MinDifficulty: MinPoWDifficulty,
		}
		srt := NewSKademliaRoutingTable(selfID, 8, config)

		// Create a node with valid proof
		nodePrivateKey, nodePublicKey, err := crypto.GenerateEd25519KeyPair()
		require.NoError(t, err)

		proof, err := GenerateNodeIDProof(nodePublicKey, nodePrivateKey, MinPoWDifficulty)
		require.NoError(t, err)

		nodeID := crypto.ToxID{PublicKey: nodePublicKey}
		node := NewNode(nodeID, &mockAddr{addr: "192.168.1.100:33445"})

		added, err := srt.AddNodeWithProof(node, proof)
		assert.NoError(t, err)
		assert.True(t, added)

		stats := srt.GetStats()
		assert.Equal(t, uint64(1), stats.NodesVerified)
		assert.Equal(t, uint64(0), stats.NodesRejected)
	})

	t.Run("rejects node without proof when required", func(t *testing.T) {
		config := &SKademliaConfig{
			RequireProofs: true,
			MinDifficulty: MinPoWDifficulty,
		}
		srt := NewSKademliaRoutingTable(selfID, 8, config)

		_, nodePublicKey, err := crypto.GenerateEd25519KeyPair()
		require.NoError(t, err)

		nodeID := crypto.ToxID{PublicKey: nodePublicKey}
		node := NewNode(nodeID, &mockAddr{addr: "192.168.1.101:33445"})

		added, err := srt.AddNodeWithProof(node, nil)
		assert.Error(t, err)
		assert.False(t, added)

		stats := srt.GetStats()
		assert.Equal(t, uint64(0), stats.NodesVerified)
		assert.Equal(t, uint64(1), stats.NodesRejected)
		assert.Equal(t, uint64(1), stats.ProofsMissing)
	})

	t.Run("accepts node without proof when not required", func(t *testing.T) {
		config := &SKademliaConfig{
			RequireProofs: false,
			MinDifficulty: MinPoWDifficulty,
		}
		srt := NewSKademliaRoutingTable(selfID, 8, config)

		_, nodePublicKey, err := crypto.GenerateEd25519KeyPair()
		require.NoError(t, err)

		nodeID := crypto.ToxID{PublicKey: nodePublicKey}
		node := NewNode(nodeID, &mockAddr{addr: "192.168.1.102:33445"})

		added, err := srt.AddNodeWithProof(node, nil)
		assert.NoError(t, err)
		assert.True(t, added)
	})

	t.Run("caches verified proofs", func(t *testing.T) {
		config := &SKademliaConfig{
			RequireProofs: true,
			MinDifficulty: MinPoWDifficulty,
		}
		srt := NewSKademliaRoutingTable(selfID, 8, config)

		nodePrivateKey, nodePublicKey, err := crypto.GenerateEd25519KeyPair()
		require.NoError(t, err)

		proof, err := GenerateNodeIDProof(nodePublicKey, nodePrivateKey, MinPoWDifficulty)
		require.NoError(t, err)

		nodeID := crypto.ToxID{PublicKey: nodePublicKey}

		// First add with proof
		node1 := NewNode(nodeID, &mockAddr{addr: "192.168.1.103:33445"})
		_, err = srt.AddNodeWithProof(node1, proof)
		require.NoError(t, err)

		stats := srt.GetStats()
		assert.Equal(t, uint64(1), stats.CacheMisses)

		// Second add should use cached proof
		node2 := NewNode(nodeID, &mockAddr{addr: "192.168.1.103:33446"})
		_, err = srt.AddNodeWithProof(node2, nil)
		require.NoError(t, err)

		stats = srt.GetStats()
		assert.Equal(t, uint64(1), stats.CacheHits)
	})
}

func TestEstimatePoWTime(t *testing.T) {
	tests := []struct {
		difficulty       uint8
		hashesPerSecond  uint64
		expectedHashes   uint64
		expectedDuration time.Duration
	}{
		{8, 1000000, 256, 0},
		{16, 1000000, 65536, 0},
		{20, 1000000, 1048576, time.Second},
	}

	for _, tt := range tests {
		t.Run("", func(t *testing.T) {
			hashes, duration := EstimatePoWTime(tt.difficulty, tt.hashesPerSecond)
			assert.Equal(t, tt.expectedHashes, hashes)
			assert.Equal(t, tt.expectedDuration, duration)
		})
	}
}

func TestCompareProofDifficulty(t *testing.T) {
	proofLow := &NodeIDProof{Difficulty: 10}
	proofHigh := &NodeIDProof{Difficulty: 20}

	assert.Equal(t, -1, CompareProofDifficulty(proofLow, proofHigh))
	assert.Equal(t, 1, CompareProofDifficulty(proofHigh, proofLow))
	assert.Equal(t, 0, CompareProofDifficulty(proofLow, proofLow))
	assert.Equal(t, -1, CompareProofDifficulty(nil, proofLow))
	assert.Equal(t, 1, CompareProofDifficulty(proofLow, nil))
	assert.Equal(t, 0, CompareProofDifficulty(nil, nil))
}

func TestNodeIDProofConstantTimeCompare(t *testing.T) {
	privateKey, publicKey, err := crypto.GenerateEd25519KeyPair()
	require.NoError(t, err)

	proof1, err := GenerateNodeIDProof(publicKey, privateKey, MinPoWDifficulty)
	require.NoError(t, err)

	// Copy proof1 exactly
	proof2 := *proof1

	// Create a deliberately different proof by modifying the nonce
	proof3 := *proof1
	proof3.Nonce[0] ^= 0xFF // Flip bits to make it different

	assert.True(t, proof1.ConstantTimeCompare(&proof2))
	assert.False(t, proof1.ConstantTimeCompare(&proof3))
	assert.False(t, proof1.ConstantTimeCompare(nil))

	var nilProof *NodeIDProof
	assert.True(t, nilProof.ConstantTimeCompare(nil))
}

func TestSKademliaRoutingTableConcurrency(t *testing.T) {
	selfPrivateKey, selfPublicKey, err := crypto.GenerateEd25519KeyPair()
	require.NoError(t, err)
	_ = selfPrivateKey

	selfID := crypto.ToxID{PublicKey: selfPublicKey}
	config := &SKademliaConfig{
		RequireProofs: false,
		MinDifficulty: MinPoWDifficulty,
	}
	srt := NewSKademliaRoutingTable(selfID, 20, config)

	const numGoroutines = 10
	const nodesPerGoroutine = 5

	var wg sync.WaitGroup
	wg.Add(numGoroutines)

	for i := 0; i < numGoroutines; i++ {
		go func(idx int) {
			defer wg.Done()
			for j := 0; j < nodesPerGoroutine; j++ {
				_, nodePublicKey, err := crypto.GenerateEd25519KeyPair()
				if err != nil {
					t.Errorf("failed to generate key pair: %v", err)
					return
				}

				nodeID := crypto.ToxID{PublicKey: nodePublicKey}
				node := NewNode(nodeID, &mockAddr{addr: "192.168.1.1:33445"})
				_, _ = srt.AddNodeWithProof(node, nil)
			}
		}(i)
	}

	wg.Wait()

	stats := srt.GetStats()
	totalAttempts := stats.NodesVerified + stats.NodesRejected
	assert.GreaterOrEqual(t, totalAttempts, uint64(numGoroutines*nodesPerGoroutine))
}

func TestValidateExistingNodes(t *testing.T) {
	selfPrivateKey, selfPublicKey, err := crypto.GenerateEd25519KeyPair()
	require.NoError(t, err)
	_ = selfPrivateKey

	selfID := crypto.ToxID{PublicKey: selfPublicKey}
	config := &SKademliaConfig{
		RequireProofs: false,
		MinDifficulty: MinPoWDifficulty,
	}
	srt := NewSKademliaRoutingTable(selfID, 8, config)

	// Add some nodes with proofs
	for i := 0; i < 3; i++ {
		nodePrivateKey, nodePublicKey, err := crypto.GenerateEd25519KeyPair()
		require.NoError(t, err)

		proof, err := GenerateNodeIDProof(nodePublicKey, nodePrivateKey, MinPoWDifficulty)
		require.NoError(t, err)

		nodeID := crypto.ToxID{PublicKey: nodePublicKey}
		node := NewNode(nodeID, &mockAddr{addr: "192.168.1.1:33445"})
		_, _ = srt.AddNodeWithProof(node, proof)
	}

	// Add some nodes without proofs (invalid when RequireProofs is false,
	// but would be invalid if we later enable RequireProofs)
	for i := 0; i < 2; i++ {
		_, nodePublicKey, err := crypto.GenerateEd25519KeyPair()
		require.NoError(t, err)

		nodeID := crypto.ToxID{PublicKey: nodePublicKey}
		node := NewNode(nodeID, &mockAddr{addr: "192.168.1.2:33445"})
		_, _ = srt.AddNodeWithProof(node, nil)
	}

	// Validate with proofs required
	srt.config.RequireProofs = true
	valid, invalid := srt.ValidateExistingNodes()

	assert.Equal(t, 3, valid)
	assert.Equal(t, 2, invalid)
}

// mockAddr implements net.Addr for testing
type mockAddr struct {
	addr string
}

func (m *mockAddr) Network() string { return "udp" }
func (m *mockAddr) String() string  { return m.addr }

func BenchmarkGenerateNodeIDProof(b *testing.B) {
	privateKey, publicKey, _ := crypto.GenerateEd25519KeyPair()

	difficulties := []uint8{8, 12, 16}

	for _, diff := range difficulties {
		b.Run("difficulty_"+string(rune('0'+diff/10))+string(rune('0'+diff%10)), func(b *testing.B) {
			b.ResetTimer()
			for i := 0; i < b.N; i++ {
				_, _ = GenerateNodeIDProof(publicKey, privateKey, diff)
			}
		})
	}
}

func BenchmarkVerifyNodeIDProof(b *testing.B) {
	privateKey, publicKey, _ := crypto.GenerateEd25519KeyPair()
	proof, _ := GenerateNodeIDProof(publicKey, privateKey, DefaultPoWDifficulty)

	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		_ = VerifyNodeIDProof(publicKey, proof, DefaultPoWDifficulty)
	}
}

func BenchmarkCountLeadingZeroBits(b *testing.B) {
	data := make([]byte, 32)
	// Set up data with ~16 leading zero bits
	data[2] = 0x01

	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		_ = countLeadingZeroBits(data)
	}
}

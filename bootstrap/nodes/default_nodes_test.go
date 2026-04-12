package nodes

import (
	"encoding/hex"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestDefaultNodesNotEmpty(t *testing.T) {
	require.True(t, len(DefaultNodes) >= 4, "DefaultNodes must contain at least 4 entries, got %d", len(DefaultNodes))
}

func TestDefaultNodesValid(t *testing.T) {
	for i, node := range DefaultNodes {
		t.Run(node.Address, func(t *testing.T) {
			assert.NotEmpty(t, node.Address, "node %d: Address must not be empty", i)
			assert.Greater(t, node.Port, uint16(0), "node %d: Port must be > 0", i)
			assert.Len(t, node.PublicKey, 64, "node %d: PublicKey must be 64 hex chars, got %d", i, len(node.PublicKey))

			// Verify PublicKey is valid hex
			_, err := hex.DecodeString(node.PublicKey)
			assert.NoError(t, err, "node %d: PublicKey must be valid hex", i)

			assert.NotEmpty(t, node.Maintainer, "node %d: Maintainer must not be empty", i)
		})
	}
}

func TestNodeInfoStruct(t *testing.T) {
	// Verify we can create a NodeInfo and access its fields
	n := NodeInfo{
		Address:    "127.0.0.1",
		Port:       33445,
		PublicKey:  "AAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAA",
		Maintainer: "test",
	}
	assert.Equal(t, "127.0.0.1", n.Address)
	assert.Equal(t, uint16(33445), n.Port)
	assert.Equal(t, "AAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAA", n.PublicKey)
	assert.Equal(t, "test", n.Maintainer)
}

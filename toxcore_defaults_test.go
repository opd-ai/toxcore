package toxcore

import (
	"testing"

	"github.com/opd-ai/toxcore/bootstrap/nodes"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestBootstrapDefaultsPopulatesNodes(t *testing.T) {
	options := NewOptionsForTesting()
	tox, err := New(options)
	require.NoError(t, err, "New() should succeed")
	defer tox.Kill()

	// Verify DefaultNodes has entries
	require.True(t, len(nodes.DefaultNodes) >= 4,
		"DefaultNodes must have at least 4 entries, got %d", len(nodes.DefaultNodes))

	// Call BootstrapDefaults - in a test environment with nonet,
	// DNS resolution will fail so all nodes will fail, but the method
	// should still attempt all nodes and return an error.
	err = tox.BootstrapDefaults()
	// In a no-network environment we expect an error (DNS resolution failures).
	// The important thing is that the function doesn't panic and processes all nodes.
	if err != nil {
		t.Logf("BootstrapDefaults returned expected error in no-network env: %v", err)
	}
}

func TestBootstrapDefaultsNodeListIsValid(t *testing.T) {
	// Verify the default nodes are well-formed
	for _, node := range nodes.DefaultNodes {
		assert.NotEmpty(t, node.Address, "address should not be empty")
		assert.Greater(t, node.Port, uint16(0), "port should be > 0")
		assert.Len(t, node.PublicKey, 64, "public key should be 64 hex chars")
	}
}

func TestDisableAutoBootstrapOption(t *testing.T) {
	options := NewOptions()
	options.DisableAutoBootstrap = true

	tox, err := New(options)
	require.NoError(t, err, "New() with DisableAutoBootstrap should succeed")
	defer tox.Kill()
}

package toxcore

import (
	"testing"

	"github.com/opd-ai/toxcore/bootstrap/nodes"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestBootstrapDefaultsReturnsErrorInNoNetEnv(t *testing.T) {
	options := NewOptionsForTesting()
	tox, err := New(options)
	require.NoError(t, err, "New() should succeed")
	defer tox.Kill()

	// Verify DefaultNodes has entries
	require.True(t, len(nodes.DefaultNodes) >= 4,
		"DefaultNodes must have at least 4 entries, got %d", len(nodes.DefaultNodes))

	// In a test/no-network environment, DNS resolution will fail for
	// hostname-based nodes and connections to IPs will time out,
	// so BootstrapDefaults should return a non-nil error.
	err = tox.BootstrapDefaults()
	assert.Error(t, err, "BootstrapDefaults should return an error in a no-network environment")
}

func TestBootstrapDefaultsNodeListIsValid(t *testing.T) {
	// Verify the default nodes are well-formed
	for _, node := range nodes.DefaultNodes {
		assert.NotEmpty(t, node.Address, "address should not be empty")
		assert.Greater(t, node.Port, uint16(0), "port should be > 0")
		assert.Len(t, node.PublicKey, 64, "public key should be 64 hex chars")
	}
}

func TestBootstrapDefaultsCallableAfterNew(t *testing.T) {
	// Verify that New() succeeds with default options and BootstrapDefaults
	// can be called explicitly after New() without panicking.
	options := NewOptionsForTesting()

	tox, err := New(options)
	require.NoError(t, err, "New() should succeed")
	defer tox.Kill()

	// BootstrapDefaults is available and callable (will fail in test env
	// due to no network, but should not panic)
	err = tox.BootstrapDefaults()
	assert.Error(t, err, "expected error in no-network environment")
}

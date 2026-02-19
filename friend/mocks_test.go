package friend

import (
	"time"

	"github.com/opd-ai/toxcore/crypto"
)

// mockTimeProvider is a mock implementation of TimeProvider for testing.
type mockTimeProvider struct {
	fixedTime time.Time
}

func (m *mockTimeProvider) Now() time.Time {
	return m.fixedTime
}

// generateTestKeyPair generates a keypair for testing using the crypto package.
func generateTestKeyPair() (*crypto.KeyPair, error) {
	return crypto.GenerateKeyPair()
}

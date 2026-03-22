package async

import (
	"testing"

	"github.com/opd-ai/toxcore/crypto"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestDefaultDynamicLimitConfig(t *testing.T) {
	config := DefaultDynamicLimitConfig()

	assert.Equal(t, 100, config.BaseLimit, "base limit should be 100")
	assert.Equal(t, 1000, config.MaxLimit, "max limit should be 1000")
	assert.Equal(t, 100, config.CapacityDivisor, "capacity divisor should be 100")
}

func TestCalculateDynamicRecipientLimit(t *testing.T) {
	tests := []struct {
		name        string
		maxCapacity int
		config      *DynamicLimitConfig
		expected    int
	}{
		{
			name:        "nil config uses defaults",
			maxCapacity: 10000,
			config:      nil,
			expected:    100, // 10000 / 100 = 100
		},
		{
			name:        "small capacity returns base limit",
			maxCapacity: 1000,
			config:      DefaultDynamicLimitConfig(),
			expected:    100, // 1000 / 100 = 10, clamped to base 100
		},
		{
			name:        "medium capacity calculates correctly",
			maxCapacity: 50000,
			config:      DefaultDynamicLimitConfig(),
			expected:    500, // 50000 / 100 = 500
		},
		{
			name:        "large capacity returns max limit",
			maxCapacity: 200000,
			config:      DefaultDynamicLimitConfig(),
			expected:    1000, // 200000 / 100 = 2000, clamped to max 1000
		},
		{
			name:        "custom config with different divisor",
			maxCapacity: 10000,
			config: &DynamicLimitConfig{
				BaseLimit:       50,
				MaxLimit:        500,
				CapacityDivisor: 50,
			},
			expected: 200, // 10000 / 50 = 200
		},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			result := CalculateDynamicRecipientLimit(tc.maxCapacity, tc.config)
			assert.Equal(t, tc.expected, result)
		})
	}
}

func TestMessageStorage_DynamicLimitsEnabled(t *testing.T) {
	keyPair, err := crypto.GenerateKeyPair()
	require.NoError(t, err)

	storage := NewMessageStorage(keyPair, "")

	// Should be enabled by default
	assert.True(t, storage.IsDynamicLimitsEnabled())

	// Can be disabled
	storage.SetDynamicLimitsEnabled(false)
	assert.False(t, storage.IsDynamicLimitsEnabled())

	// Can be re-enabled
	storage.SetDynamicLimitsEnabled(true)
	assert.True(t, storage.IsDynamicLimitsEnabled())
}

func TestMessageStorage_GetMaxMessagesPerRecipient(t *testing.T) {
	keyPair, err := crypto.GenerateKeyPair()
	require.NoError(t, err)

	storage := NewMessageStorage(keyPair, "")

	// Get the dynamic limit
	limit := storage.GetMaxMessagesPerRecipient()
	assert.Greater(t, limit, 0, "limit should be positive")

	// When disabled, should return static constant
	storage.SetDynamicLimitsEnabled(false)
	limit = storage.GetMaxMessagesPerRecipient()
	assert.Equal(t, MaxMessagesPerRecipient, limit)
}

func TestMessageStorage_GetStorageStats_IncludesDynamicLimitInfo(t *testing.T) {
	keyPair, err := crypto.GenerateKeyPair()
	require.NoError(t, err)

	storage := NewMessageStorage(keyPair, "")

	stats := storage.GetStorageStats()

	// Should include dynamic limit fields
	assert.Greater(t, stats.MaxPerRecipient, 0)
	assert.True(t, stats.DynamicLimitsEnabled)
	assert.GreaterOrEqual(t, stats.UtilizationPercent, 0.0)
}

func TestMessageStorage_DynamicLimitEnforcement(t *testing.T) {
	keyPair, err := crypto.GenerateKeyPair()
	require.NoError(t, err)

	storage := NewMessageStorage(keyPair, "")

	// Store a test message
	recipientPK := [32]byte{1, 2, 3}
	senderPK := [32]byte{4, 5, 6}

	// Get the current limit
	limit := storage.GetMaxMessagesPerRecipient()

	// Verify we can store up to the limit
	for i := 0; i < limit; i++ {
		encryptedData := make([]byte, 100)
		_, err := storage.StoreMessage(recipientPK, senderPK, encryptedData, [24]byte{byte(i)}, MessageTypeNormal)
		if err != nil && i < limit-1 {
			// Before limit, no error expected
			t.Fatalf("StoreMessage failed at message %d (limit %d): %v", i, limit, err)
		}
	}

	// The next message should fail
	encryptedData := make([]byte, 100)
	_, err = storage.StoreMessage(recipientPK, senderPK, encryptedData, [24]byte{}, MessageTypeNormal)
	assert.Error(t, err)
	assert.Contains(t, err.Error(), "too many messages for recipient")
}

func BenchmarkCalculateDynamicRecipientLimit(b *testing.B) {
	config := DefaultDynamicLimitConfig()

	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		CalculateDynamicRecipientLimit(10000, config)
	}
}

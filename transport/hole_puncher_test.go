package transport

import (
	"context"
	"net"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
)

func TestNewHolePuncher(t *testing.T) {
	localAddr := &net.UDPAddr{IP: net.IPv4(127, 0, 0, 1), Port: 0}

	hp, err := NewHolePuncher(localAddr)

	assert.NoError(t, err)
	assert.NotNil(t, hp)
	assert.NotNil(t, hp.conn)
	assert.Equal(t, 5*time.Second, hp.timeout)
	assert.Equal(t, 5, hp.maxAttempts)

	// Clean up
	hp.Close()
}

func TestNewHolePuncher_NilLocalAddr(t *testing.T) {
	hp, err := NewHolePuncher(nil)

	assert.Error(t, err)
	assert.Nil(t, hp)
	assert.Contains(t, err.Error(), "local address cannot be nil")
}

func TestHolePuncher_SetTimeout(t *testing.T) {
	localAddr := &net.UDPAddr{IP: net.IPv4(127, 0, 0, 1), Port: 0}
	hp, err := NewHolePuncher(localAddr)
	assert.NoError(t, err)
	defer hp.Close()

	newTimeout := 10 * time.Second
	hp.SetTimeout(newTimeout)

	assert.Equal(t, newTimeout, hp.timeout)
}

func TestHolePuncher_SetMaxAttempts(t *testing.T) {
	localAddr := &net.UDPAddr{IP: net.IPv4(127, 0, 0, 1), Port: 0}
	hp, err := NewHolePuncher(localAddr)
	assert.NoError(t, err)
	defer hp.Close()

	hp.SetMaxAttempts(10)
	assert.Equal(t, 10, hp.maxAttempts)

	// Test invalid value (should be ignored)
	hp.SetMaxAttempts(0)
	assert.Equal(t, 10, hp.maxAttempts)

	hp.SetMaxAttempts(-1)
	assert.Equal(t, 10, hp.maxAttempts)
}

func TestHolePuncher_GetLocalAddr(t *testing.T) {
	localAddr := &net.UDPAddr{IP: net.IPv4(127, 0, 0, 1), Port: 12345}
	hp, err := NewHolePuncher(localAddr)
	assert.NoError(t, err)
	defer hp.Close()

	returnedAddr := hp.GetLocalAddr()
	assert.Equal(t, localAddr.IP, returnedAddr.IP)
	// Port might be different if the system assigned a different one
}

func TestHolePuncher_PunchHole_NilRemoteAddr(t *testing.T) {
	localAddr := &net.UDPAddr{IP: net.IPv4(127, 0, 0, 1), Port: 0}
	hp, err := NewHolePuncher(localAddr)
	assert.NoError(t, err)
	defer hp.Close()

	ctx := context.Background()
	attempt, err := hp.PunchHole(ctx, nil)

	assert.Error(t, err)
	assert.Nil(t, attempt)
	assert.Contains(t, err.Error(), "remote address cannot be nil")
}

func TestHolePuncher_PunchHole_ContextCancellation(t *testing.T) {
	localAddr := &net.UDPAddr{IP: net.IPv4(127, 0, 0, 1), Port: 0}
	hp, err := NewHolePuncher(localAddr)
	assert.NoError(t, err)
	defer hp.Close()

	// Set low max attempts and short timeout for quick test
	hp.SetMaxAttempts(1)
	hp.SetTimeout(100 * time.Millisecond)

	ctx, cancel := context.WithCancel(context.Background())
	cancel() // Immediately cancel context

	remoteAddr := &net.UDPAddr{IP: net.IPv4(192, 0, 2, 1), Port: 12345}
	attempt, err := hp.PunchHole(ctx, remoteAddr)

	assert.Error(t, err)
	assert.Equal(t, context.Canceled, err)
	assert.NotNil(t, attempt)
	assert.Equal(t, HolePunchFailedTimeout, attempt.Result)
}

func TestHolePuncher_PunchHole_Timeout(t *testing.T) {
	localAddr := &net.UDPAddr{IP: net.IPv4(127, 0, 0, 1), Port: 0}
	hp, err := NewHolePuncher(localAddr)
	assert.NoError(t, err)
	defer hp.Close()

	// Set very low max attempts for quick test
	hp.SetMaxAttempts(1)
	hp.SetTimeout(100 * time.Millisecond)

	ctx, cancel := context.WithTimeout(context.Background(), 200*time.Millisecond)
	defer cancel()

	// Use an unreachable address (RFC 5737 test address)
	remoteAddr := &net.UDPAddr{IP: net.IPv4(192, 0, 2, 1), Port: 12345}
	attempt, err := hp.PunchHole(ctx, remoteAddr)

	assert.Error(t, err)
	assert.NotNil(t, attempt)
	assert.Equal(t, HolePunchFailedUnknown, attempt.Result)
	assert.Equal(t, 1, attempt.Attempts)
	assert.Equal(t, remoteAddr, attempt.RemoteAddr)
}

func TestHolePuncher_isValidResponse(t *testing.T) {
	localAddr := &net.UDPAddr{IP: net.IPv4(127, 0, 0, 1), Port: 0}
	hp, err := NewHolePuncher(localAddr)
	assert.NoError(t, err)
	defer hp.Close()

	tests := []struct {
		name     string
		packet   []byte
		expected bool
	}{
		{
			name:     "valid PUNCH_RESPONSE",
			packet:   []byte("PUNCH_RESPONSE"),
			expected: true,
		},
		{
			name:     "valid PONG",
			packet:   []byte("PONG"),
			expected: true,
		},
		{
			name:     "valid ACK",
			packet:   []byte("ACK"),
			expected: true,
		},
		{
			name:     "invalid response",
			packet:   []byte("INVALID"),
			expected: false,
		},
		{
			name:     "empty packet",
			packet:   []byte(""),
			expected: false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := hp.isValidResponse(tt.packet)
			assert.Equal(t, tt.expected, result)
		})
	}
}

func TestHolePuncher_GetHolePunchResult(t *testing.T) {
	localAddr := &net.UDPAddr{IP: net.IPv4(127, 0, 0, 1), Port: 0}
	hp, err := NewHolePuncher(localAddr)
	assert.NoError(t, err)
	defer hp.Close()

	remoteAddr := &net.UDPAddr{IP: net.IPv4(192, 0, 2, 1), Port: 12345}

	// Initially should not exist
	result, exists := hp.GetHolePunchResult(remoteAddr)
	assert.False(t, exists)

	// Add a result manually
	hp.punchResults[remoteAddr.String()] = HolePunchSuccess

	result, exists = hp.GetHolePunchResult(remoteAddr)
	assert.True(t, exists)
	assert.Equal(t, HolePunchSuccess, result)
}

func TestHolePuncher_ClearResults(t *testing.T) {
	localAddr := &net.UDPAddr{IP: net.IPv4(127, 0, 0, 1), Port: 0}
	hp, err := NewHolePuncher(localAddr)
	assert.NoError(t, err)
	defer hp.Close()

	// Add some results
	remoteAddr1 := &net.UDPAddr{IP: net.IPv4(192, 0, 2, 1), Port: 12345}
	remoteAddr2 := &net.UDPAddr{IP: net.IPv4(192, 0, 2, 2), Port: 12346}

	hp.punchResults[remoteAddr1.String()] = HolePunchSuccess
	hp.punchResults[remoteAddr2.String()] = HolePunchFailedTimeout

	// Verify results exist
	_, exists1 := hp.GetHolePunchResult(remoteAddr1)
	_, exists2 := hp.GetHolePunchResult(remoteAddr2)
	assert.True(t, exists1)
	assert.True(t, exists2)

	// Clear results
	hp.ClearResults()

	// Verify results are cleared
	_, exists1 = hp.GetHolePunchResult(remoteAddr1)
	_, exists2 = hp.GetHolePunchResult(remoteAddr2)
	assert.False(t, exists1)
	assert.False(t, exists2)
}

func TestHolePuncher_GetStatistics(t *testing.T) {
	localAddr := &net.UDPAddr{IP: net.IPv4(127, 0, 0, 1), Port: 0}
	hp, err := NewHolePuncher(localAddr)
	assert.NoError(t, err)
	defer hp.Close()

	// Add some results
	hp.punchResults["addr1"] = HolePunchSuccess
	hp.punchResults["addr2"] = HolePunchSuccess
	hp.punchResults["addr3"] = HolePunchFailedTimeout
	hp.punchResults["addr4"] = HolePunchFailedUnknown

	stats := hp.GetStatistics()

	assert.Equal(t, 2, stats[HolePunchSuccess])
	assert.Equal(t, 1, stats[HolePunchFailedTimeout])
	assert.Equal(t, 1, stats[HolePunchFailedUnknown])
	assert.Equal(t, 0, stats[HolePunchFailedRejected]) // Not present
}

func TestHolePuncher_TestConnectivity_NilRemoteAddr(t *testing.T) {
	localAddr := &net.UDPAddr{IP: net.IPv4(127, 0, 0, 1), Port: 0}
	hp, err := NewHolePuncher(localAddr)
	assert.NoError(t, err)
	defer hp.Close()

	ctx := context.Background()
	err = hp.TestConnectivity(ctx, nil)

	assert.Error(t, err)
	assert.Contains(t, err.Error(), "remote address cannot be nil")
}

func TestHolePuncher_SimultaneousPunch_NilRemoteAddr(t *testing.T) {
	localAddr := &net.UDPAddr{IP: net.IPv4(127, 0, 0, 1), Port: 0}
	hp, err := NewHolePuncher(localAddr)
	assert.NoError(t, err)
	defer hp.Close()

	ctx := context.Background()
	startTime := time.Now().Add(time.Second)

	attempt, err := hp.SimultaneousPunch(ctx, nil, startTime)

	assert.Error(t, err)
	assert.Nil(t, attempt)
	assert.Contains(t, err.Error(), "remote address cannot be nil")
}

func TestHolePuncher_SimultaneousPunch_ContextCancellation(t *testing.T) {
	localAddr := &net.UDPAddr{IP: net.IPv4(127, 0, 0, 1), Port: 0}
	hp, err := NewHolePuncher(localAddr)
	assert.NoError(t, err)
	defer hp.Close()

	ctx, cancel := context.WithCancel(context.Background())
	cancel() // Immediately cancel

	remoteAddr := &net.UDPAddr{IP: net.IPv4(192, 0, 2, 1), Port: 12345}
	startTime := time.Now().Add(time.Second) // Future start time

	attempt, err := hp.SimultaneousPunch(ctx, remoteAddr, startTime)

	assert.Error(t, err)
	assert.Equal(t, context.Canceled, err)
	assert.Nil(t, attempt)
}

func TestHolePuncher_Close(t *testing.T) {
	localAddr := &net.UDPAddr{IP: net.IPv4(127, 0, 0, 1), Port: 0}
	hp, err := NewHolePuncher(localAddr)
	assert.NoError(t, err)

	// Close should succeed
	err = hp.Close()
	assert.NoError(t, err)

	// Second close should also succeed (connection is nil)
	hp.conn = nil
	err = hp.Close()
	assert.NoError(t, err)
}

// Integration test for actual hole punching between two local addresses
func TestHolePuncher_Integration_LocalPunch(t *testing.T) {
	if testing.Short() {
		t.Skip("Skipping integration test in short mode")
	}

	// Create two hole punchers on different ports
	localAddr1 := &net.UDPAddr{IP: net.IPv4(127, 0, 0, 1), Port: 0}
	localAddr2 := &net.UDPAddr{IP: net.IPv4(127, 0, 0, 1), Port: 0}

	hp1, err := NewHolePuncher(localAddr1)
	assert.NoError(t, err)
	defer hp1.Close()

	hp2, err := NewHolePuncher(localAddr2)
	assert.NoError(t, err)
	defer hp2.Close()

	// Set low timeouts for quick test
	hp1.SetTimeout(500 * time.Millisecond)
	hp1.SetMaxAttempts(2)

	ctx, cancel := context.WithTimeout(context.Background(), 2*time.Second)
	defer cancel()

	// Try to punch hole from hp1 to hp2
	actualAddr2 := hp2.GetLocalAddr()
	attempt, err := hp1.PunchHole(ctx, actualAddr2)

	// This will likely fail since there's no coordinated response,
	// but we can verify the attempt was made
	assert.NotNil(t, attempt)
	assert.Equal(t, actualAddr2, attempt.RemoteAddr)
	assert.True(t, attempt.Attempts > 0)

	if err != nil {
		t.Logf("Hole punch failed as expected (no coordination): %v", err)
	} else {
		t.Logf("Hole punch succeeded: %v", attempt)
	}
}

// Benchmark hole punch packet sending
func BenchmarkHolePuncher_sendHolePunchPacket(b *testing.B) {
	localAddr := &net.UDPAddr{IP: net.IPv4(127, 0, 0, 1), Port: 0}
	hp, err := NewHolePuncher(localAddr)
	if err != nil {
		b.Fatal(err)
	}
	defer hp.Close()

	remoteAddr := &net.UDPAddr{IP: net.IPv4(127, 0, 0, 1), Port: 12345}

	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		_ = hp.sendHolePunchPacket(remoteAddr)
	}
}

// Benchmark response validation
func BenchmarkHolePuncher_isValidResponse(b *testing.B) {
	localAddr := &net.UDPAddr{IP: net.IPv4(127, 0, 0, 1), Port: 0}
	hp, err := NewHolePuncher(localAddr)
	if err != nil {
		b.Fatal(err)
	}
	defer hp.Close()

	packet := []byte("PUNCH_RESPONSE")

	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		_ = hp.isValidResponse(packet)
	}
}

package transport

import (
	"context"
	"net"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
)

func TestNewAdvancedNATTraversal(t *testing.T) {
	localAddr := &net.UDPAddr{IP: net.IPv4(127, 0, 0, 1), Port: 0}

	ant, err := NewAdvancedNATTraversal(localAddr)

	assert.NoError(t, err)
	assert.NotNil(t, ant)
	assert.NotNil(t, ant.ipResolver)
	assert.NotNil(t, ant.holePuncher)
	assert.NotNil(t, ant.natTraversal)
	assert.Equal(t, 30*time.Second, ant.timeout)

	// Check that methods are enabled by default (except relay)
	assert.True(t, ant.isMethodEnabled(ConnectionDirect))
	assert.True(t, ant.isMethodEnabled(ConnectionUPnP))
	assert.True(t, ant.isMethodEnabled(ConnectionSTUN))
	assert.True(t, ant.isMethodEnabled(ConnectionHolePunch))
	assert.False(t, ant.isMethodEnabled(ConnectionRelay))

	// Clean up
	ant.Close()
}

func TestNewAdvancedNATTraversal_NilLocalAddr(t *testing.T) {
	ant, err := NewAdvancedNATTraversal(nil)

	assert.Error(t, err)
	assert.Nil(t, ant)
	assert.Contains(t, err.Error(), "local address cannot be nil")
}

func TestNewAdvancedNATTraversal_UnsupportedAddrType(t *testing.T) {
	localAddr := &net.IPAddr{IP: net.IPv4(127, 0, 0, 1)}

	ant, err := NewAdvancedNATTraversal(localAddr)

	assert.Error(t, err)
	assert.Nil(t, ant)
	assert.Contains(t, err.Error(), "unsupported local address type")
}

func TestAdvancedNATTraversal_EnableMethod(t *testing.T) {
	localAddr := &net.UDPAddr{IP: net.IPv4(127, 0, 0, 1), Port: 0}
	ant, err := NewAdvancedNATTraversal(localAddr)
	assert.NoError(t, err)
	defer ant.Close()

	// Test disabling and enabling methods
	ant.EnableMethod(ConnectionDirect, false)
	assert.False(t, ant.isMethodEnabled(ConnectionDirect))

	ant.EnableMethod(ConnectionDirect, true)
	assert.True(t, ant.isMethodEnabled(ConnectionDirect))

	ant.EnableMethod(ConnectionRelay, true)
	assert.True(t, ant.isMethodEnabled(ConnectionRelay))
}

func TestAdvancedNATTraversal_SetTimeout(t *testing.T) {
	localAddr := &net.UDPAddr{IP: net.IPv4(127, 0, 0, 1), Port: 0}
	ant, err := NewAdvancedNATTraversal(localAddr)
	assert.NoError(t, err)
	defer ant.Close()

	newTimeout := 10 * time.Second
	ant.SetTimeout(newTimeout)

	assert.Equal(t, newTimeout, ant.timeout)
}

func TestAdvancedNATTraversal_EstablishConnection_NilRemoteAddr(t *testing.T) {
	localAddr := &net.UDPAddr{IP: net.IPv4(127, 0, 0, 1), Port: 0}
	ant, err := NewAdvancedNATTraversal(localAddr)
	assert.NoError(t, err)
	defer ant.Close()

	ctx := context.Background()
	attempt, err := ant.EstablishConnection(ctx, nil)

	assert.Error(t, err)
	assert.Nil(t, attempt)
	assert.Contains(t, err.Error(), "remote address cannot be nil")
}

func TestAdvancedNATTraversal_EstablishConnection_NoMethodsEnabled(t *testing.T) {
	localAddr := &net.UDPAddr{IP: net.IPv4(127, 0, 0, 1), Port: 0}
	ant, err := NewAdvancedNATTraversal(localAddr)
	assert.NoError(t, err)
	defer ant.Close()

	// Disable all methods
	ant.EnableMethod(ConnectionDirect, false)
	ant.EnableMethod(ConnectionUPnP, false)
	ant.EnableMethod(ConnectionSTUN, false)
	ant.EnableMethod(ConnectionHolePunch, false)
	ant.EnableMethod(ConnectionRelay, false)

	ctx := context.Background()
	remoteAddr := &net.UDPAddr{IP: net.IPv4(192, 0, 2, 1), Port: 12345}

	attempt, err := ant.EstablishConnection(ctx, remoteAddr)

	assert.Error(t, err)
	assert.Nil(t, attempt)
	assert.Contains(t, err.Error(), "no connection methods available")
}

func TestAdvancedNATTraversal_EstablishConnection_ContextCancellation(t *testing.T) {
	localAddr := &net.UDPAddr{IP: net.IPv4(127, 0, 0, 1), Port: 0}
	ant, err := NewAdvancedNATTraversal(localAddr)
	assert.NoError(t, err)
	defer ant.Close()

	ctx, cancel := context.WithCancel(context.Background())
	cancel() // Immediately cancel

	remoteAddr := &net.UDPAddr{IP: net.IPv4(192, 0, 2, 1), Port: 12345}
	_, err = ant.EstablishConnection(ctx, remoteAddr)

	assert.Error(t, err)
	assert.Equal(t, context.Canceled, err)
}

func TestAdvancedNATTraversal_attemptDirectConnection(t *testing.T) {
	localAddr := &net.UDPAddr{IP: net.IPv4(127, 0, 0, 1), Port: 0}
	ant, err := NewAdvancedNATTraversal(localAddr)
	assert.NoError(t, err)
	defer ant.Close()

	ctx := context.Background()
	remoteAddr := &net.UDPAddr{IP: net.IPv4(192, 0, 2, 1), Port: 12345}

	// This will likely fail since we don't have a real public IP
	err = ant.attemptDirectConnection(ctx, remoteAddr)

	// We expect this to fail in test environment
	assert.Error(t, err)
}

func TestAdvancedNATTraversal_attemptUPnPConnection(t *testing.T) {
	localAddr := &net.UDPAddr{IP: net.IPv4(127, 0, 0, 1), Port: 0}
	ant, err := NewAdvancedNATTraversal(localAddr)
	assert.NoError(t, err)
	defer ant.Close()

	ctx := context.Background()
	remoteAddr := &net.UDPAddr{IP: net.IPv4(192, 0, 2, 1), Port: 12345}

	// This will likely fail since UPnP is not available in test environment
	err = ant.attemptUPnPConnection(ctx, remoteAddr)

	// We expect this to fail in test environment
	assert.Error(t, err)
	assert.Contains(t, err.Error(), "UPnP not available")
}

func TestAdvancedNATTraversal_attemptHolePunchConnection_NonUDP(t *testing.T) {
	localAddr := &net.UDPAddr{IP: net.IPv4(127, 0, 0, 1), Port: 0}
	ant, err := NewAdvancedNATTraversal(localAddr)
	assert.NoError(t, err)
	defer ant.Close()

	ctx := context.Background()
	remoteAddr := &net.TCPAddr{IP: net.IPv4(192, 0, 2, 1), Port: 12345}

	err = ant.attemptHolePunchConnection(ctx, remoteAddr)

	assert.Error(t, err)
	assert.Contains(t, err.Error(), "hole punching requires UDP address")
}

func TestAdvancedNATTraversal_attemptRelayConnection(t *testing.T) {
	localAddr := &net.UDPAddr{IP: net.IPv4(127, 0, 0, 1), Port: 0}
	ant, err := NewAdvancedNATTraversal(localAddr)
	assert.NoError(t, err)
	defer ant.Close()

	ctx := context.Background()
	remoteAddr := &net.UDPAddr{IP: net.IPv4(192, 0, 2, 1), Port: 12345}

	err = ant.attemptRelayConnection(ctx, remoteAddr)

	assert.Error(t, err)
	assert.Contains(t, err.Error(), "no relay servers configured")
}

func TestAdvancedNATTraversal_extractIP(t *testing.T) {
	localAddr := &net.UDPAddr{IP: net.IPv4(127, 0, 0, 1), Port: 0}
	ant, err := NewAdvancedNATTraversal(localAddr)
	assert.NoError(t, err)
	defer ant.Close()

	tests := []struct {
		name     string
		addr     net.Addr
		expected net.IP
	}{
		{
			name:     "UDP address",
			addr:     &net.UDPAddr{IP: net.IPv4(192, 168, 1, 1), Port: 8080},
			expected: net.IPv4(192, 168, 1, 1),
		},
		{
			name:     "TCP address",
			addr:     &net.TCPAddr{IP: net.IPv4(10, 0, 0, 1), Port: 8080},
			expected: net.IPv4(10, 0, 0, 1),
		},
		{
			name:     "IP address",
			addr:     &net.IPAddr{IP: net.IPv4(172, 16, 0, 1)},
			expected: net.IPv4(172, 16, 0, 1),
		},
		{
			name:     "unsupported type",
			addr:     &net.UnixAddr{Name: "/tmp/test.sock"},
			expected: nil,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := ant.extractIP(tt.addr)
			if tt.expected == nil {
				assert.Nil(t, result)
			} else {
				assert.True(t, tt.expected.Equal(result))
			}
		})
	}
}

func TestAdvancedNATTraversal_isDirectlyReachable(t *testing.T) {
	localAddr := &net.UDPAddr{IP: net.IPv4(127, 0, 0, 1), Port: 0}
	ant, err := NewAdvancedNATTraversal(localAddr)
	assert.NoError(t, err)
	defer ant.Close()

	tests := []struct {
		name       string
		localAddr  net.Addr
		remoteAddr net.Addr
		expected   bool
	}{
		{
			name:       "both public IPs",
			localAddr:  &net.UDPAddr{IP: net.IPv4(8, 8, 8, 8), Port: 8080},
			remoteAddr: &net.UDPAddr{IP: net.IPv4(1, 1, 1, 1), Port: 8080},
			expected:   true,
		},
		{
			name:       "local private, remote public",
			localAddr:  &net.UDPAddr{IP: net.IPv4(192, 168, 1, 1), Port: 8080},
			remoteAddr: &net.UDPAddr{IP: net.IPv4(8, 8, 8, 8), Port: 8080},
			expected:   false,
		},
		{
			name:       "both private IPs",
			localAddr:  &net.UDPAddr{IP: net.IPv4(192, 168, 1, 1), Port: 8080},
			remoteAddr: &net.UDPAddr{IP: net.IPv4(10, 0, 0, 1), Port: 8080},
			expected:   false,
		},
		{
			name:       "nil IPs",
			localAddr:  &net.UnixAddr{Name: "/tmp/test.sock"},
			remoteAddr: &net.UDPAddr{IP: net.IPv4(8, 8, 8, 8), Port: 8080},
			expected:   false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := ant.isDirectlyReachable(tt.localAddr, tt.remoteAddr)
			assert.Equal(t, tt.expected, result)
		})
	}
}

func TestAdvancedNATTraversal_GetAttemptHistory(t *testing.T) {
	localAddr := &net.UDPAddr{IP: net.IPv4(127, 0, 0, 1), Port: 0}
	ant, err := NewAdvancedNATTraversal(localAddr)
	assert.NoError(t, err)
	defer ant.Close()

	// Initially should be empty
	history := ant.GetAttemptHistory()
	assert.Len(t, history, 0)

	// Add some attempts manually
	attempt1 := &ConnectionAttempt{
		Method:    ConnectionDirect,
		Success:   true,
		Timestamp: time.Now(),
	}
	attempt2 := &ConnectionAttempt{
		Method:    ConnectionUPnP,
		Success:   false,
		Timestamp: time.Now(),
	}

	ant.recordAttempt(attempt1)
	ant.recordAttempt(attempt2)

	history = ant.GetAttemptHistory()
	assert.Len(t, history, 2)
	assert.Equal(t, ConnectionDirect, history[0].Method)
	assert.Equal(t, ConnectionUPnP, history[1].Method)
	assert.True(t, history[0].Success)
	assert.False(t, history[1].Success)
}

func TestAdvancedNATTraversal_GetMethodStatistics(t *testing.T) {
	localAddr := &net.UDPAddr{IP: net.IPv4(127, 0, 0, 1), Port: 0}
	ant, err := NewAdvancedNATTraversal(localAddr)
	assert.NoError(t, err)
	defer ant.Close()

	// Add some test attempts
	attempts := []*ConnectionAttempt{
		{Method: ConnectionDirect, Success: true},
		{Method: ConnectionDirect, Success: false},
		{Method: ConnectionDirect, Success: true},
		{Method: ConnectionUPnP, Success: false},
		{Method: ConnectionUPnP, Success: false},
	}

	for _, attempt := range attempts {
		ant.recordAttempt(attempt)
	}

	stats := ant.GetMethodStatistics()

	directStats := stats[ConnectionDirect]
	assert.Equal(t, 3, directStats.Attempts)
	assert.Equal(t, 2, directStats.Successes)
	assert.InDelta(t, 66.67, directStats.SuccessRate, 0.01)

	upnpStats := stats[ConnectionUPnP]
	assert.Equal(t, 2, upnpStats.Attempts)
	assert.Equal(t, 0, upnpStats.Successes)
	assert.Equal(t, 0.0, upnpStats.SuccessRate)

	// Method with no attempts should not be in stats
	_, exists := stats[ConnectionSTUN]
	assert.False(t, exists)
}

// Integration test that tries all connection methods
func TestAdvancedNATTraversal_Integration_EstablishConnection(t *testing.T) {
	if testing.Short() {
		t.Skip("Skipping integration test in short mode")
	}

	localAddr := &net.UDPAddr{IP: net.IPv4(127, 0, 0, 1), Port: 0}
	ant, err := NewAdvancedNATTraversal(localAddr)
	assert.NoError(t, err)
	defer ant.Close()

	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()

	remoteAddr := &net.UDPAddr{IP: net.IPv4(8, 8, 8, 8), Port: 53} // Google DNS

	attempt, err := ant.EstablishConnection(ctx, remoteAddr)

	// This will likely fail in most test environments, but we can verify the attempt
	assert.NotNil(t, attempt)
	t.Logf("Connection attempt result: Method=%v, Success=%v, Error=%v",
		attempt.Method, attempt.Success, attempt.Error)

	// Check that attempt was recorded
	history := ant.GetAttemptHistory()
	assert.True(t, len(history) > 0)

	// Check statistics
	stats := ant.GetMethodStatistics()
	assert.True(t, len(stats) > 0)
}

// Benchmark connection attempt recording
func BenchmarkAdvancedNATTraversal_recordAttempt(b *testing.B) {
	localAddr := &net.UDPAddr{IP: net.IPv4(127, 0, 0, 1), Port: 0}
	ant, err := NewAdvancedNATTraversal(localAddr)
	if err != nil {
		b.Fatal(err)
	}
	defer ant.Close()

	attempt := &ConnectionAttempt{
		Method:    ConnectionDirect,
		Success:   true,
		Timestamp: time.Now(),
	}

	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		ant.recordAttempt(attempt)
	}
}

// Benchmark statistics calculation
func BenchmarkAdvancedNATTraversal_GetMethodStatistics(b *testing.B) {
	localAddr := &net.UDPAddr{IP: net.IPv4(127, 0, 0, 1), Port: 0}
	ant, err := NewAdvancedNATTraversal(localAddr)
	if err != nil {
		b.Fatal(err)
	}
	defer ant.Close()

	// Add some test data
	for i := 0; i < 100; i++ {
		attempt := &ConnectionAttempt{
			Method:  ConnectionMethod(i % 5),
			Success: i%3 == 0,
		}
		ant.recordAttempt(attempt)
	}

	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		_ = ant.GetMethodStatistics()
	}
}

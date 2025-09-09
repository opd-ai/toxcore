// Package transport implements network transport for the Tox protocol.
//
// This file implements UDP hole punching for peer-to-peer connectivity
// through NAT devices that don't support UPnP or STUN.
package transport

import (
	"context"
	"errors"
	"fmt"
	"net"
	"sync"
	"time"
)

// HolePuncher manages UDP hole punching operations
type HolePuncher struct {
	localAddr    *net.UDPAddr
	conn         *net.UDPConn
	timeout      time.Duration
	maxAttempts  int
	mu           sync.RWMutex
	punchResults map[string]HolePunchResult
}

// HolePunchAttempt represents a hole punching attempt
type HolePunchAttempt struct {
	RemoteAddr  *net.UDPAddr
	LocalAddr   *net.UDPAddr
	Attempts    int
	LastAttempt time.Time
	Result      HolePunchResult
	RTT         time.Duration
}

// NewHolePuncher creates a new hole puncher instance
func NewHolePuncher(localAddr *net.UDPAddr) (*HolePuncher, error) {
	if localAddr == nil {
		return nil, errors.New("local address cannot be nil")
	}

	conn, err := net.ListenUDP("udp", localAddr)
	if err != nil {
		return nil, fmt.Errorf("failed to create UDP connection: %w", err)
	}

	return &HolePuncher{
		localAddr:    localAddr,
		conn:         conn,
		timeout:      5 * time.Second,
		maxAttempts:  5,
		punchResults: make(map[string]HolePunchResult),
	}, nil
}

// PunchHole attempts to create a hole through NAT to reach a remote peer
func (hp *HolePuncher) PunchHole(ctx context.Context, remoteAddr *net.UDPAddr) (*HolePunchAttempt, error) {
	if remoteAddr == nil {
		return nil, errors.New("remote address cannot be nil")
	}

	hp.mu.Lock()
	defer hp.mu.Unlock()

	attempt := &HolePunchAttempt{
		RemoteAddr: remoteAddr,
		LocalAddr:  hp.localAddr,
		Result:     HolePunchFailedUnknown,
	}

	// Set connection timeout
	deadline, ok := ctx.Deadline()
	if ok {
		hp.conn.SetDeadline(deadline)
	} else {
		hp.conn.SetDeadline(time.Now().Add(hp.timeout))
	}
	defer hp.conn.SetDeadline(time.Time{}) // Reset deadline

	// Perform simultaneous hole punching attempts
	for i := 0; i < hp.maxAttempts; i++ {
		// Check if context was cancelled before starting attempt
		select {
		case <-ctx.Done():
			attempt.Result = HolePunchFailedTimeout
			return attempt, ctx.Err()
		default:
		}

		attempt.Attempts = i + 1
		attempt.LastAttempt = time.Now()

		start := time.Now()

		// Send hole punch packet
		if err := hp.sendHolePunchPacket(remoteAddr); err != nil {
			continue // Try next attempt
		}

		// Wait for response or timeout
		if hp.waitForResponse(ctx, remoteAddr) {
			attempt.RTT = time.Since(start)
			attempt.Result = HolePunchSuccess
			hp.punchResults[remoteAddr.String()] = HolePunchSuccess
			return attempt, nil
		}

		// Brief delay between attempts
		time.Sleep(time.Duration(i+1) * 100 * time.Millisecond)
	}

	attempt.Result = HolePunchFailedUnknown
	hp.punchResults[remoteAddr.String()] = HolePunchFailedUnknown
	return attempt, errors.New("hole punching failed after all attempts")
}

// sendHolePunchPacket sends a UDP packet to the remote address to create a hole
func (hp *HolePuncher) sendHolePunchPacket(remoteAddr *net.UDPAddr) error {
	// Send a simple hole punch packet
	// In a real implementation, this would be coordinated with the remote peer
	// through a relay server or mutual discovery service
	punchPacket := []byte("PUNCH_HOLE")

	_, err := hp.conn.WriteToUDP(punchPacket, remoteAddr)
	if err != nil {
		return fmt.Errorf("failed to send hole punch packet: %w", err)
	}

	return nil
}

// waitForResponse waits for a response from the remote peer
func (hp *HolePuncher) waitForResponse(ctx context.Context, expectedAddr *net.UDPAddr) bool {
	buffer := make([]byte, 1024)
	hp.setReadTimeout()

	for {
		n, remoteAddr, err := hp.conn.ReadFromUDP(buffer)
		if err != nil {
			return hp.handleReadError(err)
		}

		if hp.isValidResponseFromExpectedAddress(remoteAddr, expectedAddr, buffer[:n]) {
			return true
		}

		if hp.isContextCancelled(ctx) {
			return false
		}
	}
}

// setReadTimeout sets a shorter read timeout for individual attempts
func (hp *HolePuncher) setReadTimeout() {
	readDeadline := time.Now().Add(500 * time.Millisecond)
	hp.conn.SetReadDeadline(readDeadline)
}

// handleReadError processes read errors and determines if operation should continue
func (hp *HolePuncher) handleReadError(err error) bool {
	// Check if it's a timeout (expected) or other error
	if netErr, ok := err.(net.Error); ok && netErr.Timeout() {
		return false
	}
	return false
}

// isValidResponseFromExpectedAddress checks if the response came from the expected address and is valid
func (hp *HolePuncher) isValidResponseFromExpectedAddress(remoteAddr, expectedAddr *net.UDPAddr, packet []byte) bool {
	if !hp.isResponseFromExpectedAddress(remoteAddr, expectedAddr) {
		return false
	}
	return hp.isValidResponse(packet)
}

// isResponseFromExpectedAddress verifies the response came from the expected remote address
func (hp *HolePuncher) isResponseFromExpectedAddress(remoteAddr, expectedAddr *net.UDPAddr) bool {
	return remoteAddr.IP.Equal(expectedAddr.IP) && remoteAddr.Port == expectedAddr.Port
}

// isContextCancelled checks if the context has been cancelled
func (hp *HolePuncher) isContextCancelled(ctx context.Context) bool {
	select {
	case <-ctx.Done():
		return true
	default:
		return false
	}
}

// isValidResponse checks if the received packet is a valid hole punch response
func (hp *HolePuncher) isValidResponse(packet []byte) bool {
	// Simple validation - in practice this would be more sophisticated
	expectedResponses := []string{"PUNCH_RESPONSE", "PONG", "ACK"}

	packetStr := string(packet)
	for _, expected := range expectedResponses {
		if packetStr == expected {
			return true
		}
	}

	return false
}

// GetHolePunchResult returns the result of a previous hole punch attempt
func (hp *HolePuncher) GetHolePunchResult(remoteAddr *net.UDPAddr) (HolePunchResult, bool) {
	hp.mu.RLock()
	defer hp.mu.RUnlock()

	result, exists := hp.punchResults[remoteAddr.String()]
	return result, exists
}

// TestConnectivity tests if a hole punch was successful by sending test packets
func (hp *HolePuncher) TestConnectivity(ctx context.Context, remoteAddr *net.UDPAddr) error {
	if remoteAddr == nil {
		return errors.New("remote address cannot be nil")
	}

	// Send test packet
	testPacket := []byte("CONNECTIVITY_TEST")
	_, err := hp.conn.WriteToUDP(testPacket, remoteAddr)
	if err != nil {
		return fmt.Errorf("failed to send test packet: %w", err)
	}

	// Wait for acknowledgment
	buffer := make([]byte, 1024)
	hp.conn.SetReadDeadline(time.Now().Add(2 * time.Second))
	defer hp.conn.SetReadDeadline(time.Time{})

	n, responseAddr, err := hp.conn.ReadFromUDP(buffer)
	if err != nil {
		return fmt.Errorf("no response to connectivity test: %w", err)
	}

	if !responseAddr.IP.Equal(remoteAddr.IP) || responseAddr.Port != remoteAddr.Port {
		return fmt.Errorf("response from unexpected address: %v", responseAddr)
	}

	response := string(buffer[:n])
	if response != "CONNECTIVITY_ACK" {
		return fmt.Errorf("unexpected response: %s", response)
	}

	return nil
}

// SetMaxAttempts sets the maximum number of hole punch attempts
func (hp *HolePuncher) SetMaxAttempts(attempts int) {
	if attempts > 0 {
		hp.maxAttempts = attempts
	}
}

// SetTimeout sets the timeout for hole punching operations
func (hp *HolePuncher) SetTimeout(timeout time.Duration) {
	hp.timeout = timeout
}

// GetLocalAddr returns the local UDP address
func (hp *HolePuncher) GetLocalAddr() *net.UDPAddr {
	return hp.localAddr
}

// Close closes the hole puncher and releases resources
func (hp *HolePuncher) Close() error {
	hp.mu.Lock()
	defer hp.mu.Unlock()

	if hp.conn != nil {
		return hp.conn.Close()
	}
	return nil
}

// ClearResults clears the hole punch results cache
func (hp *HolePuncher) ClearResults() {
	hp.mu.Lock()
	defer hp.mu.Unlock()

	hp.punchResults = make(map[string]HolePunchResult)
}

// GetStatistics returns statistics about hole punching attempts
func (hp *HolePuncher) GetStatistics() map[HolePunchResult]int {
	hp.mu.RLock()
	defer hp.mu.RUnlock()

	stats := make(map[HolePunchResult]int)

	for _, result := range hp.punchResults {
		stats[result]++
	}

	return stats
}

// SimultaneousPunch performs simultaneous hole punching with a remote peer
// This requires coordination through an external relay or signaling service
func (hp *HolePuncher) SimultaneousPunch(ctx context.Context, remoteAddr *net.UDPAddr, startTime time.Time) (*HolePunchAttempt, error) {
	if remoteAddr == nil {
		return nil, errors.New("remote address cannot be nil")
	}

	// Wait until the specified start time for synchronized attempt
	now := time.Now()
	if startTime.After(now) {
		select {
		case <-time.After(startTime.Sub(now)):
		case <-ctx.Done():
			return nil, ctx.Err()
		}
	}

	// Perform the hole punch attempt
	return hp.PunchHole(ctx, remoteAddr)
}

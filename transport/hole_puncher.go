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

	"github.com/sirupsen/logrus"
)

// HolePuncher manages UDP hole punching operations
type HolePuncher struct {
	localAddr    net.Addr
	conn         net.PacketConn
	timeout      time.Duration
	maxAttempts  int
	mu           sync.RWMutex
	punchResults map[string]HolePunchResult
}

// HolePunchAttempt represents a hole punching attempt
type HolePunchAttempt struct {
	RemoteAddr  net.Addr
	LocalAddr   net.Addr
	Attempts    int
	LastAttempt time.Time
	Result      HolePunchResult
	RTT         time.Duration
}

// NewHolePuncher creates a new hole puncher instance
func NewHolePuncher(localAddr net.Addr) (*HolePuncher, error) {
	if localAddr == nil {
		return nil, errors.New("local address cannot be nil")
	}

	conn, err := net.ListenPacket("udp", localAddr.String())
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
func (hp *HolePuncher) PunchHole(ctx context.Context, remoteAddr net.Addr) (*HolePunchAttempt, error) {
	if remoteAddr == nil {
		return nil, errors.New("remote address cannot be nil")
	}

	hp.mu.Lock()
	defer hp.mu.Unlock()

	attempt := hp.initializeAttempt(remoteAddr)

	if err := hp.setupConnectionDeadline(ctx); err != nil {
		return attempt, err
	}
	defer hp.conn.SetDeadline(time.Time{}) // Reset deadline

	success, err := hp.executeAttemptLoop(ctx, remoteAddr, attempt)
	if err != nil {
		return attempt, err
	}
	if success {
		return attempt, nil
	}

	return hp.finalizeFailedAttempt(attempt, remoteAddr)
}

// initializeAttempt creates and returns a new HolePunchAttempt with default values.
func (hp *HolePuncher) initializeAttempt(remoteAddr net.Addr) *HolePunchAttempt {
	return &HolePunchAttempt{
		RemoteAddr: remoteAddr,
		LocalAddr:  hp.localAddr,
		Result:     HolePunchFailedUnknown,
	}
}

// setupConnectionDeadline sets the connection deadline based on context or default timeout.
func (hp *HolePuncher) setupConnectionDeadline(ctx context.Context) error {
	deadline, ok := ctx.Deadline()
	if ok {
		if err := hp.conn.SetDeadline(deadline); err != nil {
			return fmt.Errorf("failed to set connection deadline: %w", err)
		}
	} else {
		if err := hp.conn.SetDeadline(time.Now().Add(hp.timeout)); err != nil {
			return fmt.Errorf("failed to set connection deadline: %w", err)
		}
	}
	return nil
}

// executeAttemptLoop performs the main hole punching attempt loop with retries.
func (hp *HolePuncher) executeAttemptLoop(ctx context.Context, remoteAddr net.Addr, attempt *HolePunchAttempt) (bool, error) {
	for i := 0; i < hp.maxAttempts; i++ {
		if shouldCancelAttempt(ctx) {
			attempt.Result = HolePunchFailedTimeout
			return false, ctx.Err()
		}
		if success := hp.executeHolePunchAttempt(ctx, remoteAddr, attempt, i); success {
			return true, nil
		}
		if err := waitForHolePunchAttempt(ctx, i); err != nil {
			attempt.Result = HolePunchFailedTimeout
			return false, err
		}
	}
	return false, nil
}

// waitForHolePunchAttempt applies the retry backoff between attempts.
func waitForHolePunchAttempt(ctx context.Context, attemptIndex int) error {
	timer := time.NewTimer(time.Duration(attemptIndex+1) * 100 * time.Millisecond)
	defer timer.Stop()
	select {
	case <-timer.C:
		return nil
	case <-ctx.Done():
		return ctx.Err()
	}
}

// shouldCancelAttempt checks if the context has been cancelled.
func shouldCancelAttempt(ctx context.Context) bool {
	select {
	case <-ctx.Done():
		return true
	default:
		return false
	}
}

// executeHolePunchAttempt performs a single hole punch attempt.
func (hp *HolePuncher) executeHolePunchAttempt(ctx context.Context, remoteAddr net.Addr, attempt *HolePunchAttempt, attemptIndex int) bool {
	attempt.Attempts = attemptIndex + 1
	attempt.LastAttempt = time.Now()
	start := time.Now()

	if err := hp.sendHolePunchPacket(remoteAddr); err != nil {
		return false
	}

	if hp.waitForResponse(ctx, remoteAddr) {
		attempt.RTT = time.Since(start)
		attempt.Result = HolePunchSuccess
		hp.punchResults[remoteAddr.String()] = HolePunchSuccess
		return true
	}

	return false
}

// finalizeFailedAttempt sets the final state for a failed hole punch attempt.
func (hp *HolePuncher) finalizeFailedAttempt(attempt *HolePunchAttempt, remoteAddr net.Addr) (*HolePunchAttempt, error) {
	attempt.Result = HolePunchFailedUnknown
	hp.punchResults[remoteAddr.String()] = HolePunchFailedUnknown
	return attempt, errors.New("hole punching failed after all attempts")
}

// sendHolePunchPacket sends a UDP packet to the remote address to create a hole
func (hp *HolePuncher) sendHolePunchPacket(remoteAddr net.Addr) error {
	// Send a simple hole punch packet
	// In a real implementation, this would be coordinated with the remote peer
	// through a relay server or mutual discovery service
	punchPacket := []byte("PUNCH_HOLE")

	_, err := hp.conn.WriteTo(punchPacket, remoteAddr)
	if err != nil {
		return fmt.Errorf("failed to send hole punch packet: %w", err)
	}

	return nil
}

// waitForResponse waits for a response from the remote peer
func (hp *HolePuncher) waitForResponse(ctx context.Context, expectedAddr net.Addr) bool {
	buffer := make([]byte, 1024)
	if err := hp.setReadTimeout(); err != nil {
		// Log error but continue with default timeout behavior
		logrus.WithError(err).Warn("Failed to set read timeout for hole punching")
	}

	for {
		n, remoteAddr, err := hp.conn.ReadFrom(buffer)
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
func (hp *HolePuncher) setReadTimeout() error {
	readDeadline := time.Now().Add(500 * time.Millisecond)
	if err := hp.conn.SetReadDeadline(readDeadline); err != nil {
		return fmt.Errorf("failed to set read deadline: %w", err)
	}
	return nil
}

// handleReadError processes read errors and determines if operation should continue
func (hp *HolePuncher) handleReadError(err error) bool {
	// Check if it's a timeout (expected) or other error
	var netErr net.Error
	if errors.As(err, &netErr) && netErr.Timeout() {
		return false
	}
	return false
}

// isValidResponseFromExpectedAddress checks if the response came from the expected address and is valid
func (hp *HolePuncher) isValidResponseFromExpectedAddress(remoteAddr, expectedAddr net.Addr, packet []byte) bool {
	if !hp.isResponseFromExpectedAddress(remoteAddr, expectedAddr) {
		return false
	}
	return hp.isValidResponse(packet)
}

// isResponseFromExpectedAddress verifies the response came from the expected remote address
func (hp *HolePuncher) isResponseFromExpectedAddress(remoteAddr, expectedAddr net.Addr) bool {
	return remoteAddr.String() == expectedAddr.String()
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
func (hp *HolePuncher) GetHolePunchResult(remoteAddr net.Addr) (HolePunchResult, bool) {
	hp.mu.RLock()
	defer hp.mu.RUnlock()

	result, exists := hp.punchResults[remoteAddr.String()]
	return result, exists
}

// TestConnectivity tests if a hole punch was successful by sending test packets
func (hp *HolePuncher) TestConnectivity(ctx context.Context, remoteAddr net.Addr) error {
	if remoteAddr == nil {
		return errors.New("remote address cannot be nil")
	}
	if err := hp.sendConnectivityTest(remoteAddr); err != nil {
		return err
	}
	n, responseAddr, buffer, err := hp.readConnectivityAck(ctx)
	if err != nil {
		return err
	}
	return validateConnectivityAck(remoteAddr, responseAddr, buffer[:n])
}

// sendConnectivityTest sends the probe packet used to verify the punch path.
func (hp *HolePuncher) sendConnectivityTest(remoteAddr net.Addr) error {
	if _, err := hp.conn.WriteTo([]byte("CONNECTIVITY_TEST"), remoteAddr); err != nil {
		return fmt.Errorf("failed to send test packet: %w", err)
	}
	return nil
}

// readConnectivityAck waits for the connectivity acknowledgement packet.
func (hp *HolePuncher) readConnectivityAck(_ context.Context) (int, net.Addr, []byte, error) {
	buffer := make([]byte, 1024)
	if err := hp.conn.SetReadDeadline(time.Now().Add(2 * time.Second)); err != nil {
		return 0, nil, nil, fmt.Errorf("failed to set read deadline: %w", err)
	}
	defer hp.conn.SetReadDeadline(time.Time{})
	n, responseAddr, err := hp.conn.ReadFrom(buffer)
	if err != nil {
		return 0, nil, nil, fmt.Errorf("no response to connectivity test: %w", err)
	}
	return n, responseAddr, buffer, nil
}

// validateConnectivityAck verifies the responder and response payload.
func validateConnectivityAck(remoteAddr, responseAddr net.Addr, response []byte) error {
	if responseAddr.String() != remoteAddr.String() {
		return fmt.Errorf("response from unexpected address: %v", responseAddr)
	}
	if string(response) != "CONNECTIVITY_ACK" {
		return fmt.Errorf("unexpected response: %s", string(response))
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

// GetLocalAddr returns the local address used for hole punching
func (hp *HolePuncher) GetLocalAddr() net.Addr {
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
func (hp *HolePuncher) SimultaneousPunch(ctx context.Context, remoteAddr net.Addr, startTime time.Time) (*HolePunchAttempt, error) {
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

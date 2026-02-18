package file

import (
	"testing"
	"time"
)

// mockTimeProvider provides deterministic time for testing.
type mockTimeProvider struct {
	currentTime time.Time
}

func (m *mockTimeProvider) Now() time.Time {
	return m.currentTime
}

func (m *mockTimeProvider) Since(t time.Time) time.Duration {
	return m.currentTime.Sub(t)
}

func (m *mockTimeProvider) advance(d time.Duration) {
	m.currentTime = m.currentTime.Add(d)
}

func newMockTimeProvider() *mockTimeProvider {
	return &mockTimeProvider{
		currentTime: time.Date(2026, 1, 1, 0, 0, 0, 0, time.UTC),
	}
}

func TestTransfer_SetStallTimeout(t *testing.T) {
	transfer := NewTransfer(1, 1, "test.txt", 1024, TransferDirectionIncoming)

	// Default timeout should be set
	if transfer.GetStallTimeout() != DefaultStallTimeout {
		t.Errorf("expected default timeout %v, got %v", DefaultStallTimeout, transfer.GetStallTimeout())
	}

	// Set custom timeout
	customTimeout := 10 * time.Second
	transfer.SetStallTimeout(customTimeout)

	if transfer.GetStallTimeout() != customTimeout {
		t.Errorf("expected timeout %v, got %v", customTimeout, transfer.GetStallTimeout())
	}

	// Disable timeout
	transfer.SetStallTimeout(0)
	if transfer.GetStallTimeout() != 0 {
		t.Errorf("expected timeout 0, got %v", transfer.GetStallTimeout())
	}
}

func TestTransfer_IsStalled_NotRunning(t *testing.T) {
	transfer := NewTransfer(1, 1, "test.txt", 1024, TransferDirectionIncoming)
	tp := newMockTimeProvider()
	transfer.SetTimeProvider(tp)

	// Pending state should not be stalled
	if transfer.IsStalled() {
		t.Error("pending transfer should not be stalled")
	}

	// Advance time past timeout
	tp.advance(DefaultStallTimeout + 1*time.Second)

	// Still not stalled because not running
	if transfer.IsStalled() {
		t.Error("non-running transfer should not be stalled even after timeout")
	}
}

func TestTransfer_IsStalled_TimeoutDisabled(t *testing.T) {
	transfer := NewTransfer(1, 1, "test.txt", 1024, TransferDirectionIncoming)
	tp := newMockTimeProvider()
	transfer.SetTimeProvider(tp)
	transfer.SetStallTimeout(0)
	transfer.State = TransferStateRunning

	// Advance time
	tp.advance(1 * time.Hour)

	if transfer.IsStalled() {
		t.Error("transfer with disabled timeout should not report stalled")
	}
}

func TestTransfer_IsStalled_Running(t *testing.T) {
	transfer := NewTransfer(1, 1, "test.txt", 1024, TransferDirectionIncoming)
	tp := newMockTimeProvider()
	transfer.SetTimeProvider(tp)
	transfer.SetStallTimeout(10 * time.Second)
	transfer.State = TransferStateRunning

	// Just started - should not be stalled
	if transfer.IsStalled() {
		t.Error("newly started transfer should not be stalled")
	}

	// Advance time but not past timeout
	tp.advance(5 * time.Second)
	if transfer.IsStalled() {
		t.Error("transfer should not be stalled before timeout")
	}

	// Advance time past timeout
	tp.advance(6 * time.Second)
	if !transfer.IsStalled() {
		t.Error("transfer should be stalled after timeout")
	}
}

func TestTransfer_CheckTimeout_NotRunning(t *testing.T) {
	transfer := NewTransfer(1, 1, "test.txt", 1024, TransferDirectionIncoming)
	tp := newMockTimeProvider()
	transfer.SetTimeProvider(tp)

	// Advance time past timeout
	tp.advance(DefaultStallTimeout + 1*time.Second)

	err := transfer.CheckTimeout()
	if err != nil {
		t.Errorf("expected no error for non-running transfer, got %v", err)
	}

	// State should be unchanged
	if transfer.State != TransferStatePending {
		t.Errorf("expected state %v, got %v", TransferStatePending, transfer.State)
	}
}

func TestTransfer_CheckTimeout_TimeoutDisabled(t *testing.T) {
	transfer := NewTransfer(1, 1, "test.txt", 1024, TransferDirectionIncoming)
	tp := newMockTimeProvider()
	transfer.SetTimeProvider(tp)
	transfer.SetStallTimeout(0)
	transfer.State = TransferStateRunning

	// Advance time
	tp.advance(1 * time.Hour)

	err := transfer.CheckTimeout()
	if err != nil {
		t.Errorf("expected no error with timeout disabled, got %v", err)
	}

	// State should still be running
	if transfer.State != TransferStateRunning {
		t.Errorf("expected state %v, got %v", TransferStateRunning, transfer.State)
	}
}

func TestTransfer_CheckTimeout_Running_NoStall(t *testing.T) {
	transfer := NewTransfer(1, 1, "test.txt", 1024, TransferDirectionIncoming)
	tp := newMockTimeProvider()
	transfer.SetTimeProvider(tp)
	transfer.SetStallTimeout(10 * time.Second)
	transfer.State = TransferStateRunning

	// Advance time but not past timeout
	tp.advance(5 * time.Second)

	err := transfer.CheckTimeout()
	if err != nil {
		t.Errorf("expected no error before timeout, got %v", err)
	}

	// State should still be running
	if transfer.State != TransferStateRunning {
		t.Errorf("expected state %v, got %v", TransferStateRunning, transfer.State)
	}
}

func TestTransfer_CheckTimeout_Running_Stalled(t *testing.T) {
	transfer := NewTransfer(1, 1, "test.txt", 1024, TransferDirectionIncoming)
	tp := newMockTimeProvider()
	transfer.SetTimeProvider(tp)
	transfer.SetStallTimeout(10 * time.Second)
	transfer.State = TransferStateRunning

	// Advance time past timeout
	tp.advance(15 * time.Second)

	err := transfer.CheckTimeout()
	if err != ErrTransferStalled {
		t.Errorf("expected ErrTransferStalled, got %v", err)
	}

	// State should be error
	if transfer.State != TransferStateError {
		t.Errorf("expected state %v, got %v", TransferStateError, transfer.State)
	}

	// Error should be set
	if transfer.Error != ErrTransferStalled {
		t.Errorf("expected Error field to be ErrTransferStalled, got %v", transfer.Error)
	}
}

func TestTransfer_CheckTimeout_CallbackCalled(t *testing.T) {
	transfer := NewTransfer(1, 1, "test.txt", 1024, TransferDirectionIncoming)
	tp := newMockTimeProvider()
	transfer.SetTimeProvider(tp)
	transfer.SetStallTimeout(10 * time.Second)
	transfer.State = TransferStateRunning

	// Set up callback
	var callbackError error
	callbackCalled := false
	transfer.OnComplete(func(err error) {
		callbackCalled = true
		callbackError = err
	})

	// Advance time past timeout
	tp.advance(15 * time.Second)

	_ = transfer.CheckTimeout()

	if !callbackCalled {
		t.Error("complete callback should have been called")
	}
	if callbackError != ErrTransferStalled {
		t.Errorf("callback should receive ErrTransferStalled, got %v", callbackError)
	}
}

func TestTransfer_GetTimeSinceLastChunk(t *testing.T) {
	transfer := NewTransfer(1, 1, "test.txt", 1024, TransferDirectionIncoming)
	tp := newMockTimeProvider()
	transfer.SetTimeProvider(tp)

	// Initially should be zero (or very small)
	initial := transfer.GetTimeSinceLastChunk()
	if initial != 0 {
		t.Errorf("expected 0 initially, got %v", initial)
	}

	// Advance time
	tp.advance(5 * time.Second)

	elapsed := transfer.GetTimeSinceLastChunk()
	if elapsed != 5*time.Second {
		t.Errorf("expected 5s, got %v", elapsed)
	}
}

func TestTransfer_WriteChunk_ResetsTimeout(t *testing.T) {
	// Create a temporary file for testing
	tempFile := t.TempDir() + "/test_timeout.txt"

	transfer := NewTransfer(1, 1, tempFile, 1024, TransferDirectionIncoming)
	tp := newMockTimeProvider()
	transfer.SetTimeProvider(tp)
	transfer.SetStallTimeout(10 * time.Second)

	// Start the transfer
	transfer.State = TransferStatePending
	err := transfer.Start()
	if err != nil {
		t.Fatalf("failed to start transfer: %v", err)
	}
	defer transfer.Cancel()

	// Advance time close to timeout
	tp.advance(9 * time.Second)

	// Should not be stalled yet
	if transfer.IsStalled() {
		t.Error("transfer should not be stalled before timeout")
	}

	// Write a chunk (resets the lastChunkTime)
	err = transfer.WriteChunk([]byte("test data"))
	if err != nil {
		t.Fatalf("failed to write chunk: %v", err)
	}

	// Advance time close to timeout again
	tp.advance(9 * time.Second)

	// Should not be stalled because chunk was received
	if transfer.IsStalled() {
		t.Error("transfer should not be stalled after receiving data")
	}
}

func TestDefaultStallTimeout(t *testing.T) {
	if DefaultStallTimeout != 30*time.Second {
		t.Errorf("expected DefaultStallTimeout to be 30s, got %v", DefaultStallTimeout)
	}
}

func TestErrTransferStalled(t *testing.T) {
	if ErrTransferStalled.Error() != "transfer stalled: no data received within timeout period" {
		t.Errorf("unexpected error message: %v", ErrTransferStalled.Error())
	}
}

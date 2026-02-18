package file

import (
	"errors"
	"os"
	"path/filepath"
	"testing"
	"time"
)

// TestTransferStateTransitions tests all valid and invalid state transitions for file transfers.
func TestTransferStateTransitions(t *testing.T) {
	tests := []struct {
		name          string
		initialState  TransferState
		operation     string
		wantState     TransferState
		wantError     bool
		errorContains string
	}{
		// Valid transitions from Pending state
		{
			name:         "pending_to_running_via_start",
			initialState: TransferStatePending,
			operation:    "start",
			wantState:    TransferStateRunning,
			wantError:    false,
		},
		// Invalid transitions from Pending state
		{
			name:          "pending_cannot_pause",
			initialState:  TransferStatePending,
			operation:     "pause",
			wantState:     TransferStatePending,
			wantError:     true,
			errorContains: "not running",
		},
		{
			name:          "pending_cannot_resume",
			initialState:  TransferStatePending,
			operation:     "resume",
			wantState:     TransferStatePending,
			wantError:     true,
			errorContains: "not paused",
		},
		{
			name:         "pending_can_cancel",
			initialState: TransferStatePending,
			operation:    "cancel",
			wantState:    TransferStateCancelled,
			wantError:    false,
		},
		// Valid transitions from Running state
		{
			name:         "running_to_paused_via_pause",
			initialState: TransferStateRunning,
			operation:    "pause",
			wantState:    TransferStatePaused,
			wantError:    false,
		},
		{
			name:         "running_can_cancel",
			initialState: TransferStateRunning,
			operation:    "cancel",
			wantState:    TransferStateCancelled,
			wantError:    false,
		},
		// Invalid transitions from Running state
		{
			name:          "running_cannot_start_again",
			initialState:  TransferStateRunning,
			operation:     "start",
			wantState:     TransferStateRunning,
			wantError:     true,
			errorContains: "cannot be started",
		},
		{
			name:          "running_cannot_resume",
			initialState:  TransferStateRunning,
			operation:     "resume",
			wantState:     TransferStateRunning,
			wantError:     true,
			errorContains: "not paused",
		},
		// Valid transitions from Paused state
		{
			name:         "paused_to_running_via_resume",
			initialState: TransferStatePaused,
			operation:    "resume",
			wantState:    TransferStateRunning,
			wantError:    false,
		},
		{
			name:         "paused_to_running_via_start",
			initialState: TransferStatePaused,
			operation:    "start",
			wantState:    TransferStateRunning,
			wantError:    false,
		},
		{
			name:         "paused_can_cancel",
			initialState: TransferStatePaused,
			operation:    "cancel",
			wantState:    TransferStateCancelled,
			wantError:    false,
		},
		// Invalid transitions from Paused state
		{
			name:          "paused_cannot_pause_again",
			initialState:  TransferStatePaused,
			operation:     "pause",
			wantState:     TransferStatePaused,
			wantError:     true,
			errorContains: "not running",
		},
		// Terminal state transitions
		{
			name:          "completed_cannot_start",
			initialState:  TransferStateCompleted,
			operation:     "start",
			wantState:     TransferStateCompleted,
			wantError:     true,
			errorContains: "cannot be started",
		},
		{
			name:          "completed_cannot_pause",
			initialState:  TransferStateCompleted,
			operation:     "pause",
			wantState:     TransferStateCompleted,
			wantError:     true,
			errorContains: "not running",
		},
		{
			name:          "completed_cannot_resume",
			initialState:  TransferStateCompleted,
			operation:     "resume",
			wantState:     TransferStateCompleted,
			wantError:     true,
			errorContains: "not paused",
		},
		{
			name:          "completed_cannot_cancel",
			initialState:  TransferStateCompleted,
			operation:     "cancel",
			wantState:     TransferStateCompleted,
			wantError:     true,
			errorContains: "already finished",
		},
		{
			name:          "cancelled_cannot_start",
			initialState:  TransferStateCancelled,
			operation:     "start",
			wantState:     TransferStateCancelled,
			wantError:     true,
			errorContains: "cannot be started",
		},
		{
			name:          "cancelled_cannot_cancel_again",
			initialState:  TransferStateCancelled,
			operation:     "cancel",
			wantState:     TransferStateCancelled,
			wantError:     true,
			errorContains: "already finished",
		},
		{
			name:          "error_cannot_start",
			initialState:  TransferStateError,
			operation:     "start",
			wantState:     TransferStateError,
			wantError:     true,
			errorContains: "cannot be started",
		},
		{
			name:          "error_cannot_pause",
			initialState:  TransferStateError,
			operation:     "pause",
			wantState:     TransferStateError,
			wantError:     true,
			errorContains: "not running",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			tmpDir := t.TempDir()
			testFile := filepath.Join(tmpDir, "test_state.txt")

			// Create test file for outgoing transfers
			if err := os.WriteFile(testFile, []byte("test content for state testing"), 0o644); err != nil {
				t.Fatalf("Failed to create test file: %v", err)
			}

			transfer := NewTransfer(1, 1, testFile, 1024, TransferDirectionOutgoing)

			// Set up initial state
			transfer.State = tt.initialState

			// For operations that need a file handle
			if tt.initialState == TransferStateRunning || tt.initialState == TransferStatePaused {
				handle, err := os.Open(testFile)
				if err != nil {
					t.Fatalf("Failed to open test file: %v", err)
				}
				transfer.FileHandle = handle
			}

			// Perform operation
			var err error
			switch tt.operation {
			case "start":
				err = transfer.Start()
			case "pause":
				err = transfer.Pause()
			case "resume":
				err = transfer.Resume()
			case "cancel":
				err = transfer.Cancel()
			}

			// Check error expectation
			if tt.wantError {
				if err == nil {
					t.Errorf("Expected error containing %q, got nil", tt.errorContains)
				} else if tt.errorContains != "" && !containsString(err.Error(), tt.errorContains) {
					t.Errorf("Error %q does not contain expected %q", err.Error(), tt.errorContains)
				}
			} else if err != nil {
				t.Errorf("Unexpected error: %v", err)
			}

			// Check final state
			if transfer.State != tt.wantState {
				t.Errorf("State = %v, want %v", transfer.State, tt.wantState)
			}

			// Cleanup
			if transfer.FileHandle != nil {
				transfer.FileHandle.Close()
			}
		})
	}
}

// containsString checks if str contains substr.
func containsString(str, substr string) bool {
	return len(str) >= len(substr) && (str == substr || len(substr) == 0 ||
		(len(str) > 0 && len(substr) > 0 && findSubstring(str, substr)))
}

func findSubstring(str, substr string) bool {
	for i := 0; i <= len(str)-len(substr); i++ {
		if str[i:i+len(substr)] == substr {
			return true
		}
	}
	return false
}

// TestTransferStateWriteChunkErrors tests WriteChunk error conditions.
func TestTransferStateWriteChunkErrors(t *testing.T) {
	tests := []struct {
		name          string
		direction     TransferDirection
		state         TransferState
		data          []byte
		wantError     bool
		errorContains string
	}{
		{
			name:          "cannot_write_to_outgoing",
			direction:     TransferDirectionOutgoing,
			state:         TransferStateRunning,
			data:          []byte("test"),
			wantError:     true,
			errorContains: "outgoing",
		},
		{
			name:          "cannot_write_when_pending",
			direction:     TransferDirectionIncoming,
			state:         TransferStatePending,
			data:          []byte("test"),
			wantError:     true,
			errorContains: "not running",
		},
		{
			name:          "cannot_write_when_paused",
			direction:     TransferDirectionIncoming,
			state:         TransferStatePaused,
			data:          []byte("test"),
			wantError:     true,
			errorContains: "not running",
		},
		{
			name:          "cannot_write_when_completed",
			direction:     TransferDirectionIncoming,
			state:         TransferStateCompleted,
			data:          []byte("test"),
			wantError:     true,
			errorContains: "not running",
		},
		{
			name:          "cannot_write_oversized_chunk",
			direction:     TransferDirectionIncoming,
			state:         TransferStateRunning,
			data:          make([]byte, MaxChunkSize+1),
			wantError:     true,
			errorContains: "",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			tmpDir := t.TempDir()
			testFile := filepath.Join(tmpDir, "write_error_test.txt")

			transfer := NewTransfer(1, 1, testFile, 1000000, tt.direction)
			transfer.State = tt.state

			// For running incoming transfers, need a file handle
			if tt.state == TransferStateRunning && tt.direction == TransferDirectionIncoming {
				handle, err := os.Create(testFile)
				if err != nil {
					t.Fatalf("Failed to create test file: %v", err)
				}
				transfer.FileHandle = handle
				defer handle.Close()
			}

			err := transfer.WriteChunk(tt.data)

			if tt.wantError {
				if err == nil {
					t.Error("Expected error, got nil")
				} else if tt.errorContains != "" && !containsString(err.Error(), tt.errorContains) {
					t.Errorf("Error %q does not contain %q", err.Error(), tt.errorContains)
				}
			} else if err != nil {
				t.Errorf("Unexpected error: %v", err)
			}
		})
	}
}

// TestTransferStateReadChunkErrors tests ReadChunk error conditions.
func TestTransferStateReadChunkErrors(t *testing.T) {
	tests := []struct {
		name          string
		direction     TransferDirection
		state         TransferState
		size          uint16
		wantError     bool
		errorContains string
	}{
		{
			name:          "cannot_read_from_incoming",
			direction:     TransferDirectionIncoming,
			state:         TransferStateRunning,
			size:          1024,
			wantError:     true,
			errorContains: "incoming",
		},
		{
			name:          "cannot_read_when_pending",
			direction:     TransferDirectionOutgoing,
			state:         TransferStatePending,
			size:          1024,
			wantError:     true,
			errorContains: "not running",
		},
		{
			name:          "cannot_read_when_paused",
			direction:     TransferDirectionOutgoing,
			state:         TransferStatePaused,
			size:          1024,
			wantError:     true,
			errorContains: "not running",
		},
		{
			name:          "cannot_read_when_completed",
			direction:     TransferDirectionOutgoing,
			state:         TransferStateCompleted,
			size:          1024,
			wantError:     true,
			errorContains: "not running",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			tmpDir := t.TempDir()
			testFile := filepath.Join(tmpDir, "read_error_test.txt")

			// Create test file for outgoing
			if err := os.WriteFile(testFile, make([]byte, 10000), 0o644); err != nil {
				t.Fatalf("Failed to create test file: %v", err)
			}

			transfer := NewTransfer(1, 1, testFile, 10000, tt.direction)
			transfer.State = tt.state

			// For running outgoing transfers, need a file handle
			if tt.state == TransferStateRunning && tt.direction == TransferDirectionOutgoing {
				handle, err := os.Open(testFile)
				if err != nil {
					t.Fatalf("Failed to open test file: %v", err)
				}
				transfer.FileHandle = handle
				defer handle.Close()
			}

			_, err := transfer.ReadChunk(tt.size)

			if tt.wantError {
				if err == nil {
					t.Error("Expected error, got nil")
				} else if tt.errorContains != "" && !containsString(err.Error(), tt.errorContains) {
					t.Errorf("Error %q does not contain %q", err.Error(), tt.errorContains)
				}
			} else if err != nil {
				t.Errorf("Unexpected error: %v", err)
			}
		})
	}
}

// TestTransferProgressCalculation tests progress percentage calculation.
func TestTransferProgressCalculation(t *testing.T) {
	tests := []struct {
		name        string
		transferred uint64
		fileSize    uint64
		wantPercent float64
	}{
		{
			name:        "zero_progress",
			transferred: 0,
			fileSize:    1000,
			wantPercent: 0.0,
		},
		{
			name:        "half_progress",
			transferred: 500,
			fileSize:    1000,
			wantPercent: 50.0,
		},
		{
			name:        "complete_progress",
			transferred: 1000,
			fileSize:    1000,
			wantPercent: 100.0,
		},
		{
			name:        "zero_file_size",
			transferred: 0,
			fileSize:    0,
			wantPercent: 0.0,
		},
		{
			name:        "quarter_progress",
			transferred: 256,
			fileSize:    1024,
			wantPercent: 25.0,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			transfer := NewTransfer(1, 1, "test.txt", tt.fileSize, TransferDirectionIncoming)
			transfer.Transferred = tt.transferred

			gotPercent := transfer.GetProgress()

			if gotPercent != tt.wantPercent {
				t.Errorf("GetProgress() = %v, want %v", gotPercent, tt.wantPercent)
			}
		})
	}
}

// TestTransferCallbacks tests that callbacks are properly invoked.
func TestTransferCallbacks(t *testing.T) {
	tmpDir := t.TempDir()
	testFile := filepath.Join(tmpDir, "callback_test.txt")

	// Create test file
	testData := make([]byte, 2048)
	for i := range testData {
		testData[i] = byte(i % 256)
	}
	if err := os.WriteFile(testFile, testData, 0o644); err != nil {
		t.Fatalf("Failed to create test file: %v", err)
	}

	t.Run("progress_callback_invoked", func(t *testing.T) {
		receiveFile := filepath.Join(tmpDir, "receive_progress.txt")
		transfer := NewTransfer(1, 1, receiveFile, 2048, TransferDirectionIncoming)

		progressCalls := 0
		var lastProgress uint64
		transfer.OnProgress(func(transferred uint64) {
			progressCalls++
			lastProgress = transferred
		})

		if err := transfer.Start(); err != nil {
			t.Fatalf("Start failed: %v", err)
		}
		defer transfer.FileHandle.Close()

		// Write some data
		if err := transfer.WriteChunk([]byte("test data")); err != nil {
			t.Fatalf("WriteChunk failed: %v", err)
		}

		if progressCalls == 0 {
			t.Error("Progress callback was not invoked")
		}

		if lastProgress != uint64(len("test data")) {
			t.Errorf("Last progress = %d, want %d", lastProgress, len("test data"))
		}
	})

	t.Run("complete_callback_on_cancel", func(t *testing.T) {
		transfer := NewTransfer(1, 1, "dummy.txt", 1024, TransferDirectionIncoming)
		transfer.State = TransferStateRunning

		completeCalled := false
		var completeErr error
		transfer.OnComplete(func(err error) {
			completeCalled = true
			completeErr = err
		})

		if err := transfer.Cancel(); err != nil {
			t.Fatalf("Cancel failed: %v", err)
		}

		if !completeCalled {
			t.Error("Complete callback was not invoked on cancel")
		}

		if completeErr == nil {
			t.Error("Expected error in complete callback for cancelled transfer")
		}
	})
}

// TestTransferSpeedCalculation tests transfer speed estimation.
func TestTransferSpeedCalculation(t *testing.T) {
	tmpDir := t.TempDir()
	testFile := filepath.Join(tmpDir, "speed_test.txt")

	// Create mock time provider for deterministic testing
	testTime := &testTimeProvider{currentTime: time.Now()}
	transfer := NewTransfer(1, 1, testFile, 100000, TransferDirectionIncoming)
	transfer.SetTimeProvider(testTime)

	if err := os.WriteFile(testFile, []byte{}, 0o644); err != nil {
		t.Fatalf("Failed to create test file: %v", err)
	}

	if err := transfer.Start(); err != nil {
		t.Fatalf("Start failed: %v", err)
	}
	defer transfer.FileHandle.Close()

	// Initial speed should be 0
	if speed := transfer.GetSpeed(); speed != 0 {
		t.Errorf("Initial speed = %v, want 0", speed)
	}

	// Write a chunk and advance time by 1 second
	testTime.Advance(1 * time.Second)
	if err := transfer.WriteChunk(make([]byte, 1024)); err != nil {
		t.Fatalf("WriteChunk failed: %v", err)
	}

	// Speed should be around 1024 bytes/second
	speed := transfer.GetSpeed()
	if speed < 900 || speed > 1100 {
		t.Errorf("Speed = %v, want approximately 1024", speed)
	}
}

// TestTransferTimeRemaining tests estimated time remaining calculation.
func TestTransferTimeRemaining(t *testing.T) {
	tmpDir := t.TempDir()
	testFile := filepath.Join(tmpDir, "time_remaining_test.txt")

	testTime := &testTimeProvider{currentTime: time.Now()}
	transfer := NewTransfer(1, 1, testFile, 10240, TransferDirectionIncoming)
	transfer.SetTimeProvider(testTime)

	// Not running - should return 0
	remaining := transfer.GetEstimatedTimeRemaining()
	if remaining != 0 {
		t.Errorf("Time remaining when not running = %v, want 0", remaining)
	}

	// Start transfer
	if err := os.WriteFile(testFile, []byte{}, 0o644); err != nil {
		t.Fatalf("Failed to create test file: %v", err)
	}
	if err := transfer.Start(); err != nil {
		t.Fatalf("Start failed: %v", err)
	}
	defer transfer.FileHandle.Close()

	// Zero speed - should return 0
	remaining = transfer.GetEstimatedTimeRemaining()
	if remaining != 0 {
		t.Errorf("Time remaining with zero speed = %v, want 0", remaining)
	}

	// Write data to establish speed (1024 bytes/sec)
	testTime.Advance(1 * time.Second)
	if err := transfer.WriteChunk(make([]byte, 1024)); err != nil {
		t.Fatalf("WriteChunk failed: %v", err)
	}

	// 9216 bytes remaining at ~1024 bytes/sec = ~9 seconds
	remaining = transfer.GetEstimatedTimeRemaining()
	// Allow some tolerance due to EMA smoothing
	if remaining < 8*time.Second || remaining > 12*time.Second {
		t.Errorf("Time remaining = %v, want approximately 9s", remaining)
	}
}

// testTimeProvider implements TimeProvider for deterministic testing.
// Note: This is distinct from mockTimeProvider in transfer_timeout_test.go
// to avoid redeclaration but uses the same pattern.
type testTimeProvider struct {
	currentTime time.Time
}

func (t *testTimeProvider) Now() time.Time {
	return t.currentTime
}

func (t *testTimeProvider) Since(ref time.Time) time.Duration {
	return t.currentTime.Sub(ref)
}

func (t *testTimeProvider) Advance(d time.Duration) {
	t.currentTime = t.currentTime.Add(d)
}

// TestTransferStallDetection tests the stall timeout detection mechanism.
func TestTransferStallDetection(t *testing.T) {
	tests := []struct {
		name         string
		stallTimeout time.Duration
		timePassed   time.Duration
		state        TransferState
		wantStalled  bool
	}{
		{
			name:         "not_stalled_within_timeout",
			stallTimeout: 30 * time.Second,
			timePassed:   15 * time.Second,
			state:        TransferStateRunning,
			wantStalled:  false,
		},
		{
			name:         "stalled_after_timeout",
			stallTimeout: 30 * time.Second,
			timePassed:   31 * time.Second,
			state:        TransferStateRunning,
			wantStalled:  true,
		},
		{
			name:         "stall_disabled_zero_timeout",
			stallTimeout: 0,
			timePassed:   1 * time.Hour,
			state:        TransferStateRunning,
			wantStalled:  false,
		},
		{
			name:         "paused_cannot_stall",
			stallTimeout: 30 * time.Second,
			timePassed:   1 * time.Hour,
			state:        TransferStatePaused,
			wantStalled:  false,
		},
		{
			name:         "pending_cannot_stall",
			stallTimeout: 30 * time.Second,
			timePassed:   1 * time.Hour,
			state:        TransferStatePending,
			wantStalled:  false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			testTime := &testTimeProvider{currentTime: time.Now()}
			transfer := NewTransfer(1, 1, "test.txt", 1024, TransferDirectionIncoming)
			transfer.SetTimeProvider(testTime)
			transfer.SetStallTimeout(tt.stallTimeout)
			transfer.State = tt.state

			// Advance time
			testTime.Advance(tt.timePassed)

			isStalled := transfer.IsStalled()
			if isStalled != tt.wantStalled {
				t.Errorf("IsStalled() = %v, want %v", isStalled, tt.wantStalled)
			}
		})
	}
}

// TestCheckTimeoutTriggersError tests that CheckTimeout properly sets error state.
func TestCheckTimeoutTriggersError(t *testing.T) {
	testTime := &testTimeProvider{currentTime: time.Now()}
	transfer := NewTransfer(1, 1, "test.txt", 1024, TransferDirectionIncoming)
	transfer.SetTimeProvider(testTime)
	transfer.SetStallTimeout(30 * time.Second)
	transfer.State = TransferStateRunning

	// Should not error within timeout
	err := transfer.CheckTimeout()
	if err != nil {
		t.Errorf("CheckTimeout() within timeout returned error: %v", err)
	}

	// Advance past timeout
	testTime.Advance(31 * time.Second)

	completeCalled := false
	var completeErr error
	transfer.OnComplete(func(err error) {
		completeCalled = true
		completeErr = err
	})

	err = transfer.CheckTimeout()
	if !errors.Is(err, ErrTransferStalled) {
		t.Errorf("CheckTimeout() = %v, want ErrTransferStalled", err)
	}

	if transfer.State != TransferStateError {
		t.Errorf("State = %v, want TransferStateError", transfer.State)
	}

	if !errors.Is(transfer.Error, ErrTransferStalled) {
		t.Errorf("Transfer.Error = %v, want ErrTransferStalled", transfer.Error)
	}

	if !completeCalled {
		t.Error("Complete callback not invoked on stall timeout")
	}

	if !errors.Is(completeErr, ErrTransferStalled) {
		t.Errorf("Complete callback error = %v, want ErrTransferStalled", completeErr)
	}
}

package av

import (
	"errors"
	"testing"

	"github.com/opd-ai/toxcore/av/video"
)

// sentinelEncoder is a minimal Encoder that records calls and satisfies the
// Encoder interface for factory tests.
type sentinelEncoder struct {
	width, height uint16
	bitRate       uint32
}

func (e *sentinelEncoder) Encode(*video.VideoFrame) ([]byte, error)  { return nil, nil }
func (e *sentinelEncoder) SetBitRate(uint32) error                   { return nil }
func (e *sentinelEncoder) SupportsInterframe() bool                   { return false }
func (e *sentinelEncoder) SetKeyFrameInterval(int)                    {}
func (e *sentinelEncoder) ForceKeyFrame()                             {}
func (e *sentinelEncoder) SetGoldenFrameInterval(int)                 {}
func (e *sentinelEncoder) ForceGoldenFrame()                          {}
func (e *sentinelEncoder) Close() error                               { return nil }

// testEncoderFactory returns a factory that records the parameters it was called
// with and creates a sentinelEncoder.
func testEncoderFactory(called *bool, gotW, gotH *uint16, gotBR *uint32) func(uint16, uint16, uint32) (video.Encoder, error) {
	return func(w, h uint16, br uint32) (video.Encoder, error) {
		*called = true
		*gotW = w
		*gotH = h
		*gotBR = br
		return &sentinelEncoder{width: w, height: h, bitRate: br}, nil
	}
}

// TestSetVideoEncoderFactory_AffectsOnlyNewCalls verifies that
// SetVideoEncoderFactory is propagated to calls created after the call
// but that calls created before are not affected.
func TestSetVideoEncoderFactory_AffectsOnlyNewCalls(t *testing.T) {
	tr := newMockTransport()
	mgr, err := NewManager(tr, mockFriendLookup)
	if err != nil {
		t.Fatalf("NewManager: %v", err)
	}

	// Call created before factory is set must not have a factory.
	callBefore := NewCall(10)
	if callBefore.encoderFactory != nil {
		t.Error("call created before SetVideoEncoderFactory should have nil factory")
	}

	var factoryCalled bool
	var gotW, gotH uint16
	var gotBR uint32
	mgr.SetVideoEncoderFactory(testEncoderFactory(&factoryCalled, &gotW, &gotH, &gotBR))

	// Verify the factory is recorded on the manager.
	mgr.mu.RLock()
	hasFactory := mgr.videoEncoderFactory != nil
	mgr.mu.RUnlock()
	if !hasFactory {
		t.Fatal("videoEncoderFactory should be non-nil after SetVideoEncoderFactory")
	}

	// Calls created through the manager's internal helpers should carry the factory.
	// We create one via createCallSession (outgoing path).
	mgr.mu.Lock()
	call := mgr.createCallSession(99, 1, 48000, 512000)
	mgr.mu.Unlock()

	if call.encoderFactory == nil {
		t.Fatal("call created after SetVideoEncoderFactory should have non-nil encoderFactory")
	}

	// Invoke the factory to ensure it is the one we registered.
	_, err = call.encoderFactory(320, 240, 256000)
	if err != nil {
		t.Fatalf("encoderFactory returned unexpected error: %v", err)
	}
	if !factoryCalled {
		t.Error("registered factory was not called")
	}
}

// TestSetVideoEncoderFactory_IncomingCallPath verifies that incoming calls
// (processIncomingCall) also receive the configured factory.
func TestSetVideoEncoderFactory_IncomingCallPath(t *testing.T) {
	tr := newMockTransport()
	mgr, err := NewManager(tr, mockFriendLookup)
	if err != nil {
		t.Fatalf("NewManager: %v", err)
	}

	var factoryCalled bool
	var dummyW, dummyH uint16
	var dummyBR uint32
	mgr.SetVideoEncoderFactory(testEncoderFactory(&factoryCalled, &dummyW, &dummyH, &dummyBR))

	// Simulate an incoming call packet via processIncomingCall.
	// processIncomingCall acquires its own lock, so do NOT hold mgr.mu here.
	req := &CallRequestPacket{
		CallID:       42,
		AudioBitRate: 48000,
		VideoBitRate: 512000,
	}

	if err := mgr.processIncomingCall(77, req); err != nil {
		t.Fatalf("processIncomingCall: %v", err)
	}

	mgr.mu.RLock()
	call, exists := mgr.calls[77]
	mgr.mu.RUnlock()
	if !exists {
		t.Fatal("expected call 77 to be registered")
	}
	if call.encoderFactory == nil {
		t.Error("incoming call should carry the configured encoder factory")
	}
}

// TestSetVideoEncoderFactory_FactoryError verifies that a failing factory
// causes initializeVideoProcessor to fall back to the default processor.
func TestSetVideoEncoderFactory_FactoryError(t *testing.T) {
	errFactory := func(w, h uint16, br uint32) (video.Encoder, error) {
		return nil, errors.New("factory error")
	}

	call := NewCall(1)
	call.encoderFactory = errFactory
	// initializeVideoProcessor should fall back gracefully (no panic).
	call.initializeVideoProcessor()
	if call.videoProcessor == nil {
		t.Error("expected fallback video processor to be created on factory error")
	}
}

// TestInitializeVideoProcessor_UsesNegotiatedBitRate verifies that
// initializeVideoProcessor passes the call's videoBitRate to the factory.
func TestInitializeVideoProcessor_UsesNegotiatedBitRate(t *testing.T) {
	const negotiatedBR = uint32(1_000_000) // 1 Mbps

	var gotBR uint32
	call := NewCall(1)
	call.videoBitRate = negotiatedBR
	call.encoderFactory = func(w, h uint16, br uint32) (video.Encoder, error) {
		gotBR = br
		return &sentinelEncoder{width: w, height: h, bitRate: br}, nil
	}

	call.initializeVideoProcessor()

	if gotBR != negotiatedBR {
		t.Errorf("factory received bit rate %d, want %d", gotBR, negotiatedBR)
	}
}

// TestInitializeVideoProcessor_DefaultBitRateFallback verifies that when
// videoBitRate is 0 the factory receives DefaultProcessorBitRate.
func TestInitializeVideoProcessor_DefaultBitRateFallback(t *testing.T) {
	var gotBR uint32
	call := NewCall(1)
	call.videoBitRate = 0 // not yet negotiated
	call.encoderFactory = func(w, h uint16, br uint32) (video.Encoder, error) {
		gotBR = br
		return &sentinelEncoder{width: w, height: h, bitRate: br}, nil
	}

	call.initializeVideoProcessor()

	if gotBR != video.DefaultProcessorBitRate {
		t.Errorf("factory received bit rate %d, want DefaultProcessorBitRate %d", gotBR, video.DefaultProcessorBitRate)
	}
}

// TestSetVideoEncoderFactory_NilResets verifies that passing nil resets the
// factory so subsequent calls use the default encoder path.
func TestSetVideoEncoderFactory_NilResets(t *testing.T) {
	tr := newMockTransport()
	mgr, err := NewManager(tr, mockFriendLookup)
	if err != nil {
		t.Fatalf("NewManager: %v", err)
	}

	var dummy bool
	var dW, dH uint16
	var dBR uint32
	mgr.SetVideoEncoderFactory(testEncoderFactory(&dummy, &dW, &dH, &dBR))
	mgr.SetVideoEncoderFactory(nil) // reset

	mgr.mu.RLock()
	hasFactory := mgr.videoEncoderFactory != nil
	mgr.mu.RUnlock()

	if hasFactory {
		t.Error("videoEncoderFactory should be nil after passing nil to SetVideoEncoderFactory")
	}
}

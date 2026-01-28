package av

import (
	"testing"
)

// TestCallPauseResume verifies pause/resume call control functionality.
func TestCallPauseResume(t *testing.T) {
	transport := newMockTransport()
	manager, err := NewManager(transport, mockFriendLookup)
	if err != nil {
		t.Fatalf("Failed to create manager: %v", err)
	}

	err = manager.Start()
	if err != nil {
		t.Fatalf("Failed to start manager: %v", err)
	}
	defer manager.Stop()

	friendNumber := uint32(1)

	// Start a call first
	err = manager.StartCall(friendNumber, 48000, 500000)
	if err != nil {
		t.Fatalf("Failed to start call: %v", err)
	}

	call := manager.GetCall(friendNumber)
	if call == nil {
		t.Fatal("Call should exist after StartCall")
	}

	// Verify initial state - not paused
	if call.IsPaused() {
		t.Error("Call should not be paused initially")
	}

	// Clear sent packets
	transport.sentPackets = nil

	// Test pause
	err = manager.PauseCall(friendNumber)
	if err != nil {
		t.Fatalf("Failed to pause call: %v", err)
	}

	// Verify call is paused
	if !call.IsPaused() {
		t.Error("Call should be paused after PauseCall")
	}

	// Verify pause control packet was sent
	if len(transport.sentPackets) != 1 {
		t.Fatalf("Expected 1 pause packet, got %d", len(transport.sentPackets))
	}

	packet := transport.sentPackets[0]
	if packet.packetType != 0x32 {
		t.Errorf("Expected packet type 0x32, got 0x%02x", packet.packetType)
	}

	// Verify deserialization
	ctrl, err := DeserializeCallControl(packet.data)
	if err != nil {
		t.Fatalf("Failed to deserialize control packet: %v", err)
	}
	if ctrl.ControlType != CallControlPause {
		t.Errorf("Expected CallControlPause, got %v", ctrl.ControlType)
	}

	// Test pause when already paused - should error
	err = manager.PauseCall(friendNumber)
	if err == nil {
		t.Error("PauseCall should fail when call is already paused")
	}

	// Clear sent packets
	transport.sentPackets = nil

	// Test resume
	err = manager.ResumeCall(friendNumber)
	if err != nil {
		t.Fatalf("Failed to resume call: %v", err)
	}

	// Verify call is not paused
	if call.IsPaused() {
		t.Error("Call should not be paused after ResumeCall")
	}

	// Verify resume control packet was sent
	if len(transport.sentPackets) != 1 {
		t.Fatalf("Expected 1 resume packet, got %d", len(transport.sentPackets))
	}

	packet = transport.sentPackets[0]
	if packet.packetType != 0x32 {
		t.Errorf("Expected packet type 0x32, got 0x%02x", packet.packetType)
	}

	ctrl, err = DeserializeCallControl(packet.data)
	if err != nil {
		t.Fatalf("Failed to deserialize control packet: %v", err)
	}
	if ctrl.ControlType != CallControlResume {
		t.Errorf("Expected CallControlResume, got %v", ctrl.ControlType)
	}

	// Test resume when not paused - should error
	err = manager.ResumeCall(friendNumber)
	if err == nil {
		t.Error("ResumeCall should fail when call is not paused")
	}
}

// TestAudioMuteUnmute verifies audio mute/unmute call control functionality.
func TestAudioMuteUnmute(t *testing.T) {
	transport := newMockTransport()
	manager, err := NewManager(transport, mockFriendLookup)
	if err != nil {
		t.Fatalf("Failed to create manager: %v", err)
	}

	err = manager.Start()
	if err != nil {
		t.Fatalf("Failed to start manager: %v", err)
	}
	defer manager.Stop()

	friendNumber := uint32(2)

	// Start a call first
	err = manager.StartCall(friendNumber, 48000, 500000)
	if err != nil {
		t.Fatalf("Failed to start call: %v", err)
	}

	call := manager.GetCall(friendNumber)
	if call == nil {
		t.Fatal("Call should exist after StartCall")
	}

	// Verify initial state - not muted
	if call.IsAudioMuted() {
		t.Error("Audio should not be muted initially")
	}

	// Clear sent packets
	transport.sentPackets = nil

	// Test mute
	err = manager.MuteAudio(friendNumber)
	if err != nil {
		t.Fatalf("Failed to mute audio: %v", err)
	}

	// Verify audio is muted
	if !call.IsAudioMuted() {
		t.Error("Audio should be muted after MuteAudio")
	}

	// Verify mute control packet was sent
	if len(transport.sentPackets) != 1 {
		t.Fatalf("Expected 1 mute packet, got %d", len(transport.sentPackets))
	}

	packet := transport.sentPackets[0]
	if packet.packetType != 0x32 {
		t.Errorf("Expected packet type 0x32, got 0x%02x", packet.packetType)
	}

	ctrl, err := DeserializeCallControl(packet.data)
	if err != nil {
		t.Fatalf("Failed to deserialize control packet: %v", err)
	}
	if ctrl.ControlType != CallControlMuteAudio {
		t.Errorf("Expected CallControlMuteAudio, got %v", ctrl.ControlType)
	}

	// Test mute when already muted - should error
	err = manager.MuteAudio(friendNumber)
	if err == nil {
		t.Error("MuteAudio should fail when audio is already muted")
	}

	// Clear sent packets
	transport.sentPackets = nil

	// Test unmute
	err = manager.UnmuteAudio(friendNumber)
	if err != nil {
		t.Fatalf("Failed to unmute audio: %v", err)
	}

	// Verify audio is not muted
	if call.IsAudioMuted() {
		t.Error("Audio should not be muted after UnmuteAudio")
	}

	// Verify unmute control packet was sent
	if len(transport.sentPackets) != 1 {
		t.Fatalf("Expected 1 unmute packet, got %d", len(transport.sentPackets))
	}

	packet = transport.sentPackets[0]
	if packet.packetType != 0x32 {
		t.Errorf("Expected packet type 0x32, got 0x%02x", packet.packetType)
	}

	ctrl, err = DeserializeCallControl(packet.data)
	if err != nil {
		t.Fatalf("Failed to deserialize control packet: %v", err)
	}
	if ctrl.ControlType != CallControlUnmuteAudio {
		t.Errorf("Expected CallControlUnmuteAudio, got %v", ctrl.ControlType)
	}

	// Test unmute when not muted - should error
	err = manager.UnmuteAudio(friendNumber)
	if err == nil {
		t.Error("UnmuteAudio should fail when audio is not muted")
	}
}

// TestVideoHideShow verifies video hide/show call control functionality.
func TestVideoHideShow(t *testing.T) {
	transport := newMockTransport()
	manager, err := NewManager(transport, mockFriendLookup)
	if err != nil {
		t.Fatalf("Failed to create manager: %v", err)
	}

	err = manager.Start()
	if err != nil {
		t.Fatalf("Failed to start manager: %v", err)
	}
	defer manager.Stop()

	friendNumber := uint32(3)

	// Start a call first
	err = manager.StartCall(friendNumber, 48000, 500000)
	if err != nil {
		t.Fatalf("Failed to start call: %v", err)
	}

	call := manager.GetCall(friendNumber)
	if call == nil {
		t.Fatal("Call should exist after StartCall")
	}

	// Verify initial state - not hidden
	if call.IsVideoHidden() {
		t.Error("Video should not be hidden initially")
	}

	// Clear sent packets
	transport.sentPackets = nil

	// Test hide
	err = manager.HideVideo(friendNumber)
	if err != nil {
		t.Fatalf("Failed to hide video: %v", err)
	}

	// Verify video is hidden
	if !call.IsVideoHidden() {
		t.Error("Video should be hidden after HideVideo")
	}

	// Verify hide control packet was sent
	if len(transport.sentPackets) != 1 {
		t.Fatalf("Expected 1 hide packet, got %d", len(transport.sentPackets))
	}

	packet := transport.sentPackets[0]
	if packet.packetType != 0x32 {
		t.Errorf("Expected packet type 0x32, got 0x%02x", packet.packetType)
	}

	ctrl, err := DeserializeCallControl(packet.data)
	if err != nil {
		t.Fatalf("Failed to deserialize control packet: %v", err)
	}
	if ctrl.ControlType != CallControlHideVideo {
		t.Errorf("Expected CallControlHideVideo, got %v", ctrl.ControlType)
	}

	// Test hide when already hidden - should error
	err = manager.HideVideo(friendNumber)
	if err == nil {
		t.Error("HideVideo should fail when video is already hidden")
	}

	// Clear sent packets
	transport.sentPackets = nil

	// Test show
	err = manager.ShowVideo(friendNumber)
	if err != nil {
		t.Fatalf("Failed to show video: %v", err)
	}

	// Verify video is not hidden
	if call.IsVideoHidden() {
		t.Error("Video should not be hidden after ShowVideo")
	}

	// Verify show control packet was sent
	if len(transport.sentPackets) != 1 {
		t.Fatalf("Expected 1 show packet, got %d", len(transport.sentPackets))
	}

	packet = transport.sentPackets[0]
	if packet.packetType != 0x32 {
		t.Errorf("Expected packet type 0x32, got 0x%02x", packet.packetType)
	}

	ctrl, err = DeserializeCallControl(packet.data)
	if err != nil {
		t.Fatalf("Failed to deserialize control packet: %v", err)
	}
	if ctrl.ControlType != CallControlShowVideo {
		t.Errorf("Expected CallControlShowVideo, got %v", ctrl.ControlType)
	}

	// Test show when not hidden - should error
	err = manager.ShowVideo(friendNumber)
	if err == nil {
		t.Error("ShowVideo should fail when video is not hidden")
	}
}

// TestCallControlNoActiveCall verifies error handling when no call exists.
func TestCallControlNoActiveCall(t *testing.T) {
	transport := newMockTransport()
	manager, err := NewManager(transport, mockFriendLookup)
	if err != nil {
		t.Fatalf("Failed to create manager: %v", err)
	}

	err = manager.Start()
	if err != nil {
		t.Fatalf("Failed to start manager: %v", err)
	}
	defer manager.Stop()

	friendNumber := uint32(99)

	// Test all control functions without active call
	testCases := []struct {
		name string
		fn   func(uint32) error
	}{
		{"PauseCall", manager.PauseCall},
		{"ResumeCall", manager.ResumeCall},
		{"MuteAudio", manager.MuteAudio},
		{"UnmuteAudio", manager.UnmuteAudio},
		{"HideVideo", manager.HideVideo},
		{"ShowVideo", manager.ShowVideo},
	}

	for _, tc := range testCases {
		t.Run(tc.name, func(t *testing.T) {
			err := tc.fn(friendNumber)
			if err == nil {
				t.Errorf("%s should fail when no active call exists", tc.name)
			}
		})
	}
}

// TestIncomingCallControlPackets verifies handling of incoming call control packets.
func TestIncomingCallControlPackets(t *testing.T) {
	transport := newMockTransport()
	manager, err := NewManager(transport, mockFriendLookup)
	if err != nil {
		t.Fatalf("Failed to create manager: %v", err)
	}

	err = manager.Start()
	if err != nil {
		t.Fatalf("Failed to start manager: %v", err)
	}
	defer manager.Stop()

	friendNumber := uint32(4)
	friendAddr := []byte{byte(friendNumber), 0, 0, 0}

	// Start a call
	err = manager.StartCall(friendNumber, 48000, 500000)
	if err != nil {
		t.Fatalf("Failed to start call: %v", err)
	}

	call := manager.GetCall(friendNumber)
	if call == nil {
		t.Fatal("Call should exist after StartCall")
	}

	// Test incoming pause control
	pauseCtrl := &CallControlPacket{
		CallID:      call.callID,
		ControlType: CallControlPause,
	}
	pauseData, _ := SerializeCallControl(pauseCtrl)
	err = transport.simulatePacket(0x32, pauseData, friendAddr)
	if err != nil {
		t.Fatalf("Failed to simulate pause packet: %v", err)
	}

	if !call.IsPaused() {
		t.Error("Call should be paused after receiving pause control")
	}

	// Test incoming resume control
	resumeCtrl := &CallControlPacket{
		CallID:      call.callID,
		ControlType: CallControlResume,
	}
	resumeData, _ := SerializeCallControl(resumeCtrl)
	err = transport.simulatePacket(0x32, resumeData, friendAddr)
	if err != nil {
		t.Fatalf("Failed to simulate resume packet: %v", err)
	}

	if call.IsPaused() {
		t.Error("Call should not be paused after receiving resume control")
	}
}

// TestCallStateGetters verifies the new getter methods for call control states.
func TestCallStateGetters(t *testing.T) {
	call := NewCall(1)

	// Test initial states
	if call.IsPaused() {
		t.Error("New call should not be paused")
	}
	if call.IsAudioMuted() {
		t.Error("New call should not have audio muted")
	}
	if call.IsVideoHidden() {
		t.Error("New call should not have video hidden")
	}

	// Test setters and getters
	call.SetPaused(true)
	if !call.IsPaused() {
		t.Error("Call should be paused after SetPaused(true)")
	}

	call.SetPaused(false)
	if call.IsPaused() {
		t.Error("Call should not be paused after SetPaused(false)")
	}

	call.SetAudioMuted(true)
	if !call.IsAudioMuted() {
		t.Error("Audio should be muted after SetAudioMuted(true)")
	}

	call.SetAudioMuted(false)
	if call.IsAudioMuted() {
		t.Error("Audio should not be muted after SetAudioMuted(false)")
	}

	call.SetVideoHidden(true)
	if !call.IsVideoHidden() {
		t.Error("Video should be hidden after SetVideoHidden(true)")
	}

	call.SetVideoHidden(false)
	if call.IsVideoHidden() {
		t.Error("Video should not be hidden after SetVideoHidden(false)")
	}
}

package main

import (
	"testing"
)

// TestToxAVInstanceRetrieval tests the complete flow of creating ToxAV from Tox
// and retrieving the Tox instance back from ToxAV.
func TestToxAVInstanceRetrieval(t *testing.T) {
	// Create a new Tox instance
	toxPtr := tox_new()
	if toxPtr == nil {
		t.Fatal("Failed to create Tox instance")
	}
	defer tox_kill(toxPtr)

	// Create a ToxAV instance from the Tox instance
	toxavPtr := toxav_new(toxPtr, nil)
	if toxavPtr == nil {
		t.Fatal("Failed to create ToxAV instance from Tox instance")
	}
	defer toxav_kill(toxavPtr)

	// Verify we can retrieve the original Tox instance from ToxAV
	retrievedToxPtr := toxav_get_tox_from_av(toxavPtr)
	if retrievedToxPtr == nil {
		t.Fatal("Failed to retrieve Tox instance from ToxAV")
	}

	// Verify the retrieved pointer matches the original
	if retrievedToxPtr != toxPtr {
		t.Errorf("Retrieved Tox pointer doesn't match original: got %v, want %v", retrievedToxPtr, toxPtr)
	}

	// Verify the ToxAV instance is functional
	interval := toxav_iteration_interval(toxavPtr)
	if interval == 0 {
		t.Error("Expected non-zero iteration interval")
	}

	// Verify iteration doesn't crash
	toxav_iterate(toxavPtr)
}

// TestToxAVWithNilTox verifies proper error handling when creating ToxAV with nil Tox
func TestToxAVWithNilTox(t *testing.T) {
	toxavPtr := toxav_new(nil, nil)
	if toxavPtr != nil {
		toxav_kill(toxavPtr)
		t.Error("Expected nil ToxAV instance when creating with nil Tox")
	}
}

// TestToxAVGetToxWithNilAV verifies proper handling of nil ToxAV pointer
func TestToxAVGetToxWithNilAV(t *testing.T) {
	toxPtr := toxav_get_tox_from_av(nil)
	if toxPtr != nil {
		t.Error("Expected nil when calling toxav_get_tox_from_av with nil")
	}
}

// TestMultipleToxAVInstances verifies multiple ToxAV instances can coexist
func TestMultipleToxAVInstances(t *testing.T) {
	// Create first Tox instance and ToxAV
	tox1 := tox_new()
	if tox1 == nil {
		t.Fatal("Failed to create first Tox instance")
	}
	defer tox_kill(tox1)

	toxav1 := toxav_new(tox1, nil)
	if toxav1 == nil {
		t.Fatal("Failed to create first ToxAV instance")
	}
	defer toxav_kill(toxav1)

	// Create second Tox instance and ToxAV
	tox2 := tox_new()
	if tox2 == nil {
		t.Fatal("Failed to create second Tox instance")
	}
	defer tox_kill(tox2)

	toxav2 := toxav_new(tox2, nil)
	if toxav2 == nil {
		t.Fatal("Failed to create second ToxAV instance")
	}
	defer toxav_kill(toxav2)

	// Verify each ToxAV returns its correct Tox instance
	retrievedTox1 := toxav_get_tox_from_av(toxav1)
	if retrievedTox1 != tox1 {
		t.Error("First ToxAV returned wrong Tox instance")
	}

	retrievedTox2 := toxav_get_tox_from_av(toxav2)
	if retrievedTox2 != tox2 {
		t.Error("Second ToxAV returned wrong Tox instance")
	}

	// Verify instances are distinct
	if tox1 == tox2 {
		t.Error("Tox instances should be distinct")
	}
	if toxav1 == toxav2 {
		t.Error("ToxAV instances should be distinct")
	}
}

// TestToxAVCleanup verifies proper cleanup when ToxAV is killed
func TestToxAVCleanup(t *testing.T) {
	tox := tox_new()
	if tox == nil {
		t.Fatal("Failed to create Tox instance")
	}
	defer tox_kill(tox)

	toxav := toxav_new(tox, nil)
	if toxav == nil {
		t.Fatal("Failed to create ToxAV instance")
	}

	// Kill the ToxAV instance
	toxav_kill(toxav)

	// Verify toxav_get_tox_from_av returns nil after cleanup
	retrievedTox := toxav_get_tox_from_av(toxav)
	if retrievedTox != nil {
		t.Error("Expected nil after ToxAV cleanup")
	}

	// Verify iteration interval returns default after cleanup
	interval := toxav_iteration_interval(toxav)
	if interval != 20 {
		t.Errorf("Expected default interval (20) after cleanup, got %d", interval)
	}
}

// TestToxAVOperationsWithValidInstance verifies ToxAV operations work correctly
func TestToxAVOperationsWithValidInstance(t *testing.T) {
	tox := tox_new()
	if tox == nil {
		t.Fatal("Failed to create Tox instance")
	}
	defer tox_kill(tox)

	toxav := toxav_new(tox, nil)
	if toxav == nil {
		t.Fatal("Failed to create ToxAV instance")
	}
	defer toxav_kill(toxav)

	// Test iteration interval
	interval := toxav_iteration_interval(toxav)
	if interval == 0 {
		t.Error("Expected non-zero iteration interval")
	}

	// Test iterate (should not crash)
	toxav_iterate(toxav)

	// Test that operations with invalid friend numbers return false
	if toxav_call(toxav, 0, 48000, 0, nil) {
		t.Error("Expected false when calling non-existent friend")
	}

	if toxav_answer(toxav, 0, 48000, 0, nil) {
		t.Error("Expected false when answering non-existent call")
	}

	// Test callback registration (should not crash)
	toxav_callback_call(toxav, nil, nil)
	toxav_callback_call_state(toxav, nil, nil)
	toxav_callback_audio_bit_rate(toxav, nil, nil)
	toxav_callback_video_bit_rate(toxav, nil, nil)
}

// TestToxInstanceOperations verifies basic Tox instance operations
func TestToxInstanceOperations(t *testing.T) {
	tox := tox_new()
	if tox == nil {
		t.Fatal("Failed to create Tox instance")
	}
	defer tox_kill(tox)

	// Test iteration interval
	interval := tox_iteration_interval(tox)
	if interval == 0 {
		t.Error("Expected non-zero iteration interval")
	}

	// Test iterate (should not crash)
	tox_iterate(tox)

	// Test address size
	addrSize := tox_self_get_address_size(tox)
	if addrSize == 0 {
		t.Error("Expected non-zero address size")
	}
}

// TestConcurrentToxAVAccess tests thread safety of instance management
func TestConcurrentToxAVAccess(t *testing.T) {
	tox := tox_new()
	if tox == nil {
		t.Fatal("Failed to create Tox instance")
	}
	defer tox_kill(tox)

	toxav := toxav_new(tox, nil)
	if toxav == nil {
		t.Fatal("Failed to create ToxAV instance")
	}
	defer toxav_kill(toxav)

	// Run concurrent operations
	done := make(chan bool, 10)
	for i := 0; i < 10; i++ {
		go func() {
			defer func() { done <- true }()

			// Concurrent reads should be safe
			toxav_iteration_interval(toxav)
			toxav_get_tox_from_av(toxav)
			toxav_iterate(toxav)
		}()
	}

	// Wait for all goroutines
	for i := 0; i < 10; i++ {
		<-done
	}
}

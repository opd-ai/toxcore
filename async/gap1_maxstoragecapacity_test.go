package async

import (
	"testing"
)

// TestGap1MissingMaxStorageCapacityConstant reproduces Gap #1
// This test verifies that MaxStorageCapacity constant exists and has the correct value
func TestGap1MissingMaxStorageCapacityConstant(t *testing.T) {
	// According to README.md, MaxStorageCapacity should be 1536000
	expectedValue := 1536000
	
	// This should compile and pass once the constant is defined
	if MaxStorageCapacity != expectedValue {
		t.Errorf("MaxStorageCapacity should be %d, got %d", expectedValue, MaxStorageCapacity)
	}
}

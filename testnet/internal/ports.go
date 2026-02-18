package internal

// Port constants for testnet infrastructure.
// These constants define default port ranges for test components to avoid conflicts.
const (
	// BootstrapDefaultPort is the default port for the bootstrap server.
	// This follows the Tox protocol convention of using port 33445.
	BootstrapDefaultPort uint16 = 33445

	// AlicePortRangeStart is the starting port for test client "Alice".
	AlicePortRangeStart uint16 = 33500

	// AlicePortRangeEnd is the ending port for test client "Alice".
	AlicePortRangeEnd uint16 = 33599

	// BobPortRangeStart is the starting port for test client "Bob".
	BobPortRangeStart uint16 = 33600

	// BobPortRangeEnd is the ending port for test client "Bob".
	BobPortRangeEnd uint16 = 33699

	// OtherPortRangeStart is the starting port for other test clients.
	OtherPortRangeStart uint16 = 33700

	// OtherPortRangeEnd is the ending port for other test clients.
	OtherPortRangeEnd uint16 = 33799

	// MinValidPort is the minimum valid port number (excluding privileged ports).
	MinValidPort uint16 = 1024

	// MaxValidPort is the maximum valid port number.
	MaxValidPort uint16 = 65535
)

// ValidatePortRange checks if a port range is valid.
// Returns true if the range is valid, false otherwise.
func ValidatePortRange(startPort, endPort uint16) bool {
	// Check for valid port numbers
	if startPort < MinValidPort || endPort > MaxValidPort {
		return false
	}

	// Check for valid range (start must be less than or equal to end)
	if startPort > endPort {
		return false
	}

	return true
}

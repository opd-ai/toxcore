package toxcore

// Common test constants used across multiple test files.
// Consolidating these avoids magic values and duplication.

const (
	// testDefaultPort is the standard Tox protocol port used in tests.
	testDefaultPort = 33445

	// testAlternatePort is an alternate port used in various test scenarios.
	testAlternatePort = 12345

	// testLocalhost is the standard IPv4 loopback address.
	testLocalhost = "127.0.0.1"

	// testBootstrapNode is a well-known Tox bootstrap node address.
	testBootstrapNode = "node.tox.biribiri.org"

	// testBootstrapKey is the public key corresponding to testBootstrapNode.
	testBootstrapKey = "F404ABAA1C99A9D37D61AB54898F56793E1DEF8BD46B1038B9D822E8460FAB67"

	// testPublicKeyString is a 32-character string used when a public key
	// value is populated via copy() into a [32]byte.
	testPublicKeyString = "12345678901234567890123456789012"
)

// testSequentialPublicKey is a [32]byte public key with sequential byte values
// used widely across friend, message, and savedata tests.
var testSequentialPublicKey = [32]byte{
	1, 2, 3, 4, 5, 6, 7, 8, 9, 10,
	11, 12, 13, 14, 15, 16, 17, 18, 19, 20,
	21, 22, 23, 24, 25, 26, 27, 28, 29, 30,
	31, 32,
}

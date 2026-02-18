// Package limits provides centralized message size constants and validation functions
// for the Tox protocol. This package ensures consistent size enforcement across all
// components of the toxcore implementation.
//
// # Message Size Hierarchy
//
// The package defines a hierarchy of size limits that support different stages of
// message processing in the Tox protocol:
//
//   - MaxPlaintextMessage (1372 bytes): The Tox protocol limit for plaintext user messages.
//     This matches the original Tox specification and ensures compatibility with other
//     Tox clients.
//
//   - MaxEncryptedMessage (1388 bytes): The maximum size after NaCl box encryption.
//     This includes the plaintext plus the Poly1305 MAC tag (16 bytes) for authenticated
//     encryption.
//
//   - MaxStorageMessage (16384 bytes): The maximum for storage operations, which allows
//     for message padding to standard sizes (256, 1024, 4096, 16384 bytes) for traffic
//     analysis resistance.
//
//   - MaxProcessingBuffer (1MB): The absolute maximum for any operation. This prevents
//     memory exhaustion attacks and resource abuse.
//
// # Validation Functions
//
// Each validation function checks for empty messages and size limit violations:
//
//	err := limits.ValidatePlaintextMessage(message)
//	if err != nil {
//	    // Handle validation error (ErrMessageEmpty or ErrMessageTooLarge)
//	}
//
// For custom size limits, use the generic ValidateMessageSize function:
//
//	err := limits.ValidateMessageSize(data, 4096)
//
// # Error Types
//
// The package provides structured errors with context:
//
//   - ErrMessageEmpty: Returned when an empty or nil message is provided
//   - ErrMessageTooLarge: Returned when message exceeds the specified limit
//
// # Protocol Compliance
//
// These constants are derived from the official Tox protocol specification to ensure
// interoperability with other Tox implementations. The encryption overhead matches
// the golang.org/x/crypto/nacl/box.Overhead constant (16 bytes for Poly1305).
//
// # Security Considerations
//
// The MaxProcessingBuffer limit (1MB) provides defense against memory exhaustion
// attacks. All network-received data should be validated against this limit before
// further processing.
//
// The storage message limit supports privacy-preserving padding schemes used by the
// async messaging system to prevent traffic analysis attacks.
package limits

package crypto

import (
	"errors"
	"fmt"
	"strings"
)

// CipherSuite represents a complete cryptographic suite for Noise protocol
//
//export ToxCipherSuite
type CipherSuite struct {
	DH     string // "X25519", "P256", "P521"
	Cipher string // "ChaChaPoly", "AESGCM"
	Hash   string // "SHA256", "SHA512", "BLAKE2s"
	Name   string // Full cipher suite name
}

// Predefined cipher suites in order of preference (most secure first)
var (
	DefaultCipherSuite = CipherSuite{
		DH:     "X25519",
		Cipher: "ChaChaPoly",
		Hash:   "SHA256",
		Name:   "Noise_IK_25519_ChaChaPoly_SHA256",
	}
	
	AlternateCipherSuite = CipherSuite{
		DH:     "X25519",
		Cipher: "AESGCM",
		Hash:   "SHA256", 
		Name:   "Noise_IK_25519_AESGCM_SHA256",
	}
	
	FutureCipherSuite = CipherSuite{
		DH:     "X25519",
		Cipher: "ChaChaPoly",
		Hash:   "SHA512",
		Name:   "Noise_IK_25519_ChaChaPoly_SHA512",
	}
)

// SupportedCipherSuites lists all supported cipher suites
var SupportedCipherSuites = []CipherSuite{
	DefaultCipherSuite,
	AlternateCipherSuite,
	FutureCipherSuite,
}

// CipherSuiteNegotiator handles cipher suite selection between peers
//
//export ToxCipherSuiteNegotiator
type CipherSuiteNegotiator struct {
	LocalPreferences  []CipherSuite
	RemoteCapabilities []CipherSuite
	SelectedSuite     *CipherSuite
}

// NewCipherSuiteNegotiator creates a new cipher suite negotiator
//
//export ToxNewCipherSuiteNegotiator
func NewCipherSuiteNegotiator() *CipherSuiteNegotiator {
	return &CipherSuiteNegotiator{
		LocalPreferences: SupportedCipherSuites,
	}
}

// SetRemoteCapabilities sets the remote peer's supported cipher suites
//
//export ToxSetRemoteCapabilities
func (n *CipherSuiteNegotiator) SetRemoteCapabilities(remote []CipherSuite) {
	n.RemoteCapabilities = remote
}

// NegotiateCipherSuite selects the best mutually supported cipher suite
//
//export ToxNegotiateCipherSuite
func (n *CipherSuiteNegotiator) NegotiateCipherSuite() (*CipherSuite, error) {
	if len(n.RemoteCapabilities) == 0 {
		return nil, errors.New("no remote capabilities provided")
	}
	
	// Find the first local preference that is also supported remotely
	for _, local := range n.LocalPreferences {
		for _, remote := range n.RemoteCapabilities {
			if CipherSuitesEqual(local, remote) {
				n.SelectedSuite = &local
				return &local, nil
			}
		}
	}
	
	return nil, errors.New("no compatible cipher suite found")
}

// CipherSuitesEqual checks if two cipher suites are equivalent
//
//export ToxCipherSuitesEqual
func CipherSuitesEqual(a, b CipherSuite) bool {
	return a.DH == b.DH && a.Cipher == b.Cipher && a.Hash == b.Hash
}

// ParseCipherSuiteName parses a cipher suite name into components
//
//export ToxParseCipherSuiteName
func ParseCipherSuiteName(name string) (*CipherSuite, error) {
	// Expected format: "Noise_IK_25519_ChaChaPoly_SHA256"
	parts := strings.Split(name, "_")
	if len(parts) != 5 {
		return nil, fmt.Errorf("invalid cipher suite name format: %s", name)
	}
	
	if parts[0] != "Noise" || parts[1] != "IK" {
		return nil, fmt.Errorf("unsupported pattern: %s_%s", parts[0], parts[1])
	}
	
	return &CipherSuite{
		DH:     parts[2],
		Cipher: parts[3],
		Hash:   parts[4],
		Name:   name,
	}, nil
}

// SerializeCipherSuite converts a cipher suite to wire format
//
//export ToxSerializeCipherSuite
func SerializeCipherSuite(suite CipherSuite) []byte {
	return []byte(suite.Name)
}

// DeserializeCipherSuite converts wire format to cipher suite
//
//export ToxDeserializeCipherSuite
func DeserializeCipherSuite(data []byte) (*CipherSuite, error) {
	name := string(data)
	return ParseCipherSuiteName(name)
}

// ValidateCipherSuite checks if a cipher suite is supported and secure
//
//export ToxValidateCipherSuite
func ValidateCipherSuite(suite CipherSuite) error {
	// Check DH algorithms
	switch suite.DH {
	case "X25519":
		// Supported
	case "P256", "P521":
		// Future support
		return fmt.Errorf("DH algorithm %s not yet supported", suite.DH)
	default:
		return fmt.Errorf("unsupported DH algorithm: %s", suite.DH)
	}
	
	// Check cipher algorithms
	switch suite.Cipher {
	case "ChaChaPoly":
		// Supported
	case "AESGCM":
		// Supported
	default:
		return fmt.Errorf("unsupported cipher: %s", suite.Cipher)
	}
	
	// Check hash algorithms
	switch suite.Hash {
	case "SHA256":
		// Supported
	case "SHA512", "BLAKE2s":
		// Future support
		return fmt.Errorf("hash algorithm %s not yet supported", suite.Hash)
	default:
		return fmt.Errorf("unsupported hash algorithm: %s", suite.Hash)
	}
	
	return nil
}

// GetRecommendedCipherSuite returns the currently recommended cipher suite
//
//export ToxGetRecommendedCipherSuite
func GetRecommendedCipherSuite() CipherSuite {
	return DefaultCipherSuite
}

// IsCipherSuiteSecure checks if a cipher suite meets current security standards
//
//export ToxIsCipherSuiteSecure
func IsCipherSuiteSecure(suite CipherSuite) bool {
	// Only X25519 + ChaCha20-Poly1305 + SHA256 is currently considered secure
	// AESGCM is acceptable but less preferred
	switch suite.DH {
	case "X25519":
		switch suite.Cipher {
		case "ChaChaPoly", "AESGCM":
			switch suite.Hash {
			case "SHA256":
				return true
			}
		}
	}
	return false
}

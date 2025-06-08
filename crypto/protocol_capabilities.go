package crypto

import (
	"errors"
	"fmt"
)

// ProtocolVersion represents a semantic version for protocol capabilities.
//
//export ToxProtocolVersion
type ProtocolVersion struct {
	Major uint8 `json:"major"`
	Minor uint8 `json:"minor"`
	Patch uint8 `json:"patch"`
}

// String returns the string representation of the protocol version.
func (pv ProtocolVersion) String() string {
	return fmt.Sprintf("%d.%d.%d", pv.Major, pv.Minor, pv.Patch)
}

// Compare compares two protocol versions.
// Returns -1 if pv < other, 0 if pv == other, 1 if pv > other.
func (pv ProtocolVersion) Compare(other ProtocolVersion) int {
	if pv.Major != other.Major {
		if pv.Major < other.Major {
			return -1
		}
		return 1
	}
	if pv.Minor != other.Minor {
		if pv.Minor < other.Minor {
			return -1
		}
		return 1
	}
	if pv.Patch != other.Patch {
		if pv.Patch < other.Patch {
			return -1
		}
		return 1
	}
	return 0
}

// IsCompatibleWith checks if this version is compatible with another version.
// Compatible means same major version and this version >= other version.
func (pv ProtocolVersion) IsCompatibleWith(other ProtocolVersion) bool {
	return pv.Major == other.Major && pv.Compare(other) >= 0
}

// Protocol capability cipher suite references
var (
	// CipherChaCha20Poly1305 represents ChaCha20-Poly1305 AEAD cipher
	CipherChaCha20Poly1305 = DefaultCipherSuite
	// CipherAESGCM represents AES-256-GCM AEAD cipher  
	CipherAESGCM = AlternateCipherSuite
	// CipherLegacy represents the legacy NaCl box encryption
	CipherLegacy = CipherSuite{
		DH:     "X25519",
		Cipher: "Legacy",
		Hash:   "SHA256",
		Name:   "Legacy_NaCl_Box",
	}
)

// ProtocolCapabilities represents the cryptographic and protocol capabilities
// of a Tox client for protocol negotiation.
//
//export ToxProtocolCapabilities
type ProtocolCapabilities struct {
	// MinVersion is the minimum protocol version this client supports
	MinVersion ProtocolVersion `json:"min_version"`
	// MaxVersion is the maximum protocol version this client supports
	MaxVersion ProtocolVersion `json:"max_version"`
	// SupportedCiphers lists the encryption ciphers this client supports
	SupportedCiphers []CipherSuite `json:"supported_ciphers"`
	// NoiseSupported indicates if this client supports Noise protocol
	NoiseSupported bool `json:"noise_supported"`
	// LegacySupported indicates if this client supports legacy encryption
	LegacySupported bool `json:"legacy_supported"`
	// Extensions lists additional protocol extensions supported
	Extensions []string `json:"extensions,omitempty"`
}

// NewProtocolCapabilities creates default protocol capabilities for a Tox client.
//
//export ToxNewProtocolCapabilities
func NewProtocolCapabilities() *ProtocolCapabilities {
	return &ProtocolCapabilities{
		MinVersion: ProtocolVersion{Major: 1, Minor: 0, Patch: 0},
		MaxVersion: ProtocolVersion{Major: 2, Minor: 0, Patch: 0},
		SupportedCiphers: []CipherSuite{
			CipherChaCha20Poly1305,
			CipherAESGCM,
			CipherLegacy,
		},
		NoiseSupported:  true,
		LegacySupported: true,
		Extensions:      []string{},
	}
}

// SelectBestProtocol negotiates the best mutual protocol version and cipher
// between local and remote capabilities.
//
//export ToxSelectBestProtocol
func SelectBestProtocol(local, remote *ProtocolCapabilities) (ProtocolVersion, string, error) {
	if local == nil || remote == nil {
		return ProtocolVersion{}, "", errors.New("capabilities cannot be nil")
	}

	// Find the highest mutually supported protocol version
	var selectedVersion ProtocolVersion
	var versionFound bool

	// Check if we can find a compatible version
	// Start from the local max version and work down
	for major := local.MaxVersion.Major; major >= local.MinVersion.Major; major-- {
		for minor := local.MaxVersion.Minor; minor >= 0; minor-- {
			for patch := local.MaxVersion.Patch; patch >= 0; patch-- {
				candidateVersion := ProtocolVersion{Major: major, Minor: minor, Patch: patch}

				// Check if this version is within local range
				if candidateVersion.Compare(local.MinVersion) < 0 {
					continue
				}

				// Check if this version is compatible with remote
				if candidateVersion.Compare(remote.MinVersion) >= 0 &&
					candidateVersion.Compare(remote.MaxVersion) <= 0 {
					selectedVersion = candidateVersion
					versionFound = true
					break
				}
			}
			if versionFound {
				break
			}
		}
		if versionFound {
			break
		}
	}

	if !versionFound {
		return ProtocolVersion{}, "", errors.New("no compatible protocol version found")
	}

	// Select the best mutual cipher based on version
	var selectedCipher string

	// For version 2.x, prefer Noise protocol ciphers
	if selectedVersion.Major >= 2 && local.NoiseSupported && remote.NoiseSupported {
		// Find best mutual cipher for Noise
		preferredOrder := []CipherSuite{CipherChaCha20Poly1305, CipherAESGCM}

		for _, preferred := range preferredOrder {
			if containsCipher(local.SupportedCiphers, preferred) &&
				containsCipher(remote.SupportedCiphers, preferred) {
				selectedCipher = preferred.Name
				break
			}
		}
	}

	// Fallback to legacy cipher if no Noise cipher found
	if selectedCipher == "" {
		if local.LegacySupported && remote.LegacySupported &&
			containsCipher(local.SupportedCiphers, CipherLegacy) &&
			containsCipher(remote.SupportedCiphers, CipherLegacy) {
			selectedCipher = CipherLegacy.Name
		}
	}

	if selectedCipher == "" {
		return ProtocolVersion{}, "", errors.New("no compatible cipher found")
	}

	return selectedVersion, selectedCipher, nil
}

// containsCipher checks if a cipher is in the list of supported ciphers.
func containsCipher(ciphers []CipherSuite, target CipherSuite) bool {
	for _, cipher := range ciphers {
		if cipher == target {
			return true
		}
	}
	return false
}

// GetPreferredCipher returns the preferred cipher for a given protocol version.
//
//export ToxGetPreferredCipher
func GetPreferredCipher(version ProtocolVersion, capabilities *ProtocolCapabilities) string {
	if version.Major >= 2 && capabilities.NoiseSupported {
		// For Noise protocol, prefer ChaCha20-Poly1305
		if containsCipher(capabilities.SupportedCiphers, CipherChaCha20Poly1305) {
			return CipherChaCha20Poly1305.Name
		}
		if containsCipher(capabilities.SupportedCiphers, CipherAESGCM) {
			return CipherAESGCM.Name
		}
	}

	// Fallback to legacy
	if capabilities.LegacySupported {
		return CipherLegacy.Name
	}

	return ""
}

// IsNoiseProtocol checks if the given cipher uses Noise protocol.
//
//export ToxIsNoiseProtocol
func IsNoiseProtocol(cipher string) bool {
	return cipher == CipherChaCha20Poly1305.Name || cipher == CipherAESGCM.Name
}

// ValidateCapabilities validates that protocol capabilities are well-formed.
//
//export ToxValidateCapabilities
func ValidateCapabilities(capabilities *ProtocolCapabilities) error {
	if capabilities == nil {
		return errors.New("capabilities cannot be nil")
	}

	if capabilities.MinVersion.Compare(capabilities.MaxVersion) > 0 {
		return errors.New("minimum version cannot be greater than maximum version")
	}

	if len(capabilities.SupportedCiphers) == 0 {
		return errors.New("must support at least one cipher")
	}

	if !capabilities.NoiseSupported && !capabilities.LegacySupported {
		return errors.New("must support at least one protocol type")
	}

	return nil
}

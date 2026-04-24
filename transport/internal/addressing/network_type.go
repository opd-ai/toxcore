package addressing

import "strings"

const (
	NetworkIP   = "ip"
	NetworkTor  = "tor"
	NetworkI2P  = "i2p"
	NetworkNym  = "nym"
	NetworkLoki = "loki"

	OnionSuffix = ".onion"
	I2PSuffix   = ".i2p"
	NymSuffix   = ".nym"
	LokiSuffix  = ".loki"
)

// DetectNetworkType infers the transport network type from an address-like string.
func DetectNetworkType(address string) string {
	switch {
	case strings.Contains(address, OnionSuffix):
		return NetworkTor
	case strings.Contains(address, I2PSuffix):
		return NetworkI2P
	case strings.Contains(address, NymSuffix):
		return NetworkNym
	case strings.Contains(address, LokiSuffix):
		return NetworkLoki
	default:
		return NetworkIP
	}
}

// IsPrivacyAddress reports whether the given address-like string resolves to a privacy network.
func IsPrivacyAddress(address string) bool {
	return DetectNetworkType(address) != NetworkIP
}

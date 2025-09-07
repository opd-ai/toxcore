package transport

import (
	"fmt"
	"net"
	"strings"
	"sync"

	"github.com/sirupsen/logrus"
)

// AddressParser provides unified address parsing across multiple network types
type AddressParser interface {
	// Parse converts a string address to NetworkAddress
	Parse(address string) ([]NetworkAddress, error)

	// RegisterNetwork adds a network-specific parser
	RegisterNetwork(name string, parser NetworkParser)

	// GetSupportedNetworks returns list of supported network types
	GetSupportedNetworks() []string

	// Close cleans up parser resources
	Close() error
}

// NetworkParser handles address parsing for specific network types
type NetworkParser interface {
	// ParseAddress converts a string address to NetworkAddress
	ParseAddress(address string) (NetworkAddress, error)

	// ValidateAddress checks if the NetworkAddress is valid for this network
	ValidateAddress(addr NetworkAddress) error

	// CanParse returns true if this parser can handle the given address
	CanParse(address string) bool

	// GetNetworkType returns the network type this parser handles
	GetNetworkType() string
}

// MultiNetworkParser implements AddressParser with support for multiple networks
type MultiNetworkParser struct {
	parsers map[string]NetworkParser
	mutex   sync.RWMutex
	logger  *logrus.Entry
}

// NewMultiNetworkParser creates a new multi-network address parser
func NewMultiNetworkParser() *MultiNetworkParser {
	parser := &MultiNetworkParser{
		parsers: make(map[string]NetworkParser),
		logger:  logrus.WithField("component", "MultiNetworkParser"),
	}

	// Register default parsers
	parser.RegisterNetwork("ip", NewIPAddressParser())
	parser.RegisterNetwork("tor", NewTorAddressParser())
	parser.RegisterNetwork("i2p", NewI2PAddressParser())
	parser.RegisterNetwork("nym", NewNymAddressParser())

	parser.logger.WithField("registered_count", len(parser.parsers)).Info("Multi-network parser initialized")
	return parser
}

// Parse implements AddressParser.Parse
func (p *MultiNetworkParser) Parse(address string) ([]NetworkAddress, error) {
	p.mutex.RLock()
	defer p.mutex.RUnlock()

	p.logger.WithField("address", address).Info("Parsing address")

	var results []NetworkAddress
	var lastErr error

	// Try each parser to see which can handle this address
	for networkType, parser := range p.parsers {
		if parser.CanParse(address) {
			p.logger.WithFields(logrus.Fields{
				"address":      address,
				"network_type": networkType,
			}).Info("Found compatible parser")

			netAddr, err := parser.ParseAddress(address)
			if err != nil {
				p.logger.WithFields(logrus.Fields{
					"address":      address,
					"network_type": networkType,
					"error":        err,
				}).Error("Failed to parse address")
				lastErr = err
				continue
			}

			// Validate the parsed address
			if err := parser.ValidateAddress(netAddr); err != nil {
				p.logger.WithFields(logrus.Fields{
					"address":      address,
					"network_type": networkType,
					"error":        err,
				}).Error("Address validation failed")
				lastErr = err
				continue
			}

			results = append(results, netAddr)
			p.logger.WithFields(logrus.Fields{
				"address":          address,
				"network_type":     networkType,
				"resolved_address": netAddr.String(),
			}).Info("Address parsed successfully")
		}
	}

	if len(results) == 0 {
		if lastErr != nil {
			return nil, fmt.Errorf("failed to parse address %s: %w", address, lastErr)
		}
		return nil, fmt.Errorf("no parser found for address: %s", address)
	}

	p.logger.WithFields(logrus.Fields{
		"address":      address,
		"result_count": len(results),
	}).Info("Address parsing completed")

	return results, nil
}

// RegisterNetwork implements AddressParser.RegisterNetwork
func (p *MultiNetworkParser) RegisterNetwork(name string, parser NetworkParser) {
	p.mutex.Lock()
	defer p.mutex.Unlock()

	p.parsers[name] = parser
	p.logger.WithFields(logrus.Fields{
		"network_type": name,
		"parser":       fmt.Sprintf("%T", parser),
	}).Info("Registered network parser")
}

// GetSupportedNetworks implements AddressParser.GetSupportedNetworks
func (p *MultiNetworkParser) GetSupportedNetworks() []string {
	p.mutex.RLock()
	defer p.mutex.RUnlock()

	networks := make([]string, 0, len(p.parsers))
	for name := range p.parsers {
		networks = append(networks, name)
	}
	return networks
}

// Close implements AddressParser.Close
func (p *MultiNetworkParser) Close() error {
	p.mutex.Lock()
	defer p.mutex.Unlock()

	p.logger.WithField("parser_count", len(p.parsers)).Info("Closing multi-network parser")

	// Clear parsers
	p.parsers = make(map[string]NetworkParser)

	p.logger.Info("Multi-network parser closed successfully")
	return nil
}

// GetParser returns a specific network parser if it exists
func (p *MultiNetworkParser) GetParser(networkType string) (NetworkParser, bool) {
	p.mutex.RLock()
	defer p.mutex.RUnlock()

	parser, exists := p.parsers[networkType]
	return parser, exists
}

// IPAddressParser handles IPv4 and IPv6 address parsing
type IPAddressParser struct {
	logger *logrus.Entry
}

// NewIPAddressParser creates a new IP address parser
func NewIPAddressParser() *IPAddressParser {
	return &IPAddressParser{
		logger: logrus.WithField("component", "IPAddressParser"),
	}
}

// ParseAddress implements NetworkParser.ParseAddress for IP addresses
func (p *IPAddressParser) ParseAddress(address string) (NetworkAddress, error) {
	p.logger.WithField("address", address).Debug("Parsing IP address")

	host, port, err := net.SplitHostPort(address)
	if err != nil {
		return NetworkAddress{}, fmt.Errorf("invalid address format: %w", err)
	}

	// Resolve hostname to IP if needed
	ip := net.ParseIP(host)
	if ip == nil {
		// Try resolving hostname
		ips, err := net.LookupIP(host)
		if err != nil {
			return NetworkAddress{}, fmt.Errorf("failed to resolve hostname %s: %w", host, err)
		}
		if len(ips) == 0 {
			return NetworkAddress{}, fmt.Errorf("no IP addresses found for hostname: %s", host)
		}
		ip = ips[0] // Use first IP
		p.logger.WithFields(logrus.Fields{
			"hostname":    host,
			"resolved_ip": ip.String(),
		}).Debug("Hostname resolved to IP")
	}

	// Determine address type
	var addrType AddressType
	if ip.To4() != nil {
		addrType = AddressTypeIPv4
	} else {
		addrType = AddressTypeIPv6
	}

	// Convert port string to uint16
	portNum, err := net.LookupPort("tcp", port)
	if err != nil {
		return NetworkAddress{}, fmt.Errorf("invalid port number: %w", err)
	}

	netAddr := NetworkAddress{
		Type:    addrType,
		Data:    []byte(address),
		Port:    uint16(portNum),
		Network: "ip",
	}

	p.logger.WithFields(logrus.Fields{
		"original": address,
		"type":     addrType,
		"resolved": netAddr.String(),
	}).Debug("IP address parsed successfully")

	return netAddr, nil
}

// ValidateAddress implements NetworkParser.ValidateAddress for IP addresses
func (p *IPAddressParser) ValidateAddress(addr NetworkAddress) error {
	if addr.Type != AddressTypeIPv4 && addr.Type != AddressTypeIPv6 {
		return fmt.Errorf("invalid address type for IP parser: %s", addr.Type)
	}

	host, _, err := net.SplitHostPort(string(addr.Data))
	if err != nil {
		return fmt.Errorf("invalid IP address format: %w", err)
	}

	ip := net.ParseIP(host)
	if ip == nil {
		return fmt.Errorf("invalid IP address: %s", host)
	}

	return nil
}

// CanParse implements NetworkParser.CanParse for IP addresses
func (p *IPAddressParser) CanParse(address string) bool {
	// Check if it looks like an IP address or hostname:port
	host, _, err := net.SplitHostPort(address)
	if err != nil {
		return false
	}

	// Check for explicit privacy network domains
	if strings.Contains(host, ".onion") ||
		strings.Contains(host, ".i2p") ||
		strings.Contains(host, ".nym") ||
		strings.Contains(host, ".loki") {
		return false
	}

	// Can handle IP addresses and regular hostnames
	return true
}

// GetNetworkType implements NetworkParser.GetNetworkType
func (p *IPAddressParser) GetNetworkType() string {
	return "ip"
}

// TorAddressParser handles .onion address parsing
type TorAddressParser struct {
	logger *logrus.Entry
}

// NewTorAddressParser creates a new Tor address parser
func NewTorAddressParser() *TorAddressParser {
	return &TorAddressParser{
		logger: logrus.WithField("component", "TorAddressParser"),
	}
}

// ParseAddress implements NetworkParser.ParseAddress for Tor addresses
func (p *TorAddressParser) ParseAddress(address string) (NetworkAddress, error) {
	p.logger.WithField("address", address).Debug("Parsing Tor address")

	host, port, err := net.SplitHostPort(address)
	if err != nil {
		return NetworkAddress{}, fmt.Errorf("invalid Tor address format: %w", err)
	}

	if !strings.HasSuffix(host, ".onion") {
		return NetworkAddress{}, fmt.Errorf("invalid Tor address: must end with .onion")
	}

	// Convert port string to uint16
	portNum, err := net.LookupPort("tcp", port)
	if err != nil {
		return NetworkAddress{}, fmt.Errorf("invalid port number: %w", err)
	}

	netAddr := NetworkAddress{
		Type:    AddressTypeOnion,
		Data:    []byte(address),
		Port:    uint16(portNum),
		Network: "tor",
	}

	p.logger.WithFields(logrus.Fields{
		"original": address,
		"resolved": netAddr.String(),
	}).Debug("Tor address parsed successfully")

	return netAddr, nil
}

// ValidateAddress implements NetworkParser.ValidateAddress for Tor addresses
func (p *TorAddressParser) ValidateAddress(addr NetworkAddress) error {
	if addr.Type != AddressTypeOnion {
		return fmt.Errorf("invalid address type for Tor parser: %s", addr.Type)
	}

	host, _, err := net.SplitHostPort(string(addr.Data))
	if err != nil {
		return fmt.Errorf("invalid Tor address format: %w", err)
	}

	if !strings.HasSuffix(host, ".onion") {
		return fmt.Errorf("invalid Tor address: must end with .onion")
	}

	// Basic onion address length validation
	onionPart := strings.TrimSuffix(host, ".onion")
	if len(onionPart) != 16 && len(onionPart) != 56 {
		return fmt.Errorf("invalid onion address length: expected 16 or 56 characters, got %d", len(onionPart))
	}

	return nil
}

// CanParse implements NetworkParser.CanParse for Tor addresses
func (p *TorAddressParser) CanParse(address string) bool {
	host, _, err := net.SplitHostPort(address)
	if err != nil {
		return false
	}
	return strings.HasSuffix(host, ".onion")
}

// GetNetworkType implements NetworkParser.GetNetworkType
func (p *TorAddressParser) GetNetworkType() string {
	return "tor"
}

// I2PAddressParser handles .i2p address parsing
type I2PAddressParser struct {
	logger *logrus.Entry
}

// NewI2PAddressParser creates a new I2P address parser
func NewI2PAddressParser() *I2PAddressParser {
	return &I2PAddressParser{
		logger: logrus.WithField("component", "I2PAddressParser"),
	}
}

// ParseAddress implements NetworkParser.ParseAddress for I2P addresses
func (p *I2PAddressParser) ParseAddress(address string) (NetworkAddress, error) {
	p.logger.WithField("address", address).Debug("Parsing I2P address")

	host, port, err := net.SplitHostPort(address)
	if err != nil {
		return NetworkAddress{}, fmt.Errorf("invalid I2P address format: %w", err)
	}

	if !strings.HasSuffix(host, ".i2p") {
		return NetworkAddress{}, fmt.Errorf("invalid I2P address: must end with .i2p")
	}

	// Convert port string to uint16
	portNum, err := net.LookupPort("tcp", port)
	if err != nil {
		return NetworkAddress{}, fmt.Errorf("invalid port number: %w", err)
	}

	netAddr := NetworkAddress{
		Type:    AddressTypeI2P,
		Data:    []byte(address),
		Port:    uint16(portNum),
		Network: "i2p",
	}

	p.logger.WithFields(logrus.Fields{
		"original": address,
		"resolved": netAddr.String(),
	}).Debug("I2P address parsed successfully")

	return netAddr, nil
}

// ValidateAddress implements NetworkParser.ValidateAddress for I2P addresses
func (p *I2PAddressParser) ValidateAddress(addr NetworkAddress) error {
	if addr.Type != AddressTypeI2P {
		return fmt.Errorf("invalid address type for I2P parser: %s", addr.Type)
	}

	host, _, err := net.SplitHostPort(string(addr.Data))
	if err != nil {
		return fmt.Errorf("invalid I2P address format: %w", err)
	}

	if !strings.HasSuffix(host, ".i2p") {
		return fmt.Errorf("invalid I2P address: must end with .i2p")
	}

	return nil
}

// CanParse implements NetworkParser.CanParse for I2P addresses
func (p *I2PAddressParser) CanParse(address string) bool {
	host, _, err := net.SplitHostPort(address)
	if err != nil {
		return false
	}
	return strings.HasSuffix(host, ".i2p")
}

// GetNetworkType implements NetworkParser.GetNetworkType
func (p *I2PAddressParser) GetNetworkType() string {
	return "i2p"
}

// NymAddressParser handles .nym address parsing
type NymAddressParser struct {
	logger *logrus.Entry
}

// NewNymAddressParser creates a new Nym address parser
func NewNymAddressParser() *NymAddressParser {
	return &NymAddressParser{
		logger: logrus.WithField("component", "NymAddressParser"),
	}
}

// ParseAddress implements NetworkParser.ParseAddress for Nym addresses
func (p *NymAddressParser) ParseAddress(address string) (NetworkAddress, error) {
	p.logger.WithField("address", address).Debug("Parsing Nym address")

	host, port, err := net.SplitHostPort(address)
	if err != nil {
		return NetworkAddress{}, fmt.Errorf("invalid Nym address format: %w", err)
	}

	if !strings.HasSuffix(host, ".nym") {
		return NetworkAddress{}, fmt.Errorf("invalid Nym address: must end with .nym")
	}

	// Convert port string to uint16
	portNum, err := net.LookupPort("tcp", port)
	if err != nil {
		return NetworkAddress{}, fmt.Errorf("invalid port number: %w", err)
	}

	netAddr := NetworkAddress{
		Type:    AddressTypeNym,
		Data:    []byte(address),
		Port:    uint16(portNum),
		Network: "nym",
	}

	p.logger.WithFields(logrus.Fields{
		"original": address,
		"resolved": netAddr.String(),
	}).Debug("Nym address parsed successfully")

	return netAddr, nil
}

// ValidateAddress implements NetworkParser.ValidateAddress for Nym addresses
func (p *NymAddressParser) ValidateAddress(addr NetworkAddress) error {
	if addr.Type != AddressTypeNym {
		return fmt.Errorf("invalid address type for Nym parser: %s", addr.Type)
	}

	host, _, err := net.SplitHostPort(string(addr.Data))
	if err != nil {
		return fmt.Errorf("invalid Nym address format: %w", err)
	}

	if !strings.HasSuffix(host, ".nym") {
		return fmt.Errorf("invalid Nym address: must end with .nym")
	}

	return nil
}

// CanParse implements NetworkParser.CanParse for Nym addresses
func (p *NymAddressParser) CanParse(address string) bool {
	host, _, err := net.SplitHostPort(address)
	if err != nil {
		return false
	}
	return strings.HasSuffix(host, ".nym")
}

// GetNetworkType implements NetworkParser.GetNetworkType
func (p *NymAddressParser) GetNetworkType() string {
	return "nym"
}

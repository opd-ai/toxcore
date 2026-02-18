package transport

import (
	"testing"
)

// TestParseOnionAddress tests parsing of Tor .onion addresses.
func TestParseOnionAddress(t *testing.T) {
	tests := []struct {
		name         string
		addrStr      string
		network      string
		expectedHost string
		expectedPort uint16
	}{
		{
			name:         "onion with port",
			addrStr:      "3g2upl4pq6kufc4m.onion:8080",
			network:      "tcp",
			expectedHost: "3g2upl4pq6kufc4m.onion",
			expectedPort: 8080,
		},
		{
			name:         "onion without port",
			addrStr:      "3g2upl4pq6kufc4m.onion",
			network:      "tcp",
			expectedHost: "3g2upl4pq6kufc4m.onion",
			expectedPort: 0,
		},
		{
			name:         "v3 onion address with port",
			addrStr:      "pg6mmjiyjmcrsslvykfwnntlaru7p5svn6y2ymmju6nubxndf4pscryd.onion:443",
			network:      "tcp",
			expectedHost: "pg6mmjiyjmcrsslvykfwnntlaru7p5svn6y2ymmju6nubxndf4pscryd.onion",
			expectedPort: 443,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result, err := parseOnionAddress(tt.addrStr, tt.network)
			if err != nil {
				t.Fatalf("parseOnionAddress() error = %v", err)
			}

			if result.Type != AddressTypeOnion {
				t.Errorf("parseOnionAddress().Type = %v, want %v", result.Type, AddressTypeOnion)
			}

			if string(result.Data) != tt.expectedHost {
				t.Errorf("parseOnionAddress().Data = %v, want %v", string(result.Data), tt.expectedHost)
			}

			if result.Port != tt.expectedPort {
				t.Errorf("parseOnionAddress().Port = %v, want %v", result.Port, tt.expectedPort)
			}

			if result.Network != tt.network {
				t.Errorf("parseOnionAddress().Network = %v, want %v", result.Network, tt.network)
			}
		})
	}
}

// TestParseI2PAddress tests parsing of I2P .b32.i2p addresses.
func TestParseI2PAddress(t *testing.T) {
	tests := []struct {
		name         string
		addrStr      string
		network      string
		expectedHost string
		expectedPort uint16
	}{
		{
			name:         "i2p with port",
			addrStr:      "example.b32.i2p:7657",
			network:      "tcp",
			expectedHost: "example.b32.i2p",
			expectedPort: 7657,
		},
		{
			name:         "i2p without port",
			addrStr:      "abcdefghijklmnop.b32.i2p",
			network:      "tcp",
			expectedHost: "abcdefghijklmnop.b32.i2p",
			expectedPort: 0,
		},
		{
			name:         "full b32 address",
			addrStr:      "aaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaa.b32.i2p:4443",
			network:      "tcp",
			expectedHost: "aaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaa.b32.i2p",
			expectedPort: 4443,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result, err := parseI2PAddress(tt.addrStr, tt.network)
			if err != nil {
				t.Fatalf("parseI2PAddress() error = %v", err)
			}

			if result.Type != AddressTypeI2P {
				t.Errorf("parseI2PAddress().Type = %v, want %v", result.Type, AddressTypeI2P)
			}

			if string(result.Data) != tt.expectedHost {
				t.Errorf("parseI2PAddress().Data = %v, want %v", string(result.Data), tt.expectedHost)
			}

			if result.Port != tt.expectedPort {
				t.Errorf("parseI2PAddress().Port = %v, want %v", result.Port, tt.expectedPort)
			}

			if result.Network != tt.network {
				t.Errorf("parseI2PAddress().Network = %v, want %v", result.Network, tt.network)
			}
		})
	}
}

// TestParseNymAddress tests parsing of Nym .nym addresses.
func TestParseNymAddress(t *testing.T) {
	tests := []struct {
		name         string
		addrStr      string
		network      string
		expectedHost string
		expectedPort uint16
	}{
		{
			name:         "nym with port",
			addrStr:      "client.nym:1977",
			network:      "tcp",
			expectedHost: "client.nym",
			expectedPort: 1977,
		},
		{
			name:         "nym without port",
			addrStr:      "example-client.nym",
			network:      "tcp",
			expectedHost: "example-client.nym",
			expectedPort: 0,
		},
		{
			name:         "nym standard port",
			addrStr:      "mixnet-gateway.nym:9000",
			network:      "tcp",
			expectedHost: "mixnet-gateway.nym",
			expectedPort: 9000,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result, err := parseNymAddress(tt.addrStr, tt.network)
			if err != nil {
				t.Fatalf("parseNymAddress() error = %v", err)
			}

			if result.Type != AddressTypeNym {
				t.Errorf("parseNymAddress().Type = %v, want %v", result.Type, AddressTypeNym)
			}

			if string(result.Data) != tt.expectedHost {
				t.Errorf("parseNymAddress().Data = %v, want %v", string(result.Data), tt.expectedHost)
			}

			if result.Port != tt.expectedPort {
				t.Errorf("parseNymAddress().Port = %v, want %v", result.Port, tt.expectedPort)
			}

			if result.Network != tt.network {
				t.Errorf("parseNymAddress().Network = %v, want %v", result.Network, tt.network)
			}
		})
	}
}

// TestParseLokiAddress tests parsing of Lokinet .loki addresses.
func TestParseLokiAddress(t *testing.T) {
	tests := []struct {
		name         string
		addrStr      string
		network      string
		expectedHost string
		expectedPort uint16
	}{
		{
			name:         "loki with port",
			addrStr:      "example.loki:1234",
			network:      "tcp",
			expectedHost: "example.loki",
			expectedPort: 1234,
		},
		{
			name:         "loki without port",
			addrStr:      "service.loki",
			network:      "tcp",
			expectedHost: "service.loki",
			expectedPort: 0,
		},
		{
			name:         "loki snapp address with port",
			addrStr:      "dw68y1xhptqbhcm5s8aaaip6dbopykagig5q5u1za4c7pzxto77y.loki:8443",
			network:      "tcp",
			expectedHost: "dw68y1xhptqbhcm5s8aaaip6dbopykagig5q5u1za4c7pzxto77y.loki",
			expectedPort: 8443,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result, err := parseLokiAddress(tt.addrStr, tt.network)
			if err != nil {
				t.Fatalf("parseLokiAddress() error = %v", err)
			}

			if result.Type != AddressTypeLoki {
				t.Errorf("parseLokiAddress().Type = %v, want %v", result.Type, AddressTypeLoki)
			}

			if string(result.Data) != tt.expectedHost {
				t.Errorf("parseLokiAddress().Data = %v, want %v", string(result.Data), tt.expectedHost)
			}

			if result.Port != tt.expectedPort {
				t.Errorf("parseLokiAddress().Port = %v, want %v", result.Port, tt.expectedPort)
			}

			if result.Network != tt.network {
				t.Errorf("parseLokiAddress().Network = %v, want %v", result.Network, tt.network)
			}
		})
	}
}

// TestNetworkAddress_ToBytes tests serialization of network addresses.
func TestNetworkAddress_ToBytes_Parsing(t *testing.T) {
	tests := []struct {
		name      string
		addr      NetworkAddress
		minLen    int
		expectErr bool
	}{
		{
			name: "IPv4 address serialization",
			addr: NetworkAddress{
				Type:    AddressTypeIPv4,
				Data:    []byte{192, 168, 1, 1},
				Port:    8080,
				Network: "udp",
			},
			minLen:    6, // 4 IP + 2 port
			expectErr: false,
		},
		{
			name: "IPv6 address serialization",
			addr: NetworkAddress{
				Type:    AddressTypeIPv6,
				Data:    []byte{0x20, 0x01, 0x0d, 0xb8, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 1},
				Port:    443,
				Network: "tcp",
			},
			minLen:    18, // 16 IP + 2 port
			expectErr: false,
		},
		{
			name: "Onion address serialization fails",
			addr: NetworkAddress{
				Type:    AddressTypeOnion,
				Data:    []byte("test.onion"),
				Port:    8080,
				Network: "tcp",
			},
			expectErr: true, // Onion addresses not supported for byte serialization
		},
		{
			name: "IPv4 with insufficient data",
			addr: NetworkAddress{
				Type:    AddressTypeIPv4,
				Data:    []byte{192, 168},
				Port:    8080,
				Network: "udp",
			},
			expectErr: true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result, err := tt.addr.ToBytes()

			if tt.expectErr {
				if err == nil {
					t.Error("ToBytes() expected error, got nil")
				}
				return
			}

			if err != nil {
				t.Errorf("ToBytes() unexpected error: %v", err)
				return
			}

			if len(result) < tt.minLen {
				t.Errorf("ToBytes() length = %v, want at least %v", len(result), tt.minLen)
			}
		})
	}
}

// TestNetworkAddress_ValidateAddress tests address validation.
func TestNetworkAddress_ValidateAddress_Parsing(t *testing.T) {
	tests := []struct {
		name      string
		addr      NetworkAddress
		expectErr bool
		errMsg    string
	}{
		{
			name: "valid IPv4 public address",
			addr: NetworkAddress{
				Type: AddressTypeIPv4,
				Data: []byte{8, 8, 8, 8},
				Port: 8080,
			},
			expectErr: false,
		},
		{
			name: "valid Onion address",
			addr: NetworkAddress{
				Type: AddressTypeOnion,
				Data: []byte("valid.onion"),
				Port: 80,
			},
			expectErr: false,
		},
		{
			name: "valid I2P address",
			addr: NetworkAddress{
				Type: AddressTypeI2P,
				Data: []byte("valid.b32.i2p"),
				Port: 7657,
			},
			expectErr: false,
		},
		{
			name: "valid Nym address",
			addr: NetworkAddress{
				Type: AddressTypeNym,
				Data: []byte("client.nym"),
				Port: 1977,
			},
			expectErr: false,
		},
		{
			name: "valid Loki address",
			addr: NetworkAddress{
				Type: AddressTypeLoki,
				Data: []byte("service.loki"),
				Port: 443,
			},
			expectErr: false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			err := tt.addr.ValidateAddress()

			if tt.expectErr {
				if err == nil {
					t.Errorf("ValidateAddress() expected error containing %q, got nil", tt.errMsg)
				}
			} else {
				if err != nil {
					t.Errorf("ValidateAddress() unexpected error: %v", err)
				}
			}
		})
	}
}

// TestSerializeNetAddrToBytes_Parsing tests serialization from net.Addr.
func TestSerializeNetAddrToBytes_Parsing(t *testing.T) {
	tests := []struct {
		name      string
		addrStr   string
		network   string
		expectErr bool
		minLen    int
	}{
		{
			name:      "valid UDP IPv4",
			addrStr:   "192.168.1.1:8080",
			network:   "udp",
			expectErr: false,
			minLen:    6, // 4 IP + 2 port
		},
		{
			name:      "valid TCP IPv4",
			addrStr:   "10.0.0.1:443",
			network:   "tcp",
			expectErr: false,
			minLen:    6,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			addr := &parseTestAddr{network: tt.network, address: tt.addrStr}
			result, err := SerializeNetAddrToBytes(addr)

			if tt.expectErr {
				if err == nil {
					t.Error("SerializeNetAddrToBytes() expected error, got nil")
				}
			} else {
				if err != nil {
					t.Errorf("SerializeNetAddrToBytes() unexpected error: %v", err)
					return
				}
				if len(result) < tt.minLen {
					t.Errorf("SerializeNetAddrToBytes() length = %v, want at least %v", len(result), tt.minLen)
				}
			}
		})
	}
}

// parseTestAddr is a simple net.Addr implementation for testing.
type parseTestAddr struct {
	network string
	address string
}

func (a *parseTestAddr) Network() string { return a.network }
func (a *parseTestAddr) String() string  { return a.address }

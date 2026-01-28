package toxcore

import (
	"testing"
)

// TestProxyConfiguration tests configuring Tox with proxy options.
func TestProxyConfiguration(t *testing.T) {
	tests := []struct {
		name        string
		proxyConfig *ProxyOptions
		udpEnabled  bool
		tcpPort     uint16
		expectError bool
	}{
		{
			name: "SOCKS5 proxy with UDP",
			proxyConfig: &ProxyOptions{
				Type: ProxyTypeSOCKS5,
				Host: "127.0.0.1",
				Port: 9050,
			},
			udpEnabled:  true,
			tcpPort:     0,
			expectError: false,
		},
		{
			name: "SOCKS5 proxy with TCP",
			proxyConfig: &ProxyOptions{
				Type: ProxyTypeSOCKS5,
				Host: "127.0.0.1",
				Port: 9050,
			},
			udpEnabled:  false,
			tcpPort:     33445,
			expectError: false,
		},
		{
			name: "SOCKS5 proxy with authentication",
			proxyConfig: &ProxyOptions{
				Type:     ProxyTypeSOCKS5,
				Host:     "127.0.0.1",
				Port:     9050,
				Username: "testuser",
				Password: "testpass",
			},
			udpEnabled:  true,
			tcpPort:     0,
			expectError: false,
		},
		{
			name:        "No proxy configuration",
			proxyConfig: nil,
			udpEnabled:  true,
			tcpPort:     0,
			expectError: false,
		},
		{
			name: "Proxy type none",
			proxyConfig: &ProxyOptions{
				Type: ProxyTypeNone,
				Host: "127.0.0.1",
				Port: 9050,
			},
			udpEnabled:  true,
			tcpPort:     0,
			expectError: false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			options := NewOptions()
			options.UDPEnabled = tt.udpEnabled
			options.TCPPort = tt.tcpPort
			options.Proxy = tt.proxyConfig
			options.MinBootstrapNodes = 1 // For testing

			tox, err := New(options)

			if tt.expectError {
				if err == nil {
					t.Errorf("Expected error but got none")
				}
				return
			}

			if err != nil {
				t.Errorf("Unexpected error creating Tox instance: %v", err)
				return
			}

			if tox == nil {
				t.Errorf("Expected non-nil Tox instance")
				return
			}

			// Cleanup
			tox.Kill()
		})
	}
}

// TestProxyConfigurationPersistence tests that proxy settings can be reapplied when loading savedata.
// Note: Proxy settings are runtime configuration and are not persisted in savedata.
// This test verifies that we can recreate a Tox instance from savedata and then
// reapply the proxy configuration.
func TestProxyConfigurationPersistence(t *testing.T) {
	// Create Tox instance with proxy configuration
	options := NewOptions()
	options.UDPEnabled = true
	proxyConfig := &ProxyOptions{
		Type:     ProxyTypeSOCKS5,
		Host:     "127.0.0.1",
		Port:     9050,
		Username: "testuser",
		Password: "testpass",
	}
	options.Proxy = proxyConfig
	options.MinBootstrapNodes = 1

	tox1, err := New(options)
	if err != nil {
		t.Fatalf("Failed to create first Tox instance: %v", err)
	}

	// Get savedata
	savedata := tox1.GetSavedata()

	tox1.Kill()

	// Create new instance from savedata with proxy reapplied
	options2 := NewOptions()
	options2.SavedataType = SaveDataTypeToxSave
	options2.SavedataData = savedata
	options2.MinBootstrapNodes = 1
	// Proxy settings are runtime config - reapply them
	options2.Proxy = proxyConfig

	tox2, err := NewFromSavedata(options2, savedata)
	if err != nil {
		t.Fatalf("Failed to create Tox from savedata: %v", err)
	}

	// Verify proxy configuration was applied
	if tox2.options.Proxy == nil {
		t.Errorf("Expected proxy configuration to be applied")
	} else {
		if tox2.options.Proxy.Type != ProxyTypeSOCKS5 {
			t.Errorf("Expected proxy type SOCKS5, got %v", tox2.options.Proxy.Type)
		}
		if tox2.options.Proxy.Host != "127.0.0.1" {
			t.Errorf("Expected proxy host 127.0.0.1, got %s", tox2.options.Proxy.Host)
		}
		if tox2.options.Proxy.Port != 9050 {
			t.Errorf("Expected proxy port 9050, got %d", tox2.options.Proxy.Port)
		}
	}

	tox2.Kill()
}

// TestProxyWithBootstrap tests that proxy configuration doesn't break bootstrap.
func TestProxyWithBootstrap(t *testing.T) {
	// Note: This test doesn't actually connect to a proxy, it just verifies
	// that bootstrap logic works with proxy configuration present
	options := NewOptions()
	options.UDPEnabled = true
	options.Proxy = &ProxyOptions{
		Type: ProxyTypeSOCKS5,
		Host: "127.0.0.1",
		Port: 9050,
	}
	options.MinBootstrapNodes = 1

	tox, err := New(options)
	if err != nil {
		t.Fatalf("Failed to create Tox instance: %v", err)
	}
	defer tox.Kill()

	// Attempt bootstrap (will fail without actual proxy, but shouldn't crash)
	err = tox.Bootstrap("node.tox.biribiri.org", 33445, "F404ABAA1C99A9D37D61AB54898F56793E1DEF8BD46B1038B9D822E8460FAB67")
	
	// We expect this to work (no error from the API call itself)
	// The actual connection will fail without a real proxy, but that's okay for this test
	if err != nil {
		t.Logf("Bootstrap returned error (expected without real proxy): %v", err)
	}
}

// TestProxyOptionsValidation tests validation of proxy options.
func TestProxyOptionsValidation(t *testing.T) {
	tests := []struct {
		name        string
		proxyConfig *ProxyOptions
		shouldWork  bool
	}{
		{
			name: "Valid SOCKS5",
			proxyConfig: &ProxyOptions{
				Type: ProxyTypeSOCKS5,
				Host: "127.0.0.1",
				Port: 9050,
			},
			shouldWork: true,
		},
		{
			name: "HTTP proxy (unsupported)",
			proxyConfig: &ProxyOptions{
				Type: ProxyTypeHTTP,
				Host: "127.0.0.1",
				Port: 8080,
			},
			shouldWork: true, // Will fail gracefully, instance still created
		},
		{
			name: "Empty host",
			proxyConfig: &ProxyOptions{
				Type: ProxyTypeSOCKS5,
				Host: "",
				Port: 9050,
			},
			shouldWork: true, // Empty host will cause proxy creation to fail gracefully
		},
		{
			name: "Zero port",
			proxyConfig: &ProxyOptions{
				Type: ProxyTypeSOCKS5,
				Host: "127.0.0.1",
				Port: 0,
			},
			shouldWork: true, // Zero port will cause proxy creation to fail gracefully
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			options := NewOptions()
			options.UDPEnabled = true
			options.Proxy = tt.proxyConfig
			options.MinBootstrapNodes = 1

			tox, err := New(options)

			if !tt.shouldWork {
				if err == nil {
					t.Errorf("Expected error but got none")
					if tox != nil {
						tox.Kill()
					}
				}
				return
			}

			if err != nil {
				t.Errorf("Unexpected error: %v", err)
				return
			}

			if tox == nil {
				t.Errorf("Expected non-nil Tox instance")
				return
			}

			tox.Kill()
		})
	}
}

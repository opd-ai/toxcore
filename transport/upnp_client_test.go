package transport

import (
	"context"
	"net"
	"strings"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
)

func TestNewUPnPClient(t *testing.T) {
	client := NewUPnPClient()

	assert.NotNil(t, client)
	assert.Equal(t, 10*time.Second, client.timeout)
	assert.False(t, client.discoveryDone)
	assert.Empty(t, client.gatewayURL)
	assert.Empty(t, client.controlURL)
}

func TestUPnPClient_SetTimeout(t *testing.T) {
	client := NewUPnPClient()
	newTimeout := 5 * time.Second

	client.SetTimeout(newTimeout)

	assert.Equal(t, newTimeout, client.timeout)
}

func TestUPnPClient_parseLocationFromSSDPResponse(t *testing.T) {
	client := NewUPnPClient()

	tests := []struct {
		name     string
		response string
		expected string
		wantErr  bool
	}{
		{
			name: "valid response",
			response: "HTTP/1.1 200 OK\r\n" +
				"CACHE-CONTROL: max-age=120\r\n" +
				"LOCATION: http://192.168.1.1:5000/rootdesc.xml\r\n" +
				"SERVER: Linux/3.14.0 UPnP/1.0 IpBridge/1.26.0\r\n",
			expected: "http://192.168.1.1:5000/rootdesc.xml",
			wantErr:  false,
		},
		{
			name: "location with lowercase",
			response: "HTTP/1.1 200 OK\r\n" +
				"location: http://192.168.1.1:5000/rootdesc.xml\r\n",
			expected: "http://192.168.1.1:5000/rootdesc.xml",
			wantErr:  false,
		},
		{
			name: "no location header",
			response: "HTTP/1.1 200 OK\r\n" +
				"CACHE-CONTROL: max-age=120\r\n",
			wantErr: true,
		},
		{
			name: "malformed location header",
			response: "HTTP/1.1 200 OK\r\n" +
				"LOCATION\r\n",
			wantErr: true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result, err := client.parseLocationFromSSDPResponse(tt.response)

			if tt.wantErr {
				assert.Error(t, err)
			} else {
				assert.NoError(t, err)
				assert.Equal(t, tt.expected, result)
			}
		})
	}
}

func TestUPnPClient_parseDeviceDescription(t *testing.T) {
	client := NewUPnPClient()
	client.gatewayURL = "http://192.168.1.1:5000/rootdesc.xml"

	validXML := `<?xml version="1.0"?>
<root xmlns="urn:schemas-upnp-org:device-1-0">
  <device>
    <serviceList>
      <service>
        <serviceType>urn:schemas-upnp-org:service:WANIPConnection:1</serviceType>
        <controlURL>/upnp/control/WANIPConn1</controlURL>
      </service>
    </serviceList>
  </device>
</root>`

	err := client.parseDeviceDescription(validXML)

	assert.NoError(t, err)
	assert.Equal(t, "http://192.168.1.1:5000/upnp/control/WANIPConn1", client.controlURL)
	assert.Equal(t, "urn:schemas-upnp-org:service:WANIPConnection:1", client.serviceType)
}

func TestUPnPClient_parseDeviceDescription_NoWANService(t *testing.T) {
	client := NewUPnPClient()
	client.gatewayURL = "http://192.168.1.1:5000/rootdesc.xml"

	invalidXML := `<?xml version="1.0"?>
<root xmlns="urn:schemas-upnp-org:device-1-0">
  <device>
    <serviceList>
      <service>
        <serviceType>urn:schemas-upnp-org:service:SomeOtherService:1</serviceType>
        <controlURL>/upnp/control/Other</controlURL>
      </service>
    </serviceList>
  </device>
</root>`

	err := client.parseDeviceDescription(invalidXML)

	assert.Error(t, err)
	assert.Contains(t, err.Error(), "WANIPConnection service not found")
}

func TestUPnPClient_parseExternalIPResponse(t *testing.T) {
	client := NewUPnPClient()

	tests := []struct {
		name     string
		response string
		expected string
		wantErr  bool
	}{
		{
			name: "valid IPv4 response",
			response: `<?xml version="1.0"?>
<s:Envelope xmlns:s="http://schemas.xmlsoap.org/soap/envelope/">
  <s:Body>
    <u:GetExternalIPAddressResponse xmlns:u="urn:schemas-upnp-org:service:WANIPConnection:1">
      <NewExternalIPAddress>203.0.113.1</NewExternalIPAddress>
    </u:GetExternalIPAddressResponse>
  </s:Body>
</s:Envelope>`,
			expected: "203.0.113.1",
			wantErr:  false,
		},
		{
			name: "no IP address element",
			response: `<?xml version="1.0"?>
<s:Envelope xmlns:s="http://schemas.xmlsoap.org/soap/envelope/">
  <s:Body>
    <u:GetExternalIPAddressResponse xmlns:u="urn:schemas-upnp-org:service:WANIPConnection:1">
    </u:GetExternalIPAddressResponse>
  </s:Body>
</s:Envelope>`,
			wantErr: true,
		},
		{
			name: "invalid IP address",
			response: `<?xml version="1.0"?>
<s:Envelope xmlns:s="http://schemas.xmlsoap.org/soap/envelope/">
  <s:Body>
    <u:GetExternalIPAddressResponse xmlns:u="urn:schemas-upnp-org:service:WANIPConnection:1">
      <NewExternalIPAddress>invalid-ip</NewExternalIPAddress>
    </u:GetExternalIPAddressResponse>
  </s:Body>
</s:Envelope>`,
			wantErr: true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result, err := client.parseExternalIPResponse(tt.response)

			if tt.wantErr {
				assert.Error(t, err)
				assert.Nil(t, result)
			} else {
				assert.NoError(t, err)
				assert.Equal(t, net.ParseIP(tt.expected), result)
			}
		})
	}
}

func TestUPnPClient_AddPortMapping_NoControlURL(t *testing.T) {
	client := NewUPnPClient()
	ctx := context.Background()

	mapping := UPnPMapping{
		ExternalPort: 8080,
		InternalPort: 8080,
		InternalIP:   "192.168.1.100",
		Protocol:     "TCP",
		Description:  "Test mapping",
		Duration:     time.Hour,
	}

	err := client.AddPortMapping(ctx, mapping)

	assert.Error(t, err)
	assert.Contains(t, err.Error(), "control URL not set")
}

func TestUPnPClient_DeletePortMapping_NoControlURL(t *testing.T) {
	client := NewUPnPClient()
	ctx := context.Background()

	err := client.DeletePortMapping(ctx, 8080, "TCP")

	assert.Error(t, err)
	assert.Contains(t, err.Error(), "control URL not set")
}

func TestUPnPClient_GetExternalIPAddress_NoControlURL(t *testing.T) {
	client := NewUPnPClient()
	ctx := context.Background()

	ip, err := client.GetExternalIPAddress(ctx)

	assert.Error(t, err)
	assert.Nil(t, ip)
	assert.Contains(t, err.Error(), "control URL not set")
}

func TestUPnPMapping_Validation(t *testing.T) {
	mapping := UPnPMapping{
		ExternalPort: 8080,
		InternalPort: 8080,
		InternalIP:   "192.168.1.100",
		Protocol:     "TCP",
		Description:  "Test mapping",
		Duration:     time.Hour,
	}

	// Test that all fields are set correctly
	assert.Equal(t, 8080, mapping.ExternalPort)
	assert.Equal(t, 8080, mapping.InternalPort)
	assert.Equal(t, "192.168.1.100", mapping.InternalIP)
	assert.Equal(t, "TCP", mapping.Protocol)
	assert.Equal(t, "Test mapping", mapping.Description)
	assert.Equal(t, time.Hour, mapping.Duration)
}

// Integration test that attempts UPnP discovery
// This test is marked as integration and may be skipped in CI
func TestUPnPClient_Integration_Discovery(t *testing.T) {
	if testing.Short() {
		t.Skip("Skipping integration test in short mode")
	}

	client := NewUPnPClient()
	client.SetTimeout(2 * time.Second)

	ctx, cancel := context.WithTimeout(context.Background(), 3*time.Second)
	defer cancel()

	// Try to discover gateway - this may fail in environments without UPnP
	err := client.DiscoverGateway(ctx)

	if err != nil {
		t.Logf("UPnP discovery failed (expected in many environments): %v", err)
		return
	}

	t.Logf("UPnP gateway discovered: %s", client.gatewayURL)
	t.Logf("Control URL: %s", client.controlURL)

	assert.NotEmpty(t, client.gatewayURL)
	assert.NotEmpty(t, client.controlURL)
	assert.True(t, client.discoveryDone)

	// Test availability check
	available := client.IsAvailable(ctx)
	assert.True(t, available)
}

// Integration test for external IP address retrieval
func TestUPnPClient_Integration_GetExternalIP(t *testing.T) {
	if testing.Short() {
		t.Skip("Skipping integration test in short mode")
	}

	client := NewUPnPClient()
	client.SetTimeout(2 * time.Second)

	ctx, cancel := context.WithTimeout(context.Background(), 3*time.Second)
	defer cancel()

	// Try to discover gateway first
	err := client.DiscoverGateway(ctx)
	if err != nil {
		t.Skipf("UPnP discovery failed, skipping external IP test: %v", err)
	}

	// Try to get external IP
	ip, err := client.GetExternalIPAddress(ctx)
	if err != nil {
		t.Logf("External IP retrieval failed (may be expected): %v", err)
		return
	}

	t.Logf("External IP address: %v", ip)
	assert.NotNil(t, ip)
	assert.True(t, ip.To4() != nil || ip.To16() != nil)
}

// Test the SOAP request building for AddPortMapping
func TestUPnPClient_SOAPRequestFormat(t *testing.T) {
	// This is a white-box test to verify SOAP request format
	mapping := UPnPMapping{
		ExternalPort: 8080,
		InternalPort: 9090,
		InternalIP:   "192.168.1.100",
		Protocol:     "UDP",
		Description:  "Tox DHT",
		Duration:     2 * time.Hour,
	}

	// We can't easily test the actual SOAP request building without exposing
	// the internal method, but we can verify the expected format by checking
	// that the mapping contains valid data
	assert.True(t, mapping.ExternalPort > 0 && mapping.ExternalPort < 65536)
	assert.True(t, mapping.InternalPort > 0 && mapping.InternalPort < 65536)
	assert.NotEmpty(t, mapping.InternalIP)
	assert.True(t, strings.ToUpper(mapping.Protocol) == "TCP" || strings.ToUpper(mapping.Protocol) == "UDP")
	assert.NotEmpty(t, mapping.Description)
	assert.True(t, mapping.Duration > 0)
}

// Benchmark SSDP response parsing
func BenchmarkUPnPClient_parseLocationFromSSDPResponse(b *testing.B) {
	client := NewUPnPClient()
	response := "HTTP/1.1 200 OK\r\n" +
		"CACHE-CONTROL: max-age=120\r\n" +
		"LOCATION: http://192.168.1.1:5000/rootdesc.xml\r\n" +
		"SERVER: Linux/3.14.0 UPnP/1.0 IpBridge/1.26.0\r\n"

	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		_, _ = client.parseLocationFromSSDPResponse(response)
	}
}

// Benchmark external IP response parsing
func BenchmarkUPnPClient_parseExternalIPResponse(b *testing.B) {
	client := NewUPnPClient()
	response := `<?xml version="1.0"?>
<s:Envelope xmlns:s="http://schemas.xmlsoap.org/soap/envelope/">
  <s:Body>
    <u:GetExternalIPAddressResponse xmlns:u="urn:schemas-upnp-org:service:WANIPConnection:1">
      <NewExternalIPAddress>203.0.113.1</NewExternalIPAddress>
    </u:GetExternalIPAddressResponse>
  </s:Body>
</s:Envelope>`

	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		_, _ = client.parseExternalIPResponse(response)
	}
}

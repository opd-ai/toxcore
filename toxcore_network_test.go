package toxcore

import "testing"

func TestWrapWithProxyIfConfiguredFailsWhenSocks5UDPProxyIsRequired(t *testing.T) {
	transport := newMockUDPTransport()

	wrapped, err := wrapWithProxyIfConfigured(transport, &ProxyOptions{
		Type:            ProxyTypeSOCKS5,
		Host:            "127.0.0.1",
		Port:            1,
		UDPProxyEnabled: true,
	})

	if err == nil {
		t.Fatal("expected SOCKS5 UDP proxy setup error")
	}
	if wrapped != nil {
		t.Fatal("expected no transport on fatal SOCKS5 UDP proxy setup failure")
	}
}

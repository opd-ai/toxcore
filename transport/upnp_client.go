// Package transport implements network transport for the Tox protocol.
//
// This file implements a UPnP (Universal Plug and Play) client for automatic
// port mapping to improve NAT traversal in compatible routers.
package transport

import (
	"bufio"
	"context"
	"errors"
	"fmt"
	"io"
	"net"
	"net/http"
	"net/url"
	"strconv"
	"strings"
	"time"
)

// UPnPClient provides UPnP-based automatic port mapping functionality
type UPnPClient struct {
	timeout       time.Duration
	gatewayURL    string
	controlURL    string
	serviceType   string
	discoveryDone bool
}

// UPnPMapping represents a port mapping
type UPnPMapping struct {
	ExternalPort int
	InternalPort int
	InternalIP   string
	Protocol     string
	Description  string
	Duration     time.Duration
}

// NewUPnPClient creates a new UPnP client
func NewUPnPClient() *UPnPClient {
	return &UPnPClient{
		timeout: 10 * time.Second,
	}
}

// DiscoverGateway discovers UPnP-enabled gateway/router on the local network
func (uc *UPnPClient) DiscoverGateway(ctx context.Context) error {
	if uc.discoveryDone && uc.gatewayURL != "" {
		return nil // Already discovered
	}

	// Try SSDP discovery for Internet Gateway Device
	gatewayURL, err := uc.ssdpDiscover(ctx, "urn:schemas-upnp-org:device:InternetGatewayDevice:1")
	if err != nil {
		// Fallback to WANIPConnection service
		gatewayURL, err = uc.ssdpDiscover(ctx, "urn:schemas-upnp-org:service:WANIPConnection:1")
		if err != nil {
			return fmt.Errorf("failed to discover UPnP gateway: %w", err)
		}
	}

	uc.gatewayURL = gatewayURL
	uc.discoveryDone = true

	// Get device description to find control URL
	return uc.getDeviceDescription(ctx)
}

// ssdpDiscover performs SSDP (Simple Service Discovery Protocol) discovery
func (uc *UPnPClient) ssdpDiscover(ctx context.Context, serviceType string) (string, error) {
	// Create UDP connection for multicast
	conn, err := net.DialUDP("udp4", nil, &net.UDPAddr{
		IP:   net.IPv4(239, 255, 255, 250),
		Port: 1900,
	})
	if err != nil {
		return "", fmt.Errorf("failed to create UDP connection: %w", err)
	}
	defer conn.Close()

	// Set read timeout
	deadline, ok := ctx.Deadline()
	if ok {
		conn.SetDeadline(deadline)
	} else {
		conn.SetDeadline(time.Now().Add(uc.timeout))
	}

	// Send M-SEARCH request
	searchRequest := fmt.Sprintf(
		"M-SEARCH * HTTP/1.1\r\n"+
			"HOST: 239.255.255.250:1900\r\n"+
			"ST: %s\r\n"+
			"MAN: \"ssdp:discover\"\r\n"+
			"MX: 3\r\n\r\n",
		serviceType)

	if _, err := conn.Write([]byte(searchRequest)); err != nil {
		return "", fmt.Errorf("failed to send SSDP request: %w", err)
	}

	// Read response
	buffer := make([]byte, 2048)
	n, err := conn.Read(buffer)
	if err != nil {
		return "", fmt.Errorf("failed to read SSDP response: %w", err)
	}

	// Parse response to extract LOCATION header
	response := string(buffer[:n])
	return uc.parseLocationFromSSDPResponse(response)
}

// parseLocationFromSSDPResponse extracts the LOCATION URL from SSDP response
func (uc *UPnPClient) parseLocationFromSSDPResponse(response string) (string, error) {
	scanner := bufio.NewScanner(strings.NewReader(response))
	for scanner.Scan() {
		line := strings.TrimSpace(scanner.Text())
		if strings.HasPrefix(strings.ToUpper(line), "LOCATION:") {
			parts := strings.SplitN(line, ":", 2)
			if len(parts) == 2 {
				return strings.TrimSpace(parts[1]), nil
			}
		}
	}
	return "", errors.New("LOCATION header not found in SSDP response")
}

// getDeviceDescription fetches and parses the device description XML
func (uc *UPnPClient) getDeviceDescription(ctx context.Context) error {
	if uc.gatewayURL == "" {
		return errors.New("gateway URL not set")
	}

	client := &http.Client{Timeout: uc.timeout}
	req, err := http.NewRequestWithContext(ctx, "GET", uc.gatewayURL, nil)
	if err != nil {
		return fmt.Errorf("failed to create HTTP request: %w", err)
	}

	resp, err := client.Do(req)
	if err != nil {
		return fmt.Errorf("failed to fetch device description: %w", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		return fmt.Errorf("HTTP error: %s", resp.Status)
	}

	body, err := io.ReadAll(resp.Body)
	if err != nil {
		return fmt.Errorf("failed to read response body: %w", err)
	}

	// Parse XML to find WANIPConnection service control URL
	return uc.parseDeviceDescription(string(body))
}

// parseDeviceDescription extracts control URL and service type from device description XML
func (uc *UPnPClient) parseDeviceDescription(xml string) error {
	lines := strings.Split(xml, "\n")
	inWANService := false

	for _, line := range lines {
		line = strings.TrimSpace(line)

		if uc.checkWANServiceStart(line, &inWANService) {
			continue
		}

		if inWANService {
			if err := uc.tryExtractControlURL(line); err != nil {
				return err
			}
			if uc.controlURL != "" {
				return nil
			}
		}
	}

	return errors.New("WANIPConnection service not found in device description")
}

// checkWANServiceStart checks if the line marks the start of WAN service section.
func (uc *UPnPClient) checkWANServiceStart(line string, inWANService *bool) bool {
	if strings.Contains(line, "WANIPConnection") {
		*inWANService = true
		uc.serviceType = "urn:schemas-upnp-org:service:WANIPConnection:1"
		return true
	}
	return false
}

// tryExtractControlURL attempts to extract control URL from a line.
func (uc *UPnPClient) tryExtractControlURL(line string) error {
	if !strings.Contains(line, "<controlURL>") {
		return nil
	}

	controlPath, found := uc.extractControlPath(line)
	if !found {
		return nil
	}

	return uc.buildControlURL(controlPath)
}

// extractControlPath extracts the control path from XML line.
func (uc *UPnPClient) extractControlPath(line string) (string, bool) {
	start := strings.Index(line, "<controlURL>")
	end := strings.Index(line, "</controlURL>")

	if start == -1 || end == -1 {
		return "", false
	}

	start += len("<controlURL>")
	return line[start:end], true
}

// buildControlURL constructs the absolute control URL from path.
func (uc *UPnPClient) buildControlURL(controlPath string) error {
	baseURL, err := url.Parse(uc.gatewayURL)
	if err != nil {
		return fmt.Errorf("invalid gateway URL: %w", err)
	}

	controlURL, err := baseURL.Parse(controlPath)
	if err != nil {
		return fmt.Errorf("invalid control URL: %w", err)
	}

	uc.controlURL = controlURL.String()
	return nil
}

// AddPortMapping creates a new port mapping
func (uc *UPnPClient) AddPortMapping(ctx context.Context, mapping UPnPMapping) error {
	if uc.controlURL == "" {
		return errors.New("control URL not set - call DiscoverGateway first")
	}

	// Build SOAP envelope for AddPortMapping action
	soapAction := "urn:schemas-upnp-org:service:WANIPConnection:1#AddPortMapping"
	soapBody := fmt.Sprintf(`<?xml version="1.0"?>
<s:Envelope xmlns:s="http://schemas.xmlsoap.org/soap/envelope/" s:encodingStyle="http://schemas.xmlsoap.org/soap/encoding/">
<s:Body>
<u:AddPortMapping xmlns:u="urn:schemas-upnp-org:service:WANIPConnection:1">
<NewRemoteHost></NewRemoteHost>
<NewExternalPort>%d</NewExternalPort>
<NewProtocol>%s</NewProtocol>
<NewInternalPort>%d</NewInternalPort>
<NewInternalClient>%s</NewInternalClient>
<NewEnabled>1</NewEnabled>
<NewPortMappingDescription>%s</NewPortMappingDescription>
<NewLeaseDuration>%d</NewLeaseDuration>
</u:AddPortMapping>
</s:Body>
</s:Envelope>`,
		mapping.ExternalPort,
		strings.ToUpper(mapping.Protocol),
		mapping.InternalPort,
		mapping.InternalIP,
		mapping.Description,
		int(mapping.Duration.Seconds()))

	return uc.sendSOAPRequest(ctx, soapAction, soapBody)
}

// DeletePortMapping removes an existing port mapping
func (uc *UPnPClient) DeletePortMapping(ctx context.Context, externalPort int, protocol string) error {
	if uc.controlURL == "" {
		return errors.New("control URL not set - call DiscoverGateway first")
	}

	soapAction := "urn:schemas-upnp-org:service:WANIPConnection:1#DeletePortMapping"
	soapBody := fmt.Sprintf(`<?xml version="1.0"?>
<s:Envelope xmlns:s="http://schemas.xmlsoap.org/soap/envelope/" s:encodingStyle="http://schemas.xmlsoap.org/soap/encoding/">
<s:Body>
<u:DeletePortMapping xmlns:u="urn:schemas-upnp-org:service:WANIPConnection:1">
<NewRemoteHost></NewRemoteHost>
<NewExternalPort>%d</NewExternalPort>
<NewProtocol>%s</NewProtocol>
</u:DeletePortMapping>
</s:Body>
</s:Envelope>`,
		externalPort,
		strings.ToUpper(protocol))

	return uc.sendSOAPRequest(ctx, soapAction, soapBody)
}

// GetExternalIPAddress retrieves the external IP address from the gateway
func (uc *UPnPClient) GetExternalIPAddress(ctx context.Context) (net.IP, error) {
	if uc.controlURL == "" {
		return nil, errors.New("control URL not set - call DiscoverGateway first")
	}

	soapAction := "urn:schemas-upnp-org:service:WANIPConnection:1#GetExternalIPAddress"
	soapBody := `<?xml version="1.0"?>
<s:Envelope xmlns:s="http://schemas.xmlsoap.org/soap/envelope/" s:encodingStyle="http://schemas.xmlsoap.org/soap/encoding/">
<s:Body>
<u:GetExternalIPAddress xmlns:u="urn:schemas-upnp-org:service:WANIPConnection:1">
</u:GetExternalIPAddress>
</s:Body>
</s:Envelope>`

	response, err := uc.sendSOAPRequestWithResponse(ctx, soapAction, soapBody)
	if err != nil {
		return nil, err
	}

	// Parse response to extract IP address
	return uc.parseExternalIPResponse(response)
}

// sendSOAPRequest sends a SOAP request without expecting a response
func (uc *UPnPClient) sendSOAPRequest(ctx context.Context, soapAction, soapBody string) error {
	_, err := uc.sendSOAPRequestWithResponse(ctx, soapAction, soapBody)
	return err
}

// sendSOAPRequestWithResponse sends a SOAP request and returns the response
func (uc *UPnPClient) sendSOAPRequestWithResponse(ctx context.Context, soapAction, soapBody string) (string, error) {
	client := &http.Client{Timeout: uc.timeout}

	req, err := http.NewRequestWithContext(ctx, "POST", uc.controlURL, strings.NewReader(soapBody))
	if err != nil {
		return "", fmt.Errorf("failed to create SOAP request: %w", err)
	}

	req.Header.Set("Content-Type", "text/xml; charset=utf-8")
	req.Header.Set("SOAPAction", `"`+soapAction+`"`)
	req.Header.Set("Content-Length", strconv.Itoa(len(soapBody)))

	resp, err := client.Do(req)
	if err != nil {
		return "", fmt.Errorf("failed to send SOAP request: %w", err)
	}
	defer resp.Body.Close()

	body, err := io.ReadAll(resp.Body)
	if err != nil {
		return "", fmt.Errorf("failed to read SOAP response: %w", err)
	}

	if resp.StatusCode != http.StatusOK {
		return "", fmt.Errorf("SOAP request failed: %s - %s", resp.Status, string(body))
	}

	return string(body), nil
}

// parseExternalIPResponse extracts the external IP address from SOAP response
func (uc *UPnPClient) parseExternalIPResponse(response string) (net.IP, error) {
	// Look for NewExternalIPAddress element
	start := strings.Index(response, "<NewExternalIPAddress>")
	if start == -1 {
		return nil, errors.New("external IP address not found in response")
	}
	start += len("<NewExternalIPAddress>")

	end := strings.Index(response[start:], "</NewExternalIPAddress>")
	if end == -1 {
		return nil, errors.New("malformed external IP address in response")
	}

	ipStr := response[start : start+end]
	ip := net.ParseIP(ipStr)
	if ip == nil {
		return nil, fmt.Errorf("invalid IP address: %s", ipStr)
	}

	return ip, nil
}

// SetTimeout sets the timeout for UPnP operations
func (uc *UPnPClient) SetTimeout(timeout time.Duration) {
	uc.timeout = timeout
}

// IsAvailable checks if UPnP is available on the network
func (uc *UPnPClient) IsAvailable(ctx context.Context) bool {
	err := uc.DiscoverGateway(ctx)
	return err == nil
}

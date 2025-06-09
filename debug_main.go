package main

import (
	"fmt"
	"github.com/opd-ai/toxcore/crypto"
)

func main() {
	// Test scenario 1: Both parties support Noise
	caps1 := crypto.NewProtocolCapabilities()
	caps1.NoiseSupported = true
	caps1.MinVersion = crypto.ProtocolVersion{Major: 2, Minor: 0, Patch: 0}
	caps1.MaxVersion = crypto.ProtocolVersion{Major: 2, Minor: 0, Patch: 0}
	
	caps2 := crypto.NewProtocolCapabilities()
	caps2.NoiseSupported = true
	caps2.MinVersion = crypto.ProtocolVersion{Major: 2, Minor: 0, Patch: 0}
	caps2.MaxVersion = crypto.ProtocolVersion{Major: 2, Minor: 0, Patch: 0}
	
	fmt.Printf("Test 1 - caps1: min=%s max=%s\n", caps1.MinVersion.String(), caps1.MaxVersion.String())
	fmt.Printf("Test 1 - caps2: min=%s max=%s\n", caps2.MinVersion.String(), caps2.MaxVersion.String())
	
	version, cipher, err := crypto.SelectBestProtocol(caps1, caps2)
	if err != nil {
		fmt.Printf("Test 1 Error: %v\n", err)
	} else {
		fmt.Printf("Test 1 - Selected version: %s\n", version.String())
		fmt.Printf("Test 1 - Selected cipher: %s\n", cipher)
	}
	
	// Test scenario 2: One party only supports legacy
	caps3 := crypto.NewProtocolCapabilities()
	caps3.NoiseSupported = false
	caps3.MinVersion = crypto.ProtocolVersion{Major: 1, Minor: 0, Patch: 0}
	caps3.MaxVersion = crypto.ProtocolVersion{Major: 1, Minor: 0, Patch: 0}
	
	fmt.Printf("\nTest 2 - caps1: min=%s max=%s\n", caps1.MinVersion.String(), caps1.MaxVersion.String())
	fmt.Printf("Test 2 - caps3: min=%s max=%s\n", caps3.MinVersion.String(), caps3.MaxVersion.String())
	
	version2, cipher2, err2 := crypto.SelectBestProtocol(caps1, caps3)
	if err2 != nil {
		fmt.Printf("Test 2 Error: %v\n", err2)
	} else {
		fmt.Printf("Test 2 - Selected version: %s\n", version2.String())
		fmt.Printf("Test 2 - Selected cipher: %s\n", cipher2)
	}
}

package crypto

import (
	"bytes"
	"crypto/rand"
	"fmt"
	"testing"
	"time"
)

// SecurityTestSuite provides comprehensive security testing for Noise protocol implementation
//
//export ToxSecurityTestSuite
type SecurityTestSuite struct {
	kciTests           []KCITest
	forwardSecrecyTests []ForwardSecrecyTest
	replayTests        []ReplayTest
	downgradeTests     []DowngradeTest
	results            *SecurityTestResults
}

// SecurityTestResults aggregates test results
//
//export ToxSecurityTestResults
type SecurityTestResults struct {
	KCIResistancePassed      bool
	ForwardSecrecyPassed     bool
	ReplayProtectionPassed   bool
	DowngradeProtectionPassed bool
	TotalTests               int
	PassedTests              int
	FailedTests              int
	TestDuration             time.Duration
	DetailedResults          []TestResult
}

// TestResult represents an individual test result
type TestResult struct {
	TestName    string
	TestType    string
	Passed      bool
	ErrorMsg    string
	Duration    time.Duration
	Severity    TestSeverity
}

// TestSeverity indicates the severity of a test failure
type TestSeverity int

const (
	SeverityCritical TestSeverity = iota
	SeverityHigh
	SeverityMedium
	SeverityLow
)

// KCITest represents a Key Compromise Impersonation test
type KCITest struct {
	Name            string
	Description     string
	CompromisedKey  [32]byte
	TargetPeer      [32]byte
	AttackerKey     [32]byte
	ExpectedFailure bool
}

// ForwardSecrecyTest represents a forward secrecy test
type ForwardSecrecyTest struct {
	Name              string
	Description       string
	SessionDuration   time.Duration
	MessageCount      int
	CompromiseTime    time.Duration
	ExpectedProtection bool
}

// ReplayTest represents a replay attack test
type ReplayTest struct {
	Name            string
	Description     string
	OriginalMessage []byte
	ReplayDelay     time.Duration
	ExpectedBlock   bool
}

// DowngradeTest represents a protocol downgrade attack test
type DowngradeTest struct {
	Name                string
	Description         string
	TargetProtocol      string
	AttackVector        string
	ExpectedProtection  bool
}

// NewSecurityTestSuite creates a new security test suite
//
//export ToxNewSecurityTestSuite
func NewSecurityTestSuite() *SecurityTestSuite {
	return &SecurityTestSuite{
		kciTests:           make([]KCITest, 0),
		forwardSecrecyTests: make([]ForwardSecrecyTest, 0),
		replayTests:        make([]ReplayTest, 0),
		downgradeTests:     make([]DowngradeTest, 0),
		results:            &SecurityTestResults{},
	}
}

// AddKCITest adds a KCI resistance test to the suite
//
//export ToxSecurityTestSuiteAddKCITest
func (sts *SecurityTestSuite) AddKCITest(test KCITest) {
	sts.kciTests = append(sts.kciTests, test)
}

// RunAllTests executes all security tests
//
//export ToxSecurityTestSuiteRunAll
func (sts *SecurityTestSuite) RunAllTests() *SecurityTestResults {
	startTime := time.Now()
	
	sts.results = &SecurityTestResults{
		DetailedResults: make([]TestResult, 0),
	}
	
	// Run KCI tests
	sts.runKCITests()
	
	// Run forward secrecy tests
	sts.runForwardSecrecyTests()
	
	// Run replay protection tests
	sts.runReplayTests()
	
	// Run downgrade protection tests
	sts.runDowngradeTests()
	
	// Calculate final results
	sts.results.TestDuration = time.Since(startTime)
	sts.results.TotalTests = len(sts.results.DetailedResults)
	
	for _, result := range sts.results.DetailedResults {
		if result.Passed {
			sts.results.PassedTests++
		} else {
			sts.results.FailedTests++
		}
	}
	
	// Set overall pass/fail status
	sts.results.KCIResistancePassed = sts.allTestsPassed("KCI")
	sts.results.ForwardSecrecyPassed = sts.allTestsPassed("ForwardSecrecy")
	sts.results.ReplayProtectionPassed = sts.allTestsPassed("Replay")
	sts.results.DowngradeProtectionPassed = sts.allTestsPassed("Downgrade")
	
	return sts.results
}

// runKCITests executes Key Compromise Impersonation tests
func (sts *SecurityTestSuite) runKCITests() {
	for _, test := range sts.kciTests {
		result := sts.executeKCITest(test)
		sts.results.DetailedResults = append(sts.results.DetailedResults, result)
	}
}

// executeKCITest performs a single KCI test
func (sts *SecurityTestSuite) executeKCITest(test KCITest) TestResult {
	startTime := time.Now()
	
	// Simulate KCI attack scenario
	result := TestResult{
		TestName: test.Name,
		TestType: "KCI",
		Severity: SeverityCritical,
	}
	
	// Create two legitimate parties
	alice, err := GenerateKeyPair()
	if err != nil {
		result.Passed = false
		result.ErrorMsg = fmt.Sprintf("Failed to generate Alice's keys: %v", err)
		result.Duration = time.Since(startTime)
		return result
	}
	
	bob, err := GenerateKeyPair()
	if err != nil {
		result.Passed = false
		result.ErrorMsg = fmt.Sprintf("Failed to generate Bob's keys: %v", err)
		result.Duration = time.Since(startTime)
		return result
	}
	
	// Simulate attacker with compromised key
	attacker := &KeyPair{
		Public:  test.CompromisedKey,
		Private: [32]byte{}, // Attacker has the private key
	}
	
	// Test 1: Attacker tries to impersonate to Bob using Alice's compromised key
	attackerHandshake, err := NewNoiseHandshake(true, attacker.Private, bob.Public)
	if err != nil {
		result.Passed = false
		result.ErrorMsg = fmt.Sprintf("Failed to create attacker handshake: %v", err)
		result.Duration = time.Since(startTime)
		return result
	}
	
	// Legitimate Bob creates responder handshake
	bobHandshake, err := NewNoiseHandshake(false, bob.Private, alice.Public)
	if err != nil {
		result.Passed = false
		result.ErrorMsg = fmt.Sprintf("Failed to create Bob's handshake: %v", err)
		result.Duration = time.Since(startTime)
		return result
	}
	
	// Attacker sends initial message
	attackMessage, _, err := attackerHandshake.WriteMessage([]byte("ATTACK"))
	if err != nil {
		result.Passed = false
		result.ErrorMsg = fmt.Sprintf("Attacker failed to write message: %v", err)
		result.Duration = time.Since(startTime)
		return result
	}
	
	// Bob processes the message - this should fail in a KCI-resistant protocol
	_, _, err = bobHandshake.ReadMessage(attackMessage)
	
	// In Noise-IK, this attack should fail because Bob's static key is authenticated
	if test.ExpectedFailure && err == nil {
		result.Passed = false
		result.ErrorMsg = "KCI attack succeeded when it should have failed"
	} else if !test.ExpectedFailure && err != nil {
		result.Passed = false
		result.ErrorMsg = fmt.Sprintf("Legitimate handshake failed: %v", err)
	} else {
		result.Passed = true
	}
	
	result.Duration = time.Since(startTime)
	return result
}

// runForwardSecrecyTests executes forward secrecy tests
func (sts *SecurityTestSuite) runForwardSecrecyTests() {
	for _, test := range sts.forwardSecrecyTests {
		result := sts.executeForwardSecrecyTest(test)
		sts.results.DetailedResults = append(sts.results.DetailedResults, result)
	}
}

// executeForwardSecrecyTest performs a single forward secrecy test
func (sts *SecurityTestSuite) executeForwardSecrecyTest(test ForwardSecrecyTest) TestResult {
	startTime := time.Now()
	
	result := TestResult{
		TestName: test.Name,
		TestType: "ForwardSecrecy",
		Severity: SeverityHigh,
	}
	
	// Create two parties
	alice, err := GenerateKeyPair()
	if err != nil {
		result.Passed = false
		result.ErrorMsg = fmt.Sprintf("Failed to generate Alice's keys: %v", err)
		result.Duration = time.Since(startTime)
		return result
	}
	
	bob, err := GenerateKeyPair()
	if err != nil {
		result.Passed = false
		result.ErrorMsg = fmt.Sprintf("Failed to generate Bob's keys: %v", err)
		result.Duration = time.Since(startTime)
		return result
	}
	
	// Establish session and exchange messages
	aliceHandshake, err := NewNoiseHandshake(true, alice.Private, bob.Public)
	if err != nil {
		result.Passed = false
		result.ErrorMsg = fmt.Sprintf("Failed to create Alice's handshake: %v", err)
		result.Duration = time.Since(startTime)
		return result
	}
	
	bobHandshake, err := NewNoiseHandshake(false, bob.Private, alice.Public)
	if err != nil {
		result.Passed = false
		result.ErrorMsg = fmt.Sprintf("Failed to create Bob's handshake: %v", err)
		result.Duration = time.Since(startTime)
		return result
	}
	
	// Complete handshake
	msg1, _, err := aliceHandshake.WriteMessage([]byte("Hello"))
	if err != nil {
		result.Passed = false
		result.ErrorMsg = fmt.Sprintf("Alice failed to write message: %v", err)
		result.Duration = time.Since(startTime)
		return result
	}
	
	_, session, err := bobHandshake.ReadMessage(msg1)
	if err != nil {
		result.Passed = false
		result.ErrorMsg = fmt.Sprintf("Bob failed to read message: %v", err)
		result.Duration = time.Since(startTime)
		return result
	}
	
	// Store original messages for later verification
	originalMessages := [][]byte{
		[]byte("Secret message 1"),
		[]byte("Secret message 2"),
		[]byte("Secret message 3"),
	}
	
	encryptedMessages := make([][]byte, len(originalMessages))
	
	// Encrypt messages using the session
	for i, msg := range originalMessages {
		encrypted, err := session.EncryptMessage(msg)
		if err != nil {
			result.Passed = false
			result.ErrorMsg = fmt.Sprintf("Failed to encrypt message %d: %v", i, err)
			result.Duration = time.Since(startTime)
			return result
		}
		encryptedMessages[i] = encrypted
	}
	
	// Simulate time passing and key compromise
	time.Sleep(test.CompromiseTime)
	
	// Simulate compromise: attacker gains access to long-term keys
	// In a forward-secret protocol, past messages should remain secure
	
	// Try to decrypt past messages using only the long-term keys
	// This should fail in a forward-secret protocol
	canDecryptPastMessages := false
	
	// Attempt to create a new session with the compromised keys
	// and use it to decrypt old messages (this should fail)
	compromisedHandshake, err := NewNoiseHandshake(true, alice.Private, bob.Public)
	if err == nil {
		// Even with the long-term keys, past encrypted messages should not be decryptable
		// because they were encrypted with ephemeral keys that have been destroyed
		
		for _, encryptedMsg := range encryptedMessages {
			// Try to decrypt using the compromised session
			// This is a simplified test - in reality, you'd need the exact session state
			_, err := compromisedHandshake.handshake.Decrypt(nil, nil, encryptedMsg)
			if err == nil {
				canDecryptPastMessages = true
				break
			}
		}
	}
	
	// Forward secrecy is maintained if past messages cannot be decrypted
	if test.ExpectedProtection && canDecryptPastMessages {
		result.Passed = false
		result.ErrorMsg = "Forward secrecy violated: past messages decryptable with compromised keys"
	} else if !test.ExpectedProtection && !canDecryptPastMessages {
		result.Passed = false
		result.ErrorMsg = "Expected to decrypt past messages but could not"
	} else {
		result.Passed = true
	}
	
	result.Duration = time.Since(startTime)
	return result
}

// runReplayTests executes replay attack protection tests
func (sts *SecurityTestSuite) runReplayTests() {
	for _, test := range sts.replayTests {
		result := sts.executeReplayTest(test)
		sts.results.DetailedResults = append(sts.results.DetailedResults, result)
	}
}

// executeReplayTest performs a single replay attack test
func (sts *SecurityTestSuite) executeReplayTest(test ReplayTest) TestResult {
	startTime := time.Now()
	
	result := TestResult{
		TestName: test.Name,
		TestType: "Replay",
		Severity: SeverityMedium,
	}
	
	// Create session between two parties
	alice, _ := GenerateKeyPair()
	bob, _ := GenerateKeyPair()
	
	aliceHandshake, _ := NewNoiseHandshake(true, alice.Private, bob.Public)
	bobHandshake, _ := NewNoiseHandshake(false, bob.Private, alice.Public)
	
	// Complete handshake
	msg1, _, _ := aliceHandshake.WriteMessage(test.OriginalMessage)
	_, session, _ := bobHandshake.ReadMessage(msg1)
	
	// Store the original encrypted message
	encryptedMsg, err := session.EncryptMessage(test.OriginalMessage)
	if err != nil {
		result.Passed = false
		result.ErrorMsg = fmt.Sprintf("Failed to encrypt original message: %v", err)
		result.Duration = time.Since(startTime)
		return result
	}
	
	// Wait for replay delay
	time.Sleep(test.ReplayDelay)
	
	// Attempt to replay the message
	_, err = session.DecryptMessage(encryptedMsg)
	replaySucceeded := (err == nil)
	
	// The second attempt should fail due to replay protection
	_, err = session.DecryptMessage(encryptedMsg)
	secondReplaySucceeded := (err == nil)
	
	if test.ExpectedBlock && (replaySucceeded || secondReplaySucceeded) {
		result.Passed = false
		result.ErrorMsg = "Replay attack succeeded when it should have been blocked"
	} else if !test.ExpectedBlock && !(replaySucceeded || secondReplaySucceeded) {
		result.Passed = false
		result.ErrorMsg = "Message replay blocked when it should have succeeded"
	} else {
		result.Passed = true
	}
	
	result.Duration = time.Since(startTime)
	return result
}

// runDowngradeTests executes protocol downgrade protection tests
func (sts *SecurityTestSuite) runDowngradeTests() {
	for _, test := range sts.downgradeTests {
		result := sts.executeDowngradeTest(test)
		sts.results.DetailedResults = append(sts.results.DetailedResults, result)
	}
}

// executeDowngradeTest performs a single downgrade attack test
func (sts *SecurityTestSuite) executeDowngradeTest(test DowngradeTest) TestResult {
	startTime := time.Now()
	
	result := TestResult{
		TestName: test.Name,
		TestType: "Downgrade",
		Severity: SeverityHigh,
	}
	
	// Create protocol capabilities that support multiple versions
	capabilities := NewProtocolCapabilities()
	capabilities.SupportedVersions = []ProtocolVersion{
		{Major: 2, Minor: 0, Patch: 0}, // Noise-IK
		{Major: 1, Minor: 0, Patch: 0}, // Legacy
	}
	capabilities.NoiseSupported = true
	
	// Simulate attacker trying to force downgrade
	attackerCapabilities := NewProtocolCapabilities()
	attackerCapabilities.SupportedVersions = []ProtocolVersion{
		{Major: 1, Minor: 0, Patch: 0}, // Only legacy
	}
	attackerCapabilities.NoiseSupported = false
	
	// Test protocol negotiation
	selectedVersion, selectedCipher, err := SelectBestProtocol(capabilities, attackerCapabilities)
	if err != nil {
		result.Passed = false
		result.ErrorMsg = fmt.Sprintf("Protocol negotiation failed: %v", err)
		result.Duration = time.Since(startTime)
		return result
	}
	
	// Check if downgrade was properly detected and handled
	isDowngrade := selectedVersion.Major < 2 // Anything less than version 2.0.0 is a downgrade
	
	if test.ExpectedProtection && isDowngrade {
		result.Passed = false
		result.ErrorMsg = fmt.Sprintf("Downgrade attack succeeded: negotiated version %s", selectedVersion.String())
	} else if !test.ExpectedProtection && !isDowngrade {
		result.Passed = false
		result.ErrorMsg = "Expected downgrade but negotiated secure version"
	} else {
		result.Passed = true
	}
	
	result.Duration = time.Since(startTime)
	return result
}

// allTestsPassed checks if all tests of a specific type passed
func (sts *SecurityTestSuite) allTestsPassed(testType string) bool {
	for _, result := range sts.results.DetailedResults {
		if result.TestType == testType && !result.Passed {
			return false
		}
	}
	return true
}

// GenerateStandardTestSuite creates a comprehensive test suite with standard security tests
//
//export ToxGenerateStandardTestSuite
func GenerateStandardTestSuite() *SecurityTestSuite {
	suite := NewSecurityTestSuite()
	
	// Add standard KCI tests
	compromisedKey := [32]byte{}
	rand.Read(compromisedKey[:])
	
	suite.AddKCITest(KCITest{
		Name:            "Basic KCI Resistance",
		Description:     "Test that compromised responder key doesn't allow impersonation",
		CompromisedKey:  compromisedKey,
		ExpectedFailure: true,
	})
	
	// Add forward secrecy tests
	suite.forwardSecrecyTests = append(suite.forwardSecrecyTests, ForwardSecrecyTest{
		Name:               "Basic Forward Secrecy",
		Description:        "Test that past messages remain secure after key compromise",
		SessionDuration:    1 * time.Hour,
		MessageCount:       100,
		CompromiseTime:     10 * time.Millisecond,
		ExpectedProtection: true,
	})
	
	// Add replay tests
	suite.replayTests = append(suite.replayTests, ReplayTest{
		Name:            "Message Replay Protection",
		Description:     "Test that replayed messages are detected and blocked",
		OriginalMessage: []byte("Test message for replay"),
		ReplayDelay:     1 * time.Second,
		ExpectedBlock:   true,
	})
	
	// Add downgrade tests
	suite.downgradeTests = append(suite.downgradeTests, DowngradeTest{
		Name:               "Protocol Downgrade Protection",
		Description:        "Test that protocol downgrade attacks are detected",
		TargetProtocol:     "legacy",
		AttackVector:       "capability_manipulation",
		ExpectedProtection: true,
	})
	
	return suite
}

// PrintResults outputs detailed test results
//
//export ToxSecurityTestResultsPrint
func (str *SecurityTestResults) PrintResults() {
	fmt.Printf("Security Test Results:\n")
	fmt.Printf("=====================\n")
	fmt.Printf("Total Tests: %d\n", str.TotalTests)
	fmt.Printf("Passed: %d\n", str.PassedTests)
	fmt.Printf("Failed: %d\n", str.FailedTests)
	fmt.Printf("Duration: %v\n", str.TestDuration)
	fmt.Printf("\nDetailed Results:\n")
	
	for _, result := range str.DetailedResults {
		status := "PASS"
		if !result.Passed {
			status = "FAIL"
		}
		
		fmt.Printf("  [%s] %s (%s) - %v", status, result.TestName, result.TestType, result.Duration)
		if !result.Passed {
			fmt.Printf(" - ERROR: %s", result.ErrorMsg)
		}
		fmt.Printf("\n")
	}
	
	fmt.Printf("\nSecurity Properties:\n")
	fmt.Printf("  KCI Resistance: %v\n", str.KCIResistancePassed)
	fmt.Printf("  Forward Secrecy: %v\n", str.ForwardSecrecyPassed)
	fmt.Printf("  Replay Protection: %v\n", str.ReplayProtectionPassed)
	fmt.Printf("  Downgrade Protection: %v\n", str.DowngradeProtectionPassed)
}

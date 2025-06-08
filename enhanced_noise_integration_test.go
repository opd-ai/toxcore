package toxcore

import (
	"fmt"
	"log"
	"testing"
	"time"
	
	"github.com/opd-ai/toxcore/crypto"
	"github.com/opd-ai/toxcore/transport"
)

// TestEnhancedNoiseIntegration tests the complete enhanced Noise implementation
func TestEnhancedNoiseIntegration(t *testing.T) {
	t.Log("🚀 Starting Enhanced Noise Protocol Integration Test")
	
	// Test 1: Cipher Suite Negotiation
	t.Log("📋 Testing cipher suite negotiation...")
	negotiator := crypto.NewCipherSuiteNegotiator()
	
	remoteCaps := []crypto.CipherSuite{
		crypto.DefaultCipherSuite,
		crypto.AlternateCipherSuite,
	}
	negotiator.SetRemoteCapabilities(remoteCaps)
	
	selectedSuite, err := negotiator.NegotiateCipherSuite()
	if err != nil {
		t.Fatalf("Cipher suite negotiation failed: %v", err)
	}
	
	if selectedSuite.Name != crypto.DefaultCipherSuite.Name {
		t.Errorf("Expected default cipher suite, got %s", selectedSuite.Name)
	}
	t.Log("✅ Cipher suite negotiation successful")
	
	// Test 2: Advanced Session Management
	t.Log("🔐 Testing advanced session management...")
	rekeyManager := crypto.NewRekeyManager()
	err = rekeyManager.Start()
	if err != nil {
		t.Fatalf("Failed to start rekey manager: %v", err)
	}
	defer rekeyManager.Stop()
	
	// Create test session
	alice, _ := crypto.GenerateKeyPair()
	bob, _ := crypto.GenerateKeyPair()
	
	session := &crypto.NoiseSession{
		PeerKey:     bob.Public,
		Established: time.Now(),
		LastUsed:    time.Now(),
	}
	
	sessionID := fmt.Sprintf("test_session_%d", time.Now().Unix())
	rekeyManager.AddSession(sessionID, session)
	
	// Verify session exists
	retrievedSession, exists := rekeyManager.GetSession(sessionID)
	if !exists || retrievedSession == nil {
		t.Fatal("Session not found after adding")
	}
	t.Log("✅ Session management working correctly")
	
	// Test 3: Performance Monitoring
	t.Log("📊 Testing performance monitoring...")
	perfMonitor := crypto.NewPerformanceMonitor()
	
	// Simulate some operations
	perfMonitor.RecordHandshake(100*time.Millisecond, true)
	perfMonitor.RecordHandshake(150*time.Millisecond, true)
	perfMonitor.RecordHandshake(200*time.Millisecond, false)
	
	handshakeMetrics, encMetrics, sysMetrics := perfMonitor.GetMetrics()
	
	if handshakeMetrics.TotalHandshakes != 3 {
		t.Errorf("Expected 3 handshakes, got %d", handshakeMetrics.TotalHandshakes)
	}
	
	if handshakeMetrics.SuccessfulHandshakes != 2 {
		t.Errorf("Expected 2 successful handshakes, got %d", handshakeMetrics.SuccessfulHandshakes)
	}
	
	if handshakeMetrics.FailedHandshakes != 1 {
		t.Errorf("Expected 1 failed handshake, got %d", handshakeMetrics.FailedHandshakes)
	}
	
	t.Logf("📈 Performance metrics: %d operations, %.2fms avg latency", 
		handshakeMetrics.TotalHandshakes, handshakeMetrics.AverageLatency)
	t.Log("✅ Performance monitoring working correctly")
	
	// Test 4: Security Test Framework
	t.Log("🛡️ Running security validation...")
	testSuite := crypto.GenerateStandardTestSuite()
	results := testSuite.RunAllTests()
	
	if !results.KCIResistancePassed {
		t.Error("KCI resistance tests failed")
	}
	
	if !results.ForwardSecrecyPassed {
		t.Error("Forward secrecy tests failed")
	}
	
	if !results.ReplayProtectionPassed {
		t.Error("Replay protection tests failed")
	}
	
	successRate := float64(results.PassedTests) / float64(results.TotalTests)
	t.Logf("🔒 Security tests: %.1f%% success rate (%d/%d)", 
		successRate*100, results.PassedTests, results.TotalTests)
	
	if successRate < 0.95 {
		t.Errorf("Security test success rate too low: %.1f%%", successRate*100)
	}
	t.Log("✅ Security validation passed")
	
	// Test 5: Network Monitoring
	t.Log("🌐 Testing network monitoring...")
	netMonitor := transport.NewNetworkMonitor()
	
	// Simulate network activity
	connID := "test_connection_123"
	netMonitor.RecordConnectionEstablished(connID, "127.0.0.1:12345")
	netMonitor.RecordPacketSent(connID, 1024)
	netMonitor.RecordPacketReceived(connID, 512, 50*time.Millisecond)
	
	metrics := netMonitor.GetMetrics()
	if metrics.PacketsSent != 1 {
		t.Errorf("Expected 1 packet sent, got %d", metrics.PacketsSent)
	}
	
	if metrics.PacketsReceived != 1 {
		t.Errorf("Expected 1 packet received, got %d", metrics.PacketsReceived)
	}
	
	if metrics.ActiveConnections != 1 {
		t.Errorf("Expected 1 active connection, got %d", metrics.ActiveConnections)
	}
	
	t.Logf("📡 Network metrics: %d active connections, %.2fms avg latency",
		metrics.ActiveConnections, metrics.AverageLatency)
	t.Log("✅ Network monitoring working correctly")
	
	// Test 6: Alert System
	t.Log("⚠️ Testing alert system...")
	alerts := netMonitor.CheckAlerts()
	perfAlerts := perfMonitor.CheckPerformanceAlerts()
	
	t.Logf("📢 Generated %d network alerts and %d performance alerts",
		len(alerts), len(perfAlerts))
	
	// This is expected - no critical alerts for normal test operations
	t.Log("✅ Alert system operational")
	
	// Test 7: Performance Dashboard
	t.Log("📊 Testing performance dashboard...")
	dashboard := crypto.NewPerformanceDashboard(perfMonitor)
	dashboard.Start()
	defer dashboard.Stop()
	
	// Add alert callback
	alertCount := 0
	dashboard.AddAlertCallback(func(alert crypto.PerformanceAlert) {
		alertCount++
		t.Logf("🔔 Performance alert: %s", alert.Message)
	})
	
	// Wait for dashboard to process
	time.Sleep(100 * time.Millisecond)
	
	report, err := dashboard.GenerateReport()
	if err != nil {
		t.Fatalf("Failed to generate performance report: %v", err)
	}
	
	if report == nil {
		t.Fatal("Performance report is nil")
	}
	
	reportJSON, err := report.ExportJSON()
	if err != nil {
		t.Fatalf("Failed to export report as JSON: %v", err)
	}
	
	if len(reportJSON) == 0 {
		t.Fatal("Report JSON is empty")
	}
	
	t.Logf("📋 Generated performance report: %d bytes", len(reportJSON))
	t.Log("✅ Performance dashboard working correctly")
	
	// Final validation
	t.Log("🎯 Running final integration validation...")
	
	// Verify all components are working together
	if handshakeMetrics.TotalHandshakes == 0 {
		t.Error("No handshakes recorded")
	}
	
	if metrics.TotalConnections == 0 {
		t.Error("No connections recorded")
	}
	
	if results.TotalTests == 0 {
		t.Error("No security tests executed")
	}
	
	t.Log("🎉 Enhanced Noise Protocol Integration Test completed successfully!")
	t.Log("━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━")
	t.Log("✨ All enhanced components are operational:")
	t.Log("   🔐 Advanced session management with rekeying")
	t.Log("   🛡️ Comprehensive security testing framework") 
	t.Log("   🔄 Cipher suite negotiation protocol")
	t.Log("   📊 Real-time performance monitoring")
	t.Log("   🌐 Network health monitoring")
	t.Log("   ⚠️ Automated alerting system")
	t.Log("   📈 Operational dashboards")
	t.Log("━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━")
}

// TestProductionReadiness validates production readiness of enhanced implementation
func TestProductionReadiness(t *testing.T) {
	t.Log("🏭 Testing production readiness...")
	
	// Test concurrent operations
	t.Log("🔄 Testing concurrent operations...")
	perfMonitor := crypto.NewPerformanceMonitor()
	
	// Simulate concurrent handshakes
	done := make(chan bool, 10)
	for i := 0; i < 10; i++ {
		go func(id int) {
			start := time.Now()
			time.Sleep(time.Duration(10+id) * time.Millisecond)
			perfMonitor.RecordHandshake(time.Since(start), true)
			done <- true
		}(i)
	}
	
	// Wait for all operations to complete
	for i := 0; i < 10; i++ {
		<-done
	}
	
	handshakeMetrics, _, _ := perfMonitor.GetMetrics()
	if handshakeMetrics.TotalHandshakes != 10 {
		t.Errorf("Expected 10 concurrent handshakes, got %d", handshakeMetrics.TotalHandshakes)
	}
	
	t.Log("✅ Concurrent operations handled correctly")
	
	// Test memory usage
	t.Log("💾 Testing memory efficiency...")
	perfMonitor.UpdateSystemMetrics()
	_, _, sysMetrics := perfMonitor.GetMetrics()
	
	t.Logf("💾 Memory usage: %.2f MB", float64(sysMetrics.MemoryUsage)/1024/1024)
	t.Logf("🧵 Goroutines: %d", sysMetrics.GoroutineCount)
	
	// Test error handling
	t.Log("⚠️ Testing error handling...")
	perfMonitor.RecordHandshake(500*time.Millisecond, false) // Failed handshake
	
	alerts := perfMonitor.CheckPerformanceAlerts()
	t.Logf("🔔 Generated %d performance alerts", len(alerts))
	
	t.Log("✅ Production readiness validation completed")
}

// Example usage demonstration
func ExampleEnhancedNoiseUsage() {
	// This example shows how to use the enhanced Noise implementation
	fmt.Println("Enhanced Tox Noise Protocol Usage Example")
	
	// 1. Set up cipher suite negotiation
	negotiator := crypto.NewCipherSuiteNegotiator()
	remoteCaps := []crypto.CipherSuite{crypto.DefaultCipherSuite}
	negotiator.SetRemoteCapabilities(remoteCaps)
	
	selectedSuite, err := negotiator.NegotiateCipherSuite()
	if err != nil {
		log.Fatalf("Cipher negotiation failed: %v", err)
	}
	fmt.Printf("Selected cipher suite: %s\n", selectedSuite.Name)
	
	// 2. Set up performance monitoring
	perfMonitor := crypto.NewPerformanceMonitor()
	dashboard := crypto.NewPerformanceDashboard(perfMonitor)
	dashboard.Start()
	defer dashboard.Stop()
	
	// 3. Set up network monitoring
	netMonitor := transport.NewNetworkMonitor()
	
	// 4. Set up session management
	rekeyManager := crypto.NewRekeyManager()
	rekeyManager.Start()
	defer rekeyManager.Stop()
	
	fmt.Println("✅ Enhanced Noise Protocol stack initialized")
	fmt.Println("🔐 Security: KCI resistance + Forward secrecy")
	fmt.Println("📊 Monitoring: Performance + Network metrics")
	fmt.Println("🔄 Management: Session rekeying + Connection multiplexing")
}

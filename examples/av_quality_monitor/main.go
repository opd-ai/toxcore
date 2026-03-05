// Quality Monitoring Dashboard Demo
//
// This example demonstrates the real-time call quality monitoring and metrics
// aggregation capabilities of toxcore-go's ToxAV implementation.
//
// The demo simulates multiple concurrent calls with varying network conditions
// and displays aggregated quality metrics, showcasing how applications can
// monitor and respond to call quality issues in real-time.
package main

import (
	"fmt"
	"math/rand"
	"os"
	"os/signal"
	"syscall"
	"time"

	avpkg "github.com/opd-ai/toxcore/av"
	"github.com/sirupsen/logrus"
)

func main() {
	printHeader()
	logrus.SetLevel(logrus.WarnLevel)

	aggregator := initializeAggregator()
	if aggregator == nil {
		return
	}
	defer aggregator.Stop()

	callProfiles := createCallProfiles()
	stopChan := startCallSimulations(aggregator, callProfiles)

	waitForInterrupt()

	cleanupSimulations(aggregator, callProfiles, stopChan)
	displayFinalMetrics(aggregator)

	fmt.Println()
	fmt.Println("Demo completed. Thank you for trying ToxAV quality monitoring!")
}

func printHeader() {
	fmt.Println("═══════════════════════════════════════════════════════════")
	fmt.Println("  ToxAV Quality Monitoring Dashboard Demo")
	fmt.Println("═══════════════════════════════════════════════════════════")
	fmt.Println()
}

func initializeAggregator() *avpkg.MetricsAggregator {
	fmt.Println("📊 Initializing metrics aggregator...")
	aggregator := avpkg.NewMetricsAggregator(2 * time.Second)

	aggregator.OnReport(func(report avpkg.AggregatedReport) {
		displayReport(report)
	})

	if err := aggregator.Start(); err != nil {
		fmt.Printf("❌ Failed to start aggregator: %v\n", err)
		return nil
	}

	fmt.Println("✅ Metrics aggregator started")
	fmt.Println()
	return aggregator
}

type callProfile struct {
	friendNumber uint32
	name         string
	quality      avpkg.QualityLevel
	packetLoss   float64
	jitter       time.Duration
	bitrate      uint32
}

func createCallProfiles() []callProfile {
	return []callProfile{
		createExcellentProfile(),
		createGoodProfile(),
		createFairProfile(),
		createVariableProfile(),
	}
}

func createExcellentProfile() callProfile {
	return callProfile{101, "Alice (Excellent)", avpkg.QualityExcellent, 0.5, 15 * time.Millisecond, 128000}
}

func createGoodProfile() callProfile {
	return callProfile{102, "Bob (Good)", avpkg.QualityGood, 2.0, 40 * time.Millisecond, 96000}
}

func createFairProfile() callProfile {
	return callProfile{103, "Charlie (Fair)", avpkg.QualityFair, 5.5, 80 * time.Millisecond, 64000}
}

func createVariableProfile() callProfile {
	return callProfile{104, "Diana (Variable)", avpkg.QualityGood, 2.5, 35 * time.Millisecond, 128000}
}

func startCallSimulations(aggregator *avpkg.MetricsAggregator, profiles []callProfile) chan struct{} {
	fmt.Println("📞 Starting call simulations...")
	for _, profile := range profiles {
		aggregator.StartCallTracking(profile.friendNumber)
		fmt.Printf("   ✓ %s (Friend %d)\n", profile.name, profile.friendNumber)
	}
	fmt.Println()
	fmt.Println("📈 Monitoring call quality (press Ctrl+C to stop)...")
	fmt.Println()

	stopChan := make(chan struct{})
	for _, profile := range profiles {
		go simulateCallWithProfile(aggregator, profile, stopChan)
	}
	return stopChan
}

func waitForInterrupt() {
	sigChan := make(chan os.Signal, 1)
	signal.Notify(sigChan, os.Interrupt, syscall.SIGTERM)
	<-sigChan

	fmt.Println()
	fmt.Println("🛑 Stopping demo...")
}

func cleanupSimulations(aggregator *avpkg.MetricsAggregator, profiles []callProfile, stopChan chan struct{}) {
	close(stopChan)

	for _, profile := range profiles {
		aggregator.StopCallTracking(profile.friendNumber)
	}
}

func displayFinalMetrics(aggregator *avpkg.MetricsAggregator) {
	fmt.Println()
	fmt.Println("═══════════════════════════════════════════════════════════")
	fmt.Println("  Final System Metrics")
	fmt.Println("═══════════════════════════════════════════════════════════")
	systemMetrics := aggregator.GetSystemMetrics()
	displaySystemMetrics(systemMetrics)
}

func simulateCallWithProfile(aggregator *avpkg.MetricsAggregator, profile callProfile, stopChan chan struct{}) {
	simulateCall(aggregator, profile.friendNumber, struct {
		friendNumber uint32
		name         string
		quality      avpkg.QualityLevel
		packetLoss   float64
		jitter       time.Duration
		bitrate      uint32
	}{
		friendNumber: profile.friendNumber,
		name:         profile.name,
		quality:      profile.quality,
		packetLoss:   profile.packetLoss,
		jitter:       profile.jitter,
		bitrate:      profile.bitrate,
	}, stopChan)
}

// simulateCall simulates a call with realistic metric updates.
func simulateCall(aggregator *avpkg.MetricsAggregator, friendNumber uint32, profile struct {
	friendNumber uint32
	name         string
	quality      avpkg.QualityLevel
	packetLoss   float64
	jitter       time.Duration
	bitrate      uint32
}, stopChan chan struct{},
) {
	ticker := time.NewTicker(1 * time.Second)
	defer ticker.Stop()

	startTime := time.Now()
	packetsSent := uint64(0)
	packetsReceived := uint64(0)

	for {
		select {
		case <-stopChan:
			return
		case <-ticker.C:
			_, lossVariation := simulatePacketTransmission(&packetsSent, &packetsReceived, profile.packetLoss)
			jitterVariation := calculateJitterVariation(profile.jitter)
			quality, currentLoss, currentJitter := applyQualityVariation(profile, lossVariation, jitterVariation)
			metrics := buildCallMetrics(profile, quality, currentLoss, currentJitter, packetsSent, packetsReceived, startTime)
			aggregator.RecordMetrics(friendNumber, metrics)
		}
	}
}

// simulatePacketTransmission simulates network packet transmission with realistic loss patterns
func simulatePacketTransmission(packetsSent, packetsReceived *uint64, basePacketLoss float64) (uint64, float64) {
	newPackets := uint64(50 + rand.Intn(20))
	*packetsSent += newPackets

	lossVariation := basePacketLoss * (0.8 + rand.Float64()*0.4)
	packetsLost := uint64(float64(newPackets) * lossVariation / 100.0)
	*packetsReceived += (newPackets - packetsLost)

	return newPackets, lossVariation
}

// calculateJitterVariation adds realistic variation to jitter measurements
func calculateJitterVariation(baseJitter time.Duration) float64 {
	return float64(baseJitter) * (0.7 + rand.Float64()*0.6)
}

// applyQualityVariation applies quality degradation for variable network conditions
func applyQualityVariation(profile struct {
	friendNumber uint32
	name         string
	quality      avpkg.QualityLevel
	packetLoss   float64
	jitter       time.Duration
	bitrate      uint32
}, lossVariation, jitterVariation float64,
) (avpkg.QualityLevel, float64, time.Duration) {
	quality := profile.quality
	currentLoss := lossVariation
	currentJitter := time.Duration(jitterVariation)

	if profile.name == "Diana (Variable)" && rand.Intn(10) < 3 {
		quality = avpkg.QualityFair
		currentLoss = 6.0 + rand.Float64()*2.0
		currentJitter = 90 * time.Millisecond
	}

	return quality, currentLoss, currentJitter
}

// buildCallMetrics constructs a CallMetrics struct with current measurements
func buildCallMetrics(profile struct {
	friendNumber uint32
	name         string
	quality      avpkg.QualityLevel
	packetLoss   float64
	jitter       time.Duration
	bitrate      uint32
}, quality avpkg.QualityLevel, currentLoss float64, currentJitter time.Duration, packetsSent, packetsReceived uint64, startTime time.Time,
) avpkg.CallMetrics {
	return avpkg.CallMetrics{
		PacketLoss:      currentLoss,
		Jitter:          currentJitter,
		RoundTripTime:   currentJitter * 2,
		PacketsSent:     packetsSent,
		PacketsReceived: packetsReceived,
		AudioBitRate:    profile.bitrate / 2,
		VideoBitRate:    profile.bitrate / 2,
		NetworkQuality:  convertQualityToNetwork(quality),
		CallDuration:    time.Since(startTime),
		LastFrameAge:    time.Duration(50+rand.Intn(100)) * time.Millisecond,
		Quality:         quality,
		Timestamp:       time.Now(),
	}
}

// displayReport formats and displays an aggregated report.
func displayReport(report avpkg.AggregatedReport) {
	fmt.Println("┌─────────────────────────────────────────────────────────┐")
	fmt.Printf("│ Report: %s │\n", report.Timestamp.Format("15:04:05"))
	fmt.Println("├─────────────────────────────────────────────────────────┤")

	// System overview
	fmt.Printf("│ Active Calls:     %2d                                  │\n", report.SystemMetrics.ActiveCalls)
	fmt.Printf("│ Overall Quality:  %-11s                        │\n", qualityEmoji(report.OverallQuality))
	fmt.Printf("│ Avg Packet Loss:  %.2f%%                              │\n", report.SystemMetrics.AveragePacketLoss)
	fmt.Printf("│ Avg Jitter:       %v                            │\n", report.SystemMetrics.AverageJitter)

	if report.SystemMetrics.ActiveCalls > 0 {
		fmt.Println("├─────────────────────────────────────────────────────────┤")
		fmt.Println("│ Quality Distribution:                                   │")
		fmt.Printf("│   %s Excellent: %2d   %s Good: %2d   %s Fair: %2d   %s Poor: %2d │\n",
			"🟢", report.SystemMetrics.ExcellentCalls,
			"🟡", report.SystemMetrics.GoodCalls,
			"🟠", report.SystemMetrics.FairCalls,
			"🔴", report.SystemMetrics.PoorCalls)

		// Individual call details
		if len(report.CallReports) > 0 {
			fmt.Println("├─────────────────────────────────────────────────────────┤")
			fmt.Println("│ Per-Call Details:                                       │")
			for friendNumber, metrics := range report.CallReports {
				qualityStr := fmt.Sprintf("%-11s", qualityEmoji(metrics.Quality))
				durationStr := formatDuration(metrics.CallDuration)
				fmt.Printf("│   Friend %3d: %s Loss: %4.1f%% Dur: %s        │\n",
					friendNumber, qualityStr, metrics.PacketLoss, durationStr)
			}
		}
	}

	fmt.Println("└─────────────────────────────────────────────────────────┘")
	fmt.Println()
}

// displaySystemMetrics shows final system statistics.
func displaySystemMetrics(metrics avpkg.SystemMetrics) {
	fmt.Printf("Total Calls:      %d\n", metrics.TotalCalls)
	fmt.Printf("Active Calls:     %d\n", metrics.ActiveCalls)
	fmt.Printf("Avg Packet Loss:  %.2f%%\n", metrics.AveragePacketLoss)
	fmt.Printf("Avg Jitter:       %v\n", metrics.AverageJitter)
	fmt.Printf("Avg Bitrate:      %d bps\n", metrics.AverageBitrate)
	fmt.Println()
	fmt.Println("Quality Distribution:")
	fmt.Printf("  🟢 Excellent: %d\n", metrics.ExcellentCalls)
	fmt.Printf("  🟡 Good:      %d\n", metrics.GoodCalls)
	fmt.Printf("  🟠 Fair:      %d\n", metrics.FairCalls)
	fmt.Printf("  🔴 Poor:      %d\n", metrics.PoorCalls)
}

// qualityEmoji returns an emoji and text for quality level.
func qualityEmoji(quality avpkg.QualityLevel) string {
	switch quality {
	case avpkg.QualityExcellent:
		return "🟢 Excellent"
	case avpkg.QualityGood:
		return "🟡 Good"
	case avpkg.QualityFair:
		return "🟠 Fair"
	case avpkg.QualityPoor:
		return "🔴 Poor"
	case avpkg.QualityUnacceptable:
		return "❌ Unaccept."
	default:
		return "⚪ Unknown"
	}
}

// formatDuration formats a duration for display.
func formatDuration(d time.Duration) string {
	minutes := int(d.Minutes())
	seconds := int(d.Seconds()) % 60
	return fmt.Sprintf("%2dm%02ds", minutes, seconds)
}

// convertQualityToNetwork converts QualityLevel to NetworkQuality.
func convertQualityToNetwork(quality avpkg.QualityLevel) avpkg.NetworkQuality {
	switch quality {
	case avpkg.QualityExcellent:
		return avpkg.NetworkExcellent
	case avpkg.QualityGood:
		return avpkg.NetworkGood
	case avpkg.QualityFair:
		return avpkg.NetworkFair
	default:
		return avpkg.NetworkPoor
	}
}

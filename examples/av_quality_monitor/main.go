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
	fmt.Println("═══════════════════════════════════════════════════════════")
	fmt.Println("  ToxAV Quality Monitoring Dashboard Demo")
	fmt.Println("═══════════════════════════════════════════════════════════")
	fmt.Println()

	// Configure logging for cleaner demo output
	logrus.SetLevel(logrus.WarnLevel)

	// Create metrics aggregator with 2-second reporting
	fmt.Println("📊 Initializing metrics aggregator...")
	aggregator := avpkg.NewMetricsAggregator(2 * time.Second)

	// Set up report callback
	aggregator.OnReport(func(report avpkg.AggregatedReport) {
		displayReport(report)
	})

	// Start aggregator
	if err := aggregator.Start(); err != nil {
		fmt.Printf("❌ Failed to start aggregator: %v\n", err)
		return
	}
	defer aggregator.Stop()

	fmt.Println("✅ Metrics aggregator started")
	fmt.Println()

	// Simulate multiple calls with different quality profiles
	callProfiles := []struct {
		friendNumber uint32
		name         string
		quality      avpkg.QualityLevel
		packetLoss   float64
		jitter       time.Duration
		bitrate      uint32
	}{
		{
			friendNumber: 101,
			name:         "Alice (Excellent)",
			quality:      avpkg.QualityExcellent,
			packetLoss:   0.5,
			jitter:       15 * time.Millisecond,
			bitrate:      128000,
		},
		{
			friendNumber: 102,
			name:         "Bob (Good)",
			quality:      avpkg.QualityGood,
			packetLoss:   2.0,
			jitter:       40 * time.Millisecond,
			bitrate:      96000,
		},
		{
			friendNumber: 103,
			name:         "Charlie (Fair)",
			quality:      avpkg.QualityFair,
			packetLoss:   5.5,
			jitter:       80 * time.Millisecond,
			bitrate:      64000,
		},
		{
			friendNumber: 104,
			name:         "Diana (Variable)",
			quality:      avpkg.QualityGood,
			packetLoss:   2.5,
			jitter:       35 * time.Millisecond,
			bitrate:      128000,
		},
	}

	// Start tracking all calls
	fmt.Println("📞 Starting call simulations...")
	for _, profile := range callProfiles {
		aggregator.StartCallTracking(profile.friendNumber)
		fmt.Printf("   ✓ %s (Friend %d)\n", profile.name, profile.friendNumber)
	}
	fmt.Println()
	fmt.Println("📈 Monitoring call quality (press Ctrl+C to stop)...")
	fmt.Println()

	// Start metric simulation goroutines
	stopChan := make(chan struct{})
	for _, profile := range callProfiles {
		go simulateCall(aggregator, profile.friendNumber, profile, stopChan)
	}

	// Wait for interrupt signal
	sigChan := make(chan os.Signal, 1)
	signal.Notify(sigChan, os.Interrupt, syscall.SIGTERM)
	<-sigChan

	fmt.Println()
	fmt.Println("🛑 Stopping demo...")

	// Stop simulations
	close(stopChan)

	// Stop tracking calls
	for _, profile := range callProfiles {
		aggregator.StopCallTracking(profile.friendNumber)
	}

	// Final system metrics
	fmt.Println()
	fmt.Println("═══════════════════════════════════════════════════════════")
	fmt.Println("  Final System Metrics")
	fmt.Println("═══════════════════════════════════════════════════════════")
	systemMetrics := aggregator.GetSystemMetrics()
	displaySystemMetrics(systemMetrics)

	fmt.Println()
	fmt.Println("Demo completed. Thank you for trying ToxAV quality monitoring!")
}

// simulateCall simulates a call with realistic metric updates.
func simulateCall(aggregator *avpkg.MetricsAggregator, friendNumber uint32, profile struct {
	friendNumber uint32
	name         string
	quality      avpkg.QualityLevel
	packetLoss   float64
	jitter       time.Duration
	bitrate      uint32
}, stopChan chan struct{}) {

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
			// Simulate packet transmission
			newPackets := uint64(50 + rand.Intn(20)) // 50-70 packets per second
			packetsSent += newPackets

			// Calculate received packets based on loss rate
			lossVariation := profile.packetLoss * (0.8 + rand.Float64()*0.4) // ±20% variation
			packetsLost := uint64(float64(newPackets) * lossVariation / 100.0)
			packetsReceived += (newPackets - packetsLost)

			// Add jitter variation (±30%)
			jitterVariation := float64(profile.jitter) * (0.7 + rand.Float64()*0.6)

			// For "Variable" quality profile, occasionally degrade
			quality := profile.quality
			currentLoss := lossVariation
			currentJitter := time.Duration(jitterVariation)

			if profile.name == "Diana (Variable)" {
				if rand.Intn(10) < 3 { // 30% chance of degradation
					quality = avpkg.QualityFair
					currentLoss = 6.0 + rand.Float64()*2.0
					currentJitter = 90 * time.Millisecond
				}
			}

			// Create metrics
			metrics := avpkg.CallMetrics{
				PacketLoss:      currentLoss,
				Jitter:          currentJitter,
				RoundTripTime:   currentJitter * 2, // Approximate RTT
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

			// Record metrics
			aggregator.RecordMetrics(friendNumber, metrics)
		}
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

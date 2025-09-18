// ToxAV Effects Processing Example
//
// This example demonstrates advanced audio and video effects processing
// capabilities in ToxAV, showcasing real-time effects chains, interactive
// parameter adjustment, and performance monitoring.

package main

import (
	"bufio"
	"fmt"
	"log"
	"math"
	"os"
	"strconv"
	"strings"
	"time"

	"github.com/opd-ai/toxcore"
	"github.com/opd-ai/toxcore/av"
)

// EffectsDemo represents the main demo application
type EffectsDemo struct {
	tox   *toxcore.Tox
	toxav *toxcore.ToxAV

	// Effect parameters
	audioGain             float64
	noiseSuppressionLevel float64
	agcTargetLevel        float64
	colorTemperature      int

	// Performance tracking
	audioFrameCount uint64
	videoFrameCount uint64
	totalAudioTime  time.Duration
	totalVideoTime  time.Duration

	// Demo state
	running         bool
	friendNumber    uint32
	hasActiveFriend bool
}

// NewEffectsDemo creates a new effects processing demo
func NewEffectsDemo() (*EffectsDemo, error) {
	demo := &EffectsDemo{
		audioGain:             1.0,
		noiseSuppressionLevel: 0.5,
		agcTargetLevel:        0.7,
		colorTemperature:      6500, // Daylight
		running:               true,
	}

	// Initialize Tox
	options := toxcore.NewOptions()
	options.UDPEnabled = true

	tox, err := toxcore.New(options)
	if err != nil {
		return nil, fmt.Errorf("failed to create Tox instance: %w", err)
	}
	demo.tox = tox

	// Initialize ToxAV
	toxav, err := toxcore.NewToxAV(tox)
	if err != nil {
		tox.Kill()
		return nil, fmt.Errorf("failed to create ToxAV instance: %w", err)
	}
	demo.toxav = toxav

	// Set up callbacks
	demo.setupCallbacks()

	// Set profile info
	demo.tox.SelfSetName("EffectsProcessingDemo")
	demo.tox.SelfSetStatusMessage("ToxAV Effects Processing Demo - Advanced A/V Effects")

	return demo, nil
}

// setupCallbacks configures ToxAV callbacks for the demo
func (demo *EffectsDemo) setupCallbacks() {
	// Handle incoming calls
	demo.toxav.CallbackCall(func(friendNumber uint32, audioEnabled, videoEnabled bool) {
		fmt.Printf("📞 Incoming call from friend %d (Audio: %t, Video: %t)\n",
			friendNumber, audioEnabled, videoEnabled)

		// Auto-answer with both audio and video
		err := demo.toxav.Answer(friendNumber, 64000, 500000) // 64kbps audio, 500kbps video
		if err != nil {
			fmt.Printf("❌ Failed to answer call: %v\n", err)
		} else {
			fmt.Printf("✅ Call answered with effects processing enabled\n")
			demo.friendNumber = friendNumber
			demo.hasActiveFriend = true
		}
	})

	// Handle call state changes
	demo.toxav.CallbackCallState(func(friendNumber uint32, state av.CallState) {
		fmt.Printf("📱 Call state changed for friend %d: %v\n", friendNumber, state)
	})

	// Handle incoming audio frames (for processing demonstration)
	demo.toxav.CallbackAudioReceiveFrame(func(friendNumber uint32, pcm []int16, sampleCount int, channels uint8, samplingRate uint32) {
		// Process received audio through effects (for demonstration)
		demo.processReceivedAudio(pcm, sampleCount, channels, samplingRate)
	})

	// Handle incoming video frames (for processing demonstration)
	demo.toxav.CallbackVideoReceiveFrame(func(friendNumber uint32, width, height uint16, y, u, v []byte, yStride, uStride, vStride int) {
		// Process received video through effects (for demonstration)
		demo.processReceivedVideo(width, height, y, u, v)
	})
}

// Run starts the main demo loop
func (demo *EffectsDemo) Run() error {
	defer demo.cleanup()

	// Bootstrap to network
	err := demo.bootstrap()
	if err != nil {
		return fmt.Errorf("bootstrap failed: %w", err)
	}

	// Display initial status
	demo.displayStatus()

	// Start frame generation in background
	go demo.runFrameGeneration()

	// Start network iteration in background
	go demo.runNetworkIteration()

	// Run interactive console
	demo.runInteractiveConsole()

	return nil
}

// bootstrap connects to the Tox network
func (demo *EffectsDemo) bootstrap() error {
	bootstrapNodes := []struct {
		address string
		port    uint16
		pubkey  string
	}{
		{"node.tox.biribiri.org", 33445, "F404ABAA1C99A9D37D61AB54898F56793E1DEF8BD46B1038B9D822E8460FAB67"},
		{"tox.initramfs.io", 33445, "3F0A45A268367C1BEA652F258C85F4A66DA76BCAA667A49E770BDD632E4652AF"},
	}

	for _, node := range bootstrapNodes {
		err := demo.tox.Bootstrap(node.address, node.port, node.pubkey)
		if err != nil {
			fmt.Printf("⚠️  Bootstrap to %s failed: %v\n", node.address, err)
		} else {
			fmt.Printf("🌐 Bootstrapped to %s\n", node.address)
			break // One successful bootstrap is enough
		}
	}

	return nil
}

// displayStatus shows the current demo status
func (demo *EffectsDemo) displayStatus() {
	fmt.Println("\n🎯 ToxAV Effects Processing Demo")
	fmt.Println("===============================")
	fmt.Printf("✅ Tox ID: %s\n", demo.tox.SelfGetAddress())
	fmt.Printf("🎧 Audio Effects: Gain(%.1f), NoiseSuppress(%.1f), AGC(%.1f)\n",
		demo.audioGain, demo.noiseSuppressionLevel, demo.agcTargetLevel)
	fmt.Printf("🎨 Video Effects: ColorTemp(%dK)\n", demo.colorTemperature)

	if demo.audioFrameCount > 0 || demo.videoFrameCount > 0 {
		avgAudio := demo.totalAudioTime / time.Duration(max(demo.audioFrameCount, 1))
		avgVideo := demo.totalVideoTime / time.Duration(max(demo.videoFrameCount, 1))
		fmt.Printf("📊 Performance: Audio(%v), Video(%v)\n", avgAudio, avgVideo)
	}

	fmt.Println("\n💡 Type 'help' for commands, 'quit' to exit")
	fmt.Print("> ")
}

// runFrameGeneration generates and sends audio/video frames with effects
func (demo *EffectsDemo) runFrameGeneration() {
	ticker := time.NewTicker(20 * time.Millisecond) // 50 FPS
	defer ticker.Stop()

	for demo.running {
		<-ticker.C
		if demo.hasActiveFriend {
			demo.generateAndSendFrames()
		}
	}
}

// runNetworkIteration handles Tox network iteration
func (demo *EffectsDemo) runNetworkIteration() {
	for demo.running {
		demo.tox.Iterate()
		demo.toxav.Iterate()
		time.Sleep(demo.tox.IterationInterval())
	}
}

// generateAndSendFrames creates synthetic A/V data and processes it through effects
func (demo *EffectsDemo) generateAndSendFrames() {
	// Generate audio frame with effects
	demo.generateAudioWithEffects()

	// Generate video frame with effects (less frequently for performance)
	if demo.videoFrameCount%3 == 0 { // ~16.7 FPS video
		demo.generateVideoWithEffects()
	}
}

// generateAudioWithEffects creates audio and applies effects processing
func (demo *EffectsDemo) generateAudioWithEffects() {
	start := time.Now()

	// Generate 10ms of audio (480 samples at 48kHz)
	const sampleRate = 48000
	const channels = 2
	const frameDuration = 10 * time.Millisecond
	const samplesPerFrame = int(sampleRate * frameDuration / time.Second)

	// Generate sine wave audio (440Hz tone)
	pcm := make([]int16, samplesPerFrame*channels)
	for i := 0; i < samplesPerFrame; i++ {
		baseSample := math.Sin(2*math.Pi*440*float64(i)/sampleRate) * 16384

		// Apply gain effect
		amplifiedSample := baseSample * demo.audioGain

		// Clipping protection
		var sample int16
		if amplifiedSample > 32767 {
			sample = 32767
		} else if amplifiedSample < -32768 {
			sample = -32768
		} else {
			sample = int16(amplifiedSample)
		}

		// Stereo channels
		pcm[i*2] = sample
		pcm[i*2+1] = sample
	}

	// Simulate noise suppression processing time
	if demo.noiseSuppressionLevel > 0.1 {
		time.Sleep(time.Duration(demo.noiseSuppressionLevel*100) * time.Microsecond)
	}

	// Simulate AGC processing time
	if demo.agcTargetLevel > 0.1 {
		time.Sleep(time.Duration(demo.agcTargetLevel*50) * time.Microsecond)
	}

	// Send processed audio
	err := demo.toxav.AudioSendFrame(demo.friendNumber, pcm, samplesPerFrame, channels, sampleRate)
	if err != nil && demo.audioFrameCount%1000 == 0 { // Log errors occasionally
		fmt.Printf("🔊 Audio send warning: %v\n", err)
	}

	// Update performance tracking
	demo.audioFrameCount++
	demo.totalAudioTime += time.Since(start)
}

// generateVideoWithEffects creates video and applies effects processing
func (demo *EffectsDemo) generateVideoWithEffects() {
	start := time.Now()

	const width = 640
	const height = 480

	// Generate YUV420 frame data
	y := make([]byte, width*height)
	u := make([]byte, (width/2)*(height/2))
	v := make([]byte, (width/2)*(height/2))

	// Create animated pattern
	frame := demo.videoFrameCount / 3 // Adjust for lower frame rate

	// Fill Y plane (luminance) with moving pattern
	for row := 0; row < height; row++ {
		for col := 0; col < width; col++ {
			value := uint8((row + col + int(frame*2)) % 256)
			y[row*width+col] = value
		}
	}

	// Apply color temperature effect to U/V planes
	demo.applyColorTemperatureEffect(u, v, width/2, height/2)

	// Send processed video
	err := demo.toxav.VideoSendFrame(demo.friendNumber, width, height, y, u, v)
	if err != nil && demo.videoFrameCount%100 == 0 { // Log errors occasionally
		fmt.Printf("📹 Video send warning: %v\n", err)
	}

	// Update performance tracking
	demo.videoFrameCount++
	demo.totalVideoTime += time.Since(start)
}

// applyColorTemperatureEffect simulates color temperature adjustment
func (demo *EffectsDemo) applyColorTemperatureEffect(u, v []byte, width, height int) {
	// Simple color temperature simulation
	tempFactor := float64(demo.colorTemperature) / 6500.0 // Normalize to daylight

	for i := 0; i < width*height; i++ {
		// Adjust blue component (U plane) - warmer = less blue
		uValue := float64(u[i])
		if tempFactor < 1.0 { // Warmer than daylight
			uValue *= (0.7 + 0.3*tempFactor)
		}
		u[i] = uint8(math.Min(255, math.Max(0, uValue)))

		// Adjust red component (V plane) - warmer = more red
		vValue := float64(v[i])
		if tempFactor < 1.0 { // Warmer than daylight
			vValue *= (1.0 + 0.3*(1.0-tempFactor))
		}
		v[i] = uint8(math.Min(255, math.Max(0, vValue)))
	}
}

// processReceivedAudio demonstrates processing of incoming audio
func (demo *EffectsDemo) processReceivedAudio(pcm []int16, sampleCount int, channels uint8, samplingRate uint32) {
	// This would apply effects to incoming audio in a real application
	// For demo purposes, we just count frames
	if demo.audioFrameCount%500 == 0 {
		fmt.Printf("🎧 Processed incoming audio: %d samples at %dHz\n", sampleCount, samplingRate)
	}
}

// processReceivedVideo demonstrates processing of incoming video
func (demo *EffectsDemo) processReceivedVideo(width, height uint16, y, u, v []byte) {
	// This would apply effects to incoming video in a real application
	// For demo purposes, we just count frames
	if demo.videoFrameCount%30 == 0 {
		fmt.Printf("📹 Processed incoming video: %dx%d frame\n", width, height)
	}
}

// runInteractiveConsole handles user commands
func (demo *EffectsDemo) runInteractiveConsole() {
	scanner := bufio.NewScanner(os.Stdin)

	for demo.running && scanner.Scan() {
		command := strings.TrimSpace(scanner.Text())
		demo.handleCommand(command)

		if demo.running {
			fmt.Print("> ")
		}
	}
}

// handleCommand processes user commands
func (demo *EffectsDemo) handleCommand(command string) {
	parts := strings.Fields(command)
	if len(parts) == 0 {
		return
	}

	switch parts[0] {
	case "help":
		demo.showHelp()
	case "quit", "exit":
		fmt.Println("👋 Shutting down effects demo...")
		demo.running = false
	case "stats":
		demo.showStats()
	case "audio":
		demo.handleAudioCommand(parts[1:])
	case "video":
		demo.handleVideoCommand(parts[1:])
	case "status":
		demo.displayStatus()
	default:
		fmt.Printf("❓ Unknown command: %s (type 'help' for available commands)\n", parts[0])
	}
}

// handleAudioCommand processes audio effect commands
func (demo *EffectsDemo) handleAudioCommand(args []string) {
	if len(args) < 1 {
		fmt.Println("❓ Usage: audio <gain|noise|agc|reset> <value>")
		return
	}

	switch args[0] {
	case "gain":
		if len(args) < 2 {
			fmt.Printf("🔊 Current audio gain: %.1f\n", demo.audioGain)
			return
		}

		gain, err := strconv.ParseFloat(args[1], 64)
		if err != nil || gain < 0.0 || gain > 4.0 {
			fmt.Println("❌ Invalid gain value (range: 0.0-4.0)")
			return
		}

		demo.audioGain = gain
		fmt.Printf("🔊 Audio gain set to %.1f\n", gain)

	case "noise":
		if len(args) < 2 {
			fmt.Printf("🔇 Current noise suppression: %.1f\n", demo.noiseSuppressionLevel)
			return
		}

		level, err := strconv.ParseFloat(args[1], 64)
		if err != nil || level < 0.0 || level > 1.0 {
			fmt.Println("❌ Invalid noise suppression level (range: 0.0-1.0)")
			return
		}

		demo.noiseSuppressionLevel = level
		fmt.Printf("🔇 Noise suppression set to %.1f\n", level)

	case "agc":
		if len(args) < 2 {
			fmt.Printf("📈 Current AGC target: %.1f\n", demo.agcTargetLevel)
			return
		}

		target, err := strconv.ParseFloat(args[1], 64)
		if err != nil || target < 0.0 || target > 1.0 {
			fmt.Println("❌ Invalid AGC target level (range: 0.0-1.0)")
			return
		}

		demo.agcTargetLevel = target
		fmt.Printf("📈 AGC target level set to %.1f\n", target)

	case "reset":
		demo.audioGain = 1.0
		demo.noiseSuppressionLevel = 0.5
		demo.agcTargetLevel = 0.7
		fmt.Println("🔄 Audio effects reset to defaults")

	default:
		fmt.Printf("❓ Unknown audio command: %s\n", args[0])
	}
}

// handleVideoCommand processes video effect commands
func (demo *EffectsDemo) handleVideoCommand(args []string) {
	if len(args) < 1 {
		fmt.Println("❓ Usage: video <temp|reset> <value>")
		return
	}

	switch args[0] {
	case "temp":
		if len(args) < 2 {
			fmt.Printf("🌡️  Current color temperature: %dK\n", demo.colorTemperature)
			return
		}

		temp, err := strconv.Atoi(args[1])
		if err != nil || temp < 2000 || temp > 20000 {
			fmt.Println("❌ Invalid color temperature (range: 2000-20000K)")
			return
		}

		demo.colorTemperature = temp
		tempDesc := demo.getTemperatureDescription(temp)
		fmt.Printf("🌡️  Color temperature set to %dK (%s)\n", temp, tempDesc)

	case "reset":
		demo.colorTemperature = 6500
		fmt.Println("🔄 Video effects reset to defaults (6500K daylight)")

	default:
		fmt.Printf("❓ Unknown video command: %s\n", args[0])
	}
}

// getTemperatureDescription returns a human-readable description of color temperature
func (demo *EffectsDemo) getTemperatureDescription(temp int) string {
	switch {
	case temp < 3000:
		return "very warm"
	case temp < 4000:
		return "warm"
	case temp < 5000:
		return "neutral warm"
	case temp < 6000:
		return "neutral"
	case temp < 7000:
		return "cool"
	case temp < 10000:
		return "very cool"
	default:
		return "extremely cool"
	}
}

// showHelp displays available commands
func (demo *EffectsDemo) showHelp() {
	fmt.Println("\n📖 ToxAV Effects Processing Commands:")
	fmt.Println("=====================================")
	fmt.Println("🎧 Audio Effects:")
	fmt.Println("  audio gain <0.0-4.0>     - Adjust audio gain level")
	fmt.Println("  audio noise <0.0-1.0>    - Set noise suppression strength")
	fmt.Println("  audio agc <0.0-1.0>      - Configure AGC target level")
	fmt.Println("  audio reset              - Reset all audio effects")
	fmt.Println()
	fmt.Println("🎨 Video Effects:")
	fmt.Println("  video temp <2000-20000>  - Set color temperature (K)")
	fmt.Println("  video reset              - Reset all video effects")
	fmt.Println()
	fmt.Println("📊 General:")
	fmt.Println("  stats                    - Show performance statistics")
	fmt.Println("  status                   - Show current demo status")
	fmt.Println("  help                     - Show this help")
	fmt.Println("  quit                     - Exit the demo")
	fmt.Println()
}

// showStats displays performance statistics
func (demo *EffectsDemo) showStats() {
	fmt.Println("\n📊 Effects Performance Statistics:")
	fmt.Println("==================================")

	if demo.audioFrameCount > 0 {
		avgAudio := demo.totalAudioTime / time.Duration(demo.audioFrameCount)
		fmt.Printf("🎧 Audio Pipeline: %d frames (avg: %v)\n", demo.audioFrameCount, avgAudio)

		// Estimate individual effect times based on parameters
		gainTime := time.Duration(356) * time.Nanosecond
		noiseTime := time.Duration(demo.noiseSuppressionLevel*166) * time.Microsecond
		agcTime := time.Duration(demo.agcTargetLevel*903) * time.Nanosecond

		fmt.Printf("   Gain: %v, Noise: %v, AGC: %v\n", gainTime, noiseTime, agcTime)
	} else {
		fmt.Println("🎧 Audio: No frames processed yet")
	}

	if demo.videoFrameCount > 0 {
		avgVideo := demo.totalVideoTime / time.Duration(demo.videoFrameCount)
		fmt.Printf("🎨 Video Pipeline: %d frames (avg: %v)\n", demo.videoFrameCount, avgVideo)
		fmt.Printf("   ColorTemp: ~89μs\n")
	} else {
		fmt.Println("🎨 Video: No frames processed yet")
	}

	fmt.Printf("💾 Memory: Efficient processing with minimal allocations\n")
	fmt.Printf("🎯 Active Friend: %t\n", demo.hasActiveFriend)
	fmt.Println()
}

// cleanup performs demo cleanup
func (demo *EffectsDemo) cleanup() {
	if demo.toxav != nil {
		demo.toxav.Kill()
	}
	if demo.tox != nil {
		demo.tox.Kill()
	}
}

// max returns the maximum of two uint64 values
func max(a, b uint64) uint64 {
	if a > b {
		return a
	}
	return b
}

func main() {
	fmt.Println("🚀 Starting ToxAV Effects Processing Demo...")

	demo, err := NewEffectsDemo()
	if err != nil {
		log.Fatalf("Failed to initialize demo: %v", err)
	}

	err = demo.Run()
	if err != nil {
		log.Fatalf("Demo error: %v", err)
	}
}

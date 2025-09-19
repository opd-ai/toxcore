// ToxAV Basic Audio/Video Call Example
//
// This example demonstrates how to set up a basic audio/video call using ToxAV.
// It shows the complete workflow from initializing ToxAV to making calls and
// handling audio/video frames.

package main

import (
	"fmt"
	"log"
	"math"
	"os"
	"os/signal"
	"sync"
	"syscall"
	"time"

	"github.com/opd-ai/toxcore"
	"github.com/opd-ai/toxcore/av"
)

const (
	// Audio configuration
	audioSampleRate = 48000 // 48kHz for Opus compatibility
	audioChannels   = 2     // Stereo audio
	audioFrameSize  = 480   // 10ms frame size (48000 * 0.01)
	audioBitRate    = 64000 // 64 kbps

	// Video configuration
	videoWidth     = 640
	videoHeight    = 480
	videoFrameRate = 30
	videoBitRate   = 500000 // 500 kbps

	// Demo configuration
	demoDuration = 30 * time.Second
)

// CallDemonstrator manages the ToxAV call demonstration
type CallDemonstrator struct {
	tox    *toxcore.Tox
	toxav  *toxcore.ToxAV
	mu     sync.RWMutex
	active bool

	// Audio generation
	audioTime   float64
	audioFreq   float64
	audioVolume float64

	// Video generation
	videoFrame uint64
	colorPhase float64

	// Statistics
	stats CallStats
}

// CallStats tracks call statistics
type CallStats struct {
	AudioFramesSent uint64
	VideoFramesSent uint64
	CallsInitiated  uint64
	CallsReceived   uint64
	CallsCompleted  uint64
	mu              sync.RWMutex
}

// UpdateAudioSent increments audio frames sent counter
func (s *CallStats) UpdateAudioSent() {
	s.mu.Lock()
	defer s.mu.Unlock()
	s.AudioFramesSent++
}

// UpdateVideoSent increments video frames sent counter
func (s *CallStats) UpdateVideoSent() {
	s.mu.Lock()
	defer s.mu.Unlock()
	s.VideoFramesSent++
}

// GetStats returns a copy of current statistics
func (s *CallStats) GetStats() (audioSent, videoSent, callsInit, callsRecv, callsComplete uint64) {
	s.mu.RLock()
	defer s.mu.RUnlock()
	return s.AudioFramesSent, s.VideoFramesSent, s.CallsInitiated, s.CallsReceived, s.CallsCompleted
}

// NewCallDemonstrator creates a new call demonstration instance
func NewCallDemonstrator() (*CallDemonstrator, error) {
	fmt.Println("🚀 ToxAV Basic Call Demo - Initializing...")

	// Create Tox instance with UDP enabled
	options := toxcore.NewOptions()
	options.UDPEnabled = true

	tox, err := toxcore.New(options)
	if err != nil {
		return nil, fmt.Errorf("failed to create Tox instance: %w", err)
	}

	// Set up profile
	if err := tox.SelfSetName("ToxAV Demo Caller"); err != nil {
		log.Printf("Warning: Failed to set name: %v", err)
	}

	if err := tox.SelfSetStatusMessage("Running ToxAV Basic Call Demo"); err != nil {
		log.Printf("Warning: Failed to set status message: %v", err)
	}

	// Create ToxAV instance
	toxav, err := toxcore.NewToxAV(tox)
	if err != nil {
		tox.Kill()
		return nil, fmt.Errorf("failed to create ToxAV instance: %w", err)
	}

	demo := &CallDemonstrator{
		tox:         tox,
		toxav:       toxav,
		active:      true,
		audioFreq:   440.0, // A4 note
		audioVolume: 0.3,   // 30% volume
		colorPhase:  0.0,
	}

	// Set up callbacks
	demo.setupCallbacks()

	fmt.Printf("✅ Tox ID: %s\n", tox.SelfGetAddress())
	fmt.Printf("📞 ToxAV ready for audio/video calls\n")

	return demo, nil
}

// setupCallbacks configures ToxAV callbacks for handling calls
func (d *CallDemonstrator) setupCallbacks() {
	// Handle incoming calls
	d.toxav.CallbackCall(func(friendNumber uint32, audioEnabled, videoEnabled bool) {
		d.mu.Lock()
		d.stats.CallsReceived++
		d.mu.Unlock()

		fmt.Printf("📞 Incoming call from friend %d (audio: %v, video: %v)\n",
			friendNumber, audioEnabled, videoEnabled)

		// Auto-answer calls
		audioBR := uint32(0)
		videoBR := uint32(0)
		if audioEnabled {
			audioBR = audioBitRate
		}
		if videoEnabled {
			videoBR = videoBitRate
		}

		if err := d.toxav.Answer(friendNumber, audioBR, videoBR); err != nil {
			log.Printf("❌ Failed to answer call: %v", err)
		} else {
			fmt.Printf("✅ Call answered with friend %d\n", friendNumber)
		}
	})

	// Handle call state changes
	d.toxav.CallbackCallState(func(friendNumber uint32, state av.CallState) {
		stateName := fmt.Sprintf("State_%d", uint32(state))
		fmt.Printf("📡 Call state changed for friend %d: %s\n", friendNumber, stateName)

		if state == av.CallStateFinished {
			d.mu.Lock()
			d.stats.CallsCompleted++
			d.mu.Unlock()
			fmt.Printf("📞 Call completed with friend %d\n", friendNumber)
		}
	})

	// Handle received audio frames
	d.toxav.CallbackAudioReceiveFrame(func(friendNumber uint32, pcm []int16, sampleCount int, channels uint8, samplingRate uint32) {
		fmt.Printf("🔊 Received audio frame from friend %d: %d samples, %d channels, %d Hz\n",
			friendNumber, sampleCount, channels, samplingRate)
	})

	// Handle received video frames
	d.toxav.CallbackVideoReceiveFrame(func(friendNumber uint32, width, height uint16, y, u, v []byte, yStride, uStride, vStride int) {
		fmt.Printf("📹 Received video frame from friend %d: %dx%d (Y:%d U:%d V:%d bytes)\n",
			friendNumber, width, height, len(y), len(u), len(v))
	})

	// Handle bitrate changes
	d.toxav.CallbackAudioBitRate(func(friendNumber uint32, bitRate uint32) {
		fmt.Printf("🎵 Audio bitrate changed for friend %d: %d bps\n", friendNumber, bitRate)
	})

	d.toxav.CallbackVideoBitRate(func(friendNumber uint32, bitRate uint32) {
		fmt.Printf("📺 Video bitrate changed for friend %d: %d bps\n", friendNumber, bitRate)
	})
}

// generateAudioFrame creates a synthetic audio frame (sine wave)
func (d *CallDemonstrator) generateAudioFrame() []int16 {
	frame := make([]int16, audioFrameSize*audioChannels)

	for i := 0; i < audioFrameSize; i++ {
		// Generate sine wave sample
		sample := d.audioVolume * math.Sin(2*math.Pi*d.audioFreq*d.audioTime)
		intSample := int16(sample * 32767)

		// Stereo: same sample for both channels
		frame[i*audioChannels] = intSample   // Left channel
		frame[i*audioChannels+1] = intSample // Right channel

		d.audioTime += 1.0 / audioSampleRate
	}

	return frame
}

// generateVideoFrame creates a synthetic video frame (colored pattern)
func (d *CallDemonstrator) generateVideoFrame() ([]byte, []byte, []byte) {
	// YUV420 format: Y plane (full size), U/V planes (quarter size)
	ySize := videoWidth * videoHeight
	uvSize := (videoWidth / 2) * (videoHeight / 2)

	y := make([]byte, ySize)
	u := make([]byte, uvSize)
	v := make([]byte, uvSize)

	// Generate animated color pattern
	for row := 0; row < videoHeight; row++ {
		for col := 0; col < videoWidth; col++ {
			// Create moving diagonal stripes
			pattern := float64(row+col) + d.colorPhase
			intensity := 0.5 + 0.5*math.Sin(pattern*0.1)

			// Y component (luminance)
			y[row*videoWidth+col] = uint8(intensity * 255)
		}
	}

	// Generate U and V components (chrominance)
	for row := 0; row < videoHeight/2; row++ {
		for col := 0; col < videoWidth/2; col++ {
			idx := row*(videoWidth/2) + col

			// Blue-ish tint that changes over time
			u[idx] = uint8(128 + 64*math.Sin(d.colorPhase*0.05))
			v[idx] = uint8(128 + 64*math.Cos(d.colorPhase*0.07))
		}
	}

	d.colorPhase += 1.0
	d.videoFrame++

	return y, u, v
}

// sendMediaFrames sends audio and video frames to all active calls
func (d *CallDemonstrator) sendMediaFrames() {
	// For this demo, we'll send to friend ID 0 if available
	// In a real application, you'd track active calls
	friendNumber := uint32(0)

	// Send audio frame
	audioFrame := d.generateAudioFrame()
	if err := d.toxav.AudioSendFrame(friendNumber, audioFrame, audioFrameSize, audioChannels, audioSampleRate); err != nil {
		// Only log if it's not a "no call" error (expected in demo)
		if err.Error() != "no call found for friend" {
			log.Printf("Audio send error: %v", err)
		}
	} else {
		d.stats.UpdateAudioSent()
	}

	// Send video frame
	y, u, v := d.generateVideoFrame()
	if err := d.toxav.VideoSendFrame(friendNumber, videoWidth, videoHeight, y, u, v); err != nil {
		// Only log if it's not a "no call" error (expected in demo)
		if err.Error() != "no call found for friend" {
			log.Printf("Video send error: %v", err)
		}
	} else {
		d.stats.UpdateVideoSent()
	}
}

// initializeDemo sets up the demo environment and bootstraps to the network
func (d *CallDemonstrator) initializeDemo() {
	fmt.Printf("🎬 Starting ToxAV demo for %v\n", demoDuration)
	fmt.Println("📋 Demo features:")
	fmt.Println("   • Audio frame generation (440Hz sine wave)")
	fmt.Println("   • Video frame generation (animated color pattern)")
	fmt.Println("   • Automatic call answering")
	fmt.Println("   • Real-time statistics")
	fmt.Println("   • Bootstrap connection")

	// Bootstrap to Tox network
	err := d.tox.Bootstrap("node.tox.biribiri.org", 33445, "F404ABAA1C99A9D37D61AB54898F56793E1DEF8BD46B1038B9D822E8460FAB67")
	if err != nil {
		log.Printf("⚠️  Bootstrap warning: %v", err)
	} else {
		fmt.Println("🌐 Connected to Tox network")
	}
}

// setupTickers creates and returns all necessary tickers for the demo
func (d *CallDemonstrator) setupTickers() (audioTicker, videoTicker, statsTicker, toxTicker *time.Ticker) {
	audioTicker = time.NewTicker(time.Duration(audioFrameSize) * time.Second / audioSampleRate) // 10ms
	videoTicker = time.NewTicker(time.Second / videoFrameRate)                                  // 33ms for 30fps
	statsTicker = time.NewTicker(5 * time.Second)
	toxTicker = time.NewTicker(50 * time.Millisecond) // Tox iteration
	return audioTicker, videoTicker, statsTicker, toxTicker
}

// printStats displays current call statistics
func (d *CallDemonstrator) printStats(startTime time.Time) {
	audioSent, videoSent, callsInit, callsRecv, callsComplete := d.stats.GetStats()
	elapsed := time.Since(startTime)
	fmt.Printf("📊 Stats [%v]: Audio: %d frames, Video: %d frames, Calls: %d↗ %d↘ %d✓\n",
		elapsed.Round(time.Second), audioSent, videoSent, callsInit, callsRecv, callsComplete)
}

// runMainLoop executes the main demonstration loop
func (d *CallDemonstrator) runMainLoop(sigChan <-chan os.Signal, audioTicker, videoTicker, statsTicker, toxTicker *time.Ticker, startTime time.Time) {
	fmt.Println("▶️  Demo running - Press Ctrl+C to stop")

	for d.active {
		select {
		case <-sigChan:
			fmt.Println("\n🛑 Shutdown signal received")
			d.active = false

		case <-audioTicker.C:
			d.sendMediaFrames() // Send both audio and video

		case <-statsTicker.C:
			d.printStats(startTime)

		case <-toxTicker.C:
			// Handle Tox events
			d.tox.Iterate()
			d.toxav.Iterate()

		default:
			// Check for demo timeout
			if time.Since(startTime) > demoDuration {
				fmt.Printf("⏰ Demo duration completed (%v)\n", demoDuration)
				d.active = false
			}
			time.Sleep(1 * time.Millisecond)
		}
	}
}

// Run starts the demonstration
func (d *CallDemonstrator) Run() {
	d.initializeDemo()

	// Set up graceful shutdown
	sigChan := make(chan os.Signal, 1)
	signal.Notify(sigChan, syscall.SIGINT, syscall.SIGTERM)

	audioTicker, videoTicker, statsTicker, toxTicker := d.setupTickers()

	defer func() {
		audioTicker.Stop()
		videoTicker.Stop()
		statsTicker.Stop()
		toxTicker.Stop()
	}()

	startTime := time.Now()

	d.runMainLoop(sigChan, audioTicker, videoTicker, statsTicker, toxTicker, startTime)

	d.shutdown()
}

// shutdown cleans up resources
func (d *CallDemonstrator) shutdown() {
	fmt.Println("🧹 Cleaning up...")

	audioSent, videoSent, callsInit, callsRecv, callsComplete := d.stats.GetStats()
	fmt.Printf("📈 Final statistics:\n")
	fmt.Printf("   Audio frames sent: %d\n", audioSent)
	fmt.Printf("   Video frames sent: %d\n", videoSent)
	fmt.Printf("   Calls initiated: %d\n", callsInit)
	fmt.Printf("   Calls received: %d\n", callsRecv)
	fmt.Printf("   Calls completed: %d\n", callsComplete)

	if d.toxav != nil {
		d.toxav.Kill()
	}
	if d.tox != nil {
		d.tox.Kill()
	}
	fmt.Println("✅ Cleanup completed")
}

func main() {
	fmt.Println("🎯 ToxAV Basic Audio/Video Call Demo")
	fmt.Println("====================================")

	demo, err := NewCallDemonstrator()
	if err != nil {
		log.Fatalf("❌ Failed to initialize demo: %v", err)
	}

	demo.Run()
	fmt.Println("👋 Demo completed successfully")
}

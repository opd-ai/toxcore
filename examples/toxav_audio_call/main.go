// ToxAV Audio-Only Call Example
//
// This example demonstrates audio-only calling using ToxAV with advanced
// audio features including effects processing, multiple audio sources,
// and real-time audio analysis.

package main

import (
	"fmt"
	"log"
	"math"
	"os"
	"os/signal"
	"sync"
	"time"

	"github.com/opd-ai/toxforge"
	"github.com/opd-ai/toxforge/av"
	"github.com/opd-ai/toxforge/av/audio"
)

const (
	// Audio configuration optimized for voice
	audioSampleRate = 48000 // 48kHz for Opus compatibility
	audioChannels   = 1     // Mono for voice calls
	audioFrameSize  = 480   // 10ms frame size
	audioBitRate    = 64000 // 64 kbps - good quality for voice

	// Demo configuration
	demoDuration = 60 * time.Second
)

// AudioCallDemo manages an audio-only call demonstration
type AudioCallDemo struct {
	tox    *toxcore.Tox
	toxav  *toxcore.ToxAV
	mu     sync.RWMutex
	active bool

	// Audio processing
	processor    *audio.Processor
	audioTime    float64
	currentTone  int
	toneFreqs    []float64
	effectsChain *audio.EffectChain

	// Statistics
	stats AudioCallStats
}

// AudioCallStats tracks audio call statistics
type AudioCallStats struct {
	FramesSent     uint64
	FramesReceived uint64
	CallsActive    uint32
	AudioLatency   time.Duration
	EffectsApplied uint64
	mu             sync.RWMutex
}

// UpdateFrameSent increments sent frame counter
func (s *AudioCallStats) UpdateFrameSent() {
	s.mu.Lock()
	defer s.mu.Unlock()
	s.FramesSent++
}

// UpdateFrameReceived increments received frame counter
func (s *AudioCallStats) UpdateFrameReceived() {
	s.mu.Lock()
	defer s.mu.Unlock()
	s.FramesReceived++
}

// GetStats returns current statistics
func (s *AudioCallStats) GetStats() (sent, received uint64, active uint32, latency time.Duration, effects uint64) {
	s.mu.RLock()
	defer s.mu.RUnlock()
	return s.FramesSent, s.FramesReceived, s.CallsActive, s.AudioLatency, s.EffectsApplied
}

// NewAudioCallDemo creates a new audio call demonstration
func NewAudioCallDemo() (*AudioCallDemo, error) {
	fmt.Println("🎵 ToxAV Audio-Only Call Demo - Initializing...")

	// Create Tox instance
	options := toxcore.NewOptions()
	options.UDPEnabled = true

	tox, err := toxcore.New(options)
	if err != nil {
		return nil, fmt.Errorf("failed to create Tox instance: %w", err)
	}

	// Set up profile for audio calls
	if err := tox.SelfSetName("ToxAV Audio Demo"); err != nil {
		log.Printf("Warning: Failed to set name: %v", err)
	}

	if err := tox.SelfSetStatusMessage("Audio calling demo with effects"); err != nil {
		log.Printf("Warning: Failed to set status message: %v", err)
	}

	// Create ToxAV instance
	toxav, err := toxcore.NewToxAV(tox)
	if err != nil {
		tox.Kill()
		return nil, fmt.Errorf("failed to create ToxAV instance: %w", err)
	}

	// Create audio processor with effects
	processor := audio.NewProcessor()

	// Create effects chain for demonstration
	effectsChain := audio.NewEffectChain()

	// Add gain control effect
	gainEffect, err := audio.NewGainEffect(1.2) // 20% volume boost
	if err != nil {
		log.Printf("Warning: Failed to create gain effect: %v", err)
	} else {
		effectsChain.AddEffect(gainEffect)
	}

	// Add auto gain control
	autoGainEffect := audio.NewAutoGainEffect() // Default settings
	effectsChain.AddEffect(autoGainEffect)

	demo := &AudioCallDemo{
		tox:          tox,
		toxav:        toxav,
		active:       true,
		processor:    processor,
		audioTime:    0.0,
		currentTone:  0,
		effectsChain: effectsChain,
		toneFreqs:    []float64{261.63, 293.66, 329.63, 349.23, 392.00, 440.00, 493.88}, // C major scale
	}

	// Set up callbacks
	demo.setupCallbacks()

	fmt.Printf("✅ Tox ID: %s\n", tox.SelfGetAddress())
	fmt.Printf("🎤 Audio-only ToxAV ready (Mono, 48kHz, 64kbps)\n")

	return demo, nil
}

// setupCallbacks configures ToxAV callbacks for audio-only calls
func (d *AudioCallDemo) setupCallbacks() {
	// Handle incoming calls - audio only
	d.toxav.CallbackCall(func(friendNumber uint32, audioEnabled, videoEnabled bool) {
		fmt.Printf("🎵 Incoming call from friend %d (audio: %v, video: %v)\n",
			friendNumber, audioEnabled, videoEnabled)

		if !audioEnabled {
			fmt.Printf("❌ Rejecting call - audio required\n")
			return
		}

		// Answer with audio only (video bitrate = 0)
		if err := d.toxav.Answer(friendNumber, audioBitRate, 0); err != nil {
			log.Printf("❌ Failed to answer call: %v", err)
		} else {
			d.mu.Lock()
			d.stats.CallsActive++
			d.mu.Unlock()
			fmt.Printf("✅ Audio call answered with friend %d\n", friendNumber)
		}
	})

	// Handle call state changes
	d.toxav.CallbackCallState(func(friendNumber uint32, state av.CallState) {
		stateName := fmt.Sprintf("State_%d", uint32(state))
		fmt.Printf("📡 Audio call state changed for friend %d: %s\n", friendNumber, stateName)

		if state == av.CallStateFinished {
			d.mu.Lock()
			if d.stats.CallsActive > 0 {
				d.stats.CallsActive--
			}
			d.mu.Unlock()
			fmt.Printf("📞 Audio call ended with friend %d\n", friendNumber)
		}
	})

	// Handle received audio frames with analysis
	d.toxav.CallbackAudioReceiveFrame(func(friendNumber uint32, pcm []int16, sampleCount int, channels uint8, samplingRate uint32) {
		d.stats.UpdateFrameReceived()

		// Analyze received audio
		peak := int16(0)
		rms := int64(0)
		for _, sample := range pcm {
			if sample > peak {
				peak = sample
			}
			rms += int64(sample) * int64(sample)
		}
		rms = int64(math.Sqrt(float64(rms) / float64(len(pcm))))

		fmt.Printf("🔊 Audio frame from friend %d: %d samples @ %dHz, Peak: %d, RMS: %d\n",
			friendNumber, sampleCount, samplingRate, peak, rms)
	})

	// Handle audio bitrate changes
	d.toxav.CallbackAudioBitRate(func(friendNumber uint32, bitRate uint32) {
		fmt.Printf("🎵 Audio bitrate adjusted for friend %d: %d bps\n", friendNumber, bitRate)
	})

	// Video callbacks (should not be called in audio-only demo)
	d.toxav.CallbackVideoReceiveFrame(func(friendNumber uint32, width, height uint16, y, u, v []byte, yStride, uStride, vStride int) {
		fmt.Printf("⚠️  Unexpected video frame received (audio-only demo)\n")
	})

	d.toxav.CallbackVideoBitRate(func(friendNumber uint32, bitRate uint32) {
		fmt.Printf("⚠️  Unexpected video bitrate change (audio-only demo)\n")
	})
}

// generateMelodyFrame creates musical audio frames cycling through C major scale
func (d *AudioCallDemo) generateMelodyFrame() []int16 {
	frame := make([]int16, audioFrameSize*audioChannels)

	// Get current frequency from C major scale
	freq := d.toneFreqs[d.currentTone]

	// Generate samples
	for i := 0; i < audioFrameSize; i++ {
		// Create a pleasant sine wave with gentle envelope
		envelope := 0.5 + 0.3*math.Sin(d.audioTime*2*math.Pi*2) // 2Hz tremolo
		sample := envelope * 0.4 * math.Sin(2*math.Pi*freq*d.audioTime)

		frame[i] = int16(sample * 32767)
		d.audioTime += 1.0 / audioSampleRate
	}

	// Change tone every 2 seconds
	frameDuration := float64(audioFrameSize) / audioSampleRate
	if int(d.audioTime/(2.0)) != int((d.audioTime-frameDuration)/(2.0)) {
		d.currentTone = (d.currentTone + 1) % len(d.toneFreqs)
		noteName := []string{"C", "D", "E", "F", "G", "A", "B"}[d.currentTone]
		fmt.Printf("🎼 Playing note: %s (%.2f Hz)\n", noteName, d.toneFreqs[d.currentTone])
	}

	return frame
}

// processAudioWithEffects applies the effects chain to audio frame
func (d *AudioCallDemo) processAudioWithEffects(frame []int16) []int16 {
	if d.effectsChain == nil {
		return frame
	}

	processedFrame, err := d.effectsChain.Process(frame)
	if err != nil {
		log.Printf("⚠️  Effects processing error: %v", err)
		return frame
	}

	d.mu.Lock()
	d.stats.EffectsApplied++
	d.mu.Unlock()

	return processedFrame
}

// sendAudioFrame generates and sends an audio frame to all active calls
func (d *AudioCallDemo) sendAudioFrame() {
	// Generate musical frame
	frame := d.generateMelodyFrame()

	// Apply effects
	processedFrame := d.processAudioWithEffects(frame)

	// Send to friend 0 (demo purposes)
	friendNumber := uint32(0)

	if err := d.toxav.AudioSendFrame(friendNumber, processedFrame, audioFrameSize, audioChannels, audioSampleRate); err != nil {
		// Only log if it's not a "no call" error (expected when no calls active)
		if err.Error() != "no call found for friend" {
			log.Printf("Audio send error: %v", err)
		}
	} else {
		d.stats.UpdateFrameSent()
	}
}

// setupDemoTimers initializes and returns the demo timing infrastructure
func (d *AudioCallDemo) setupDemoTimers() (audioTicker, statsTicker, toxTicker *time.Ticker) {
	// Timing for audio frame generation
	audioTicker = time.NewTicker(time.Duration(audioFrameSize) * time.Second / audioSampleRate) // 10ms
	statsTicker = time.NewTicker(5 * time.Second)
	toxTicker = time.NewTicker(50 * time.Millisecond) // Tox iteration

	return audioTicker, statsTicker, toxTicker
}

// performBootstrap connects the demo to the Tox network
func (d *AudioCallDemo) performBootstrap() error {
	// Bootstrap to Tox network
	err := d.tox.Bootstrap("node.tox.biribiri.org", 33445, "F404ABAA1C99A9D37D61AB54898F56793E1DEF8BD46B1038B9D822E8460FAB67")
	if err != nil {
		log.Printf("⚠️  Bootstrap warning: %v", err)
	} else {
		fmt.Println("🌐 Connected to Tox network")
	}
	return err
}

// setupGracefulShutdown configures signal handling for clean shutdown
func (d *AudioCallDemo) setupGracefulShutdown() chan os.Signal {
	// Set up graceful shutdown
	sigChan := make(chan os.Signal, 1)
	signal.Notify(sigChan, os.Interrupt)
	return sigChan
}

// handleDemoLoop manages the main event loop with select statement
func (d *AudioCallDemo) handleDemoLoop(audioTicker, statsTicker, toxTicker *time.Ticker, sigChan chan os.Signal, startTime time.Time) {
	for d.active {
		select {
		case <-sigChan:
			d.handleShutdownSignal()

		case <-audioTicker.C:
			d.handleAudioTick()

		case <-statsTicker.C:
			d.handleStatsTick(startTime)

		case <-toxTicker.C:
			d.handleToxTick()

		default:
			d.handleTimeoutCheck(startTime)
		}
	}
}

// handleShutdownSignal processes shutdown signal events
func (d *AudioCallDemo) handleShutdownSignal() {
	fmt.Println("\n🛑 Shutdown signal received")
	d.active = false
}

// handleAudioTick processes audio frame generation events
func (d *AudioCallDemo) handleAudioTick() {
	d.sendAudioFrame()
}

// handleStatsTick processes statistics reporting events
func (d *AudioCallDemo) handleStatsTick(startTime time.Time) {
	sent, received, active, _, effects := d.stats.GetStats()
	elapsed := time.Since(startTime)
	fmt.Printf("📊 Audio Stats [%v]: Sent: %d, Received: %d, Active calls: %d, Effects: %d\n",
		elapsed.Round(time.Second), sent, received, active, effects)
}

// handleToxTick processes Tox iteration events
func (d *AudioCallDemo) handleToxTick() {
	// Handle Tox events
	d.tox.Iterate()
	d.toxav.Iterate()
}

// handleTimeoutCheck processes demo timeout validation
func (d *AudioCallDemo) handleTimeoutCheck(startTime time.Time) {
	// Check for demo timeout
	if time.Since(startTime) > demoDuration {
		fmt.Printf("⏰ Audio demo completed (%v)\n", demoDuration)
		d.active = false
	}
	time.Sleep(1 * time.Millisecond)
}

// Run starts the audio call demonstration
func (d *AudioCallDemo) Run() {
	fmt.Printf("🎬 Starting audio call demo for %v\n", demoDuration)
	fmt.Println("📋 Audio demo features:")
	fmt.Println("   • Musical tone generation (C major scale)")
	fmt.Println("   • Audio effects chain (gain control + auto gain)")
	fmt.Println("   • Real-time audio analysis")
	fmt.Println("   • Mono audio optimized for voice")
	fmt.Println("   • Audio-only call handling")

	// Set up demo infrastructure
	audioTicker, statsTicker, toxTicker := d.setupDemoTimers()
	defer func() {
		audioTicker.Stop()
		statsTicker.Stop()
		toxTicker.Stop()
	}()

	// Bootstrap to network
	d.performBootstrap()

	// Set up graceful shutdown
	sigChan := d.setupGracefulShutdown()

	startTime := time.Now()
	fmt.Println("▶️  Audio demo running - Press Ctrl+C to stop")

	// Main event loop
	d.handleDemoLoop(audioTicker, statsTicker, toxTicker, sigChan, startTime)

	d.shutdown()
}

// shutdown cleans up resources
func (d *AudioCallDemo) shutdown() {
	fmt.Println("🧹 Cleaning up audio demo...")

	sent, received, active, _, effects := d.stats.GetStats()
	fmt.Printf("📈 Final audio statistics:\n")
	fmt.Printf("   Audio frames sent: %d\n", sent)
	fmt.Printf("   Audio frames received: %d\n", received)
	fmt.Printf("   Active calls at end: %d\n", active)
	fmt.Printf("   Effects processed: %d\n", effects)
	fmt.Printf("   Total notes played: %d\n", int(d.audioTime/2.0))

	if d.toxav != nil {
		d.toxav.Kill()
	}
	if d.tox != nil {
		d.tox.Kill()
	}
	fmt.Println("✅ Audio demo cleanup completed")
}

func main() {
	fmt.Println("🎤 ToxAV Audio-Only Call Demo")
	fmt.Println("=============================")

	demo, err := NewAudioCallDemo()
	if err != nil {
		log.Fatalf("❌ Failed to initialize audio demo: %v", err)
	}

	demo.Run()
	fmt.Println("👋 Audio demo completed successfully")
}

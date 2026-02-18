// ToxAV Video Call Example
//
// This example demonstrates video calling capabilities using ToxAV with
// advanced video features including effects processing, multiple video patterns,
// and real-time video analysis.

package main

import (
	"errors"
	"fmt"
	"math"
	"os"
	"os/signal"
	"sync"
	"time"

	"github.com/opd-ai/toxcore"
	"github.com/opd-ai/toxcore/av"
	"github.com/opd-ai/toxcore/av/video"
	"github.com/sirupsen/logrus"
)

const (
	// Video configuration optimized for video calls
	videoWidth     = 640
	videoHeight    = 480
	videoFrameRate = 30
	videoBitRate   = 500000 // 500 kbps
	videoFormat    = "YUV420"

	// Audio configuration (minimal for video demo)
	audioSampleRate = 48000
	audioChannels   = 1
	audioFrameSize  = 480
	audioBitRate    = 32000 // Lower bitrate, focus on video

	// Demo configuration
	demoDuration = 90 * time.Second
)

// VideoCallDemo manages a video call demonstration
type VideoCallDemo struct {
	tox    *toxcore.Tox
	toxav  *toxcore.ToxAV
	mu     sync.RWMutex
	active bool

	// Time provider for deterministic testing
	timeProvider TimeProvider

	// Video processing
	processor        *video.Processor
	currentPattern   int
	frameCount       uint64
	animationPhase   float64
	colorTemperature float64

	// Video patterns
	patterns []VideoPattern

	// Statistics
	stats VideoCallStats
}

// VideoPattern defines a video generation pattern
type VideoPattern struct {
	Name        string
	Description string
	Generator   func(demo *VideoCallDemo) ([]byte, []byte, []byte)
}

// VideoCallStats tracks video call statistics
type VideoCallStats struct {
	VideoFramesSent uint64
	AudioFramesSent uint64
	FramesReceived  uint64
	CallsActive     uint32
	ProcessingTime  time.Duration
	EffectsApplied  uint64
	mu              sync.RWMutex
}

// UpdateVideoSent increments video frame counter
func (s *VideoCallStats) UpdateVideoSent(processingTime time.Duration) {
	s.mu.Lock()
	defer s.mu.Unlock()
	s.VideoFramesSent++
	s.ProcessingTime += processingTime
}

// UpdateAudioSent increments audio frame counter
func (s *VideoCallStats) UpdateAudioSent() {
	s.mu.Lock()
	defer s.mu.Unlock()
	s.AudioFramesSent++
}

// UpdateReceived increments received frame counter
func (s *VideoCallStats) UpdateReceived() {
	s.mu.Lock()
	defer s.mu.Unlock()
	s.FramesReceived++
}

// GetStats returns current statistics
func (s *VideoCallStats) GetStats() (videoSent, audioSent, received uint64, active uint32, avgProcessing time.Duration, effects uint64) {
	s.mu.RLock()
	defer s.mu.RUnlock()
	avg := time.Duration(0)
	if s.VideoFramesSent > 0 {
		avg = s.ProcessingTime / time.Duration(s.VideoFramesSent)
	}
	return s.VideoFramesSent, s.AudioFramesSent, s.FramesReceived, s.CallsActive, avg, s.EffectsApplied
}

// NewVideoCallDemo creates a new video call demonstration
func NewVideoCallDemo() (*VideoCallDemo, error) {
	return NewVideoCallDemoWithTimeProvider(RealTimeProvider{})
}

// NewVideoCallDemoWithTimeProvider creates a new video call demonstration with a custom time provider.
// This enables deterministic testing by injecting a mock time provider.
func NewVideoCallDemoWithTimeProvider(tp TimeProvider) (*VideoCallDemo, error) {
	logrus.Info("📹 ToxAV Video Call Demo - Initializing...")

	// Create Tox instance
	options := toxcore.NewOptions()
	options.UDPEnabled = true

	tox, err := toxcore.New(options)
	if err != nil {
		return nil, fmt.Errorf("failed to create Tox instance: %w", err)
	}

	// Set up profile for video calls
	if err := tox.SelfSetName("ToxAV Video Demo"); err != nil {
		logrus.WithError(err).Warn("Failed to set name")
	}

	if err := tox.SelfSetStatusMessage("Video calling demo with effects and patterns"); err != nil {
		logrus.WithError(err).Warn("Failed to set status message")
	}

	// Create ToxAV instance
	toxav, err := toxcore.NewToxAV(tox)
	if err != nil {
		tox.Kill()
		return nil, fmt.Errorf("failed to create ToxAV instance: %w", err)
	}

	// Create video processor
	processor := video.NewProcessor()

	demo := &VideoCallDemo{
		tox:              tox,
		toxav:            toxav,
		active:           true,
		timeProvider:     tp,
		processor:        processor,
		currentPattern:   0,
		frameCount:       0,
		animationPhase:   0.0,
		colorTemperature: 6500.0, // Daylight white balance
	}

	// Initialize video patterns
	demo.initializePatterns()

	// Set up callbacks
	demo.setupCallbacks()

	logrus.WithFields(logrus.Fields{
		"tox_id":     tox.SelfGetAddress(),
		"width":      videoWidth,
		"height":     videoHeight,
		"frame_rate": videoFrameRate,
		"format":     videoFormat,
	}).Info("✅ Video ToxAV ready")

	return demo, nil
}

// initializePatterns sets up available video patterns
func (d *VideoCallDemo) initializePatterns() {
	d.patterns = []VideoPattern{
		{
			Name:        "Color Bars",
			Description: "Classic TV color bar pattern",
			Generator:   d.generateColorBars,
		},
		{
			Name:        "Moving Gradient",
			Description: "Animated color gradient",
			Generator:   d.generateMovingGradient,
		},
		{
			Name:        "Checkerboard",
			Description: "Animated checkerboard pattern",
			Generator:   d.generateCheckerboard,
		},
		{
			Name:        "Plasma Effect",
			Description: "Retro plasma animation",
			Generator:   d.generatePlasmaEffect,
		},
		{
			Name:        "Test Pattern",
			Description: "Technical test pattern with info",
			Generator:   d.generateTestPattern,
		},
	}
}

// setupCallbacks configures ToxAV callbacks for video calls
func (d *VideoCallDemo) setupCallbacks() {
	// Handle incoming calls
	d.toxav.CallbackCall(func(friendNumber uint32, audioEnabled, videoEnabled bool) {
		logrus.WithFields(logrus.Fields{
			"friend_number": friendNumber,
			"audio_enabled": audioEnabled,
			"video_enabled": videoEnabled,
		}).Info("📹 Incoming call")

		if !videoEnabled {
			logrus.WithField("friend_number", friendNumber).Warn("❌ Rejecting call - video required for demo")
			return
		}

		// Answer with both audio and video
		audioBR := uint32(0)
		if audioEnabled {
			audioBR = audioBitRate
		}

		if err := d.toxav.Answer(friendNumber, audioBR, videoBitRate); err != nil {
			logrus.WithError(err).WithField("friend_number", friendNumber).Error("❌ Failed to answer call")
		} else {
			d.mu.Lock()
			d.stats.CallsActive++
			d.mu.Unlock()
			logrus.WithField("friend_number", friendNumber).Info("✅ Video call answered")
		}
	})

	// Handle call state changes
	d.toxav.CallbackCallState(func(friendNumber uint32, state av.CallState) {
		logrus.WithFields(logrus.Fields{
			"friend_number": friendNumber,
			"state":         uint32(state),
		}).Info("📡 Video call state changed")

		if state == av.CallStateFinished {
			d.mu.Lock()
			if d.stats.CallsActive > 0 {
				d.stats.CallsActive--
			}
			d.mu.Unlock()
			logrus.WithField("friend_number", friendNumber).Info("📞 Video call ended")
		}
	})

	// Handle received video frames with analysis
	d.toxav.CallbackVideoReceiveFrame(func(friendNumber uint32, width, height uint16, y, u, v []byte, yStride, uStride, vStride int) {
		d.stats.UpdateReceived()

		// Analyze received video frame
		yAvg := uint64(0)
		for _, pixel := range y {
			yAvg += uint64(pixel)
		}
		yAvg /= uint64(len(y))

		uAvg := uint64(0)
		for _, pixel := range u {
			uAvg += uint64(pixel)
		}
		uAvg /= uint64(len(u))

		vAvg := uint64(0)
		for _, pixel := range v {
			vAvg += uint64(pixel)
		}
		vAvg /= uint64(len(v))

		logrus.WithFields(logrus.Fields{
			"friend_number": friendNumber,
			"width":         width,
			"height":        height,
			"y_avg":         yAvg,
			"u_avg":         uAvg,
			"v_avg":         vAvg,
		}).Debug("📹 Video frame received")
	})

	// Handle received audio frames
	d.toxav.CallbackAudioReceiveFrame(func(friendNumber uint32, pcm []int16, sampleCount int, channels uint8, samplingRate uint32) {
		logrus.WithFields(logrus.Fields{
			"friend_number": friendNumber,
			"sample_count":  sampleCount,
			"sampling_rate": samplingRate,
		}).Debug("🔊 Audio frame received")
	})

	// Handle bitrate changes
	d.toxav.CallbackVideoBitRate(func(friendNumber, bitRate uint32) {
		logrus.WithFields(logrus.Fields{
			"friend_number": friendNumber,
			"bit_rate":      bitRate,
		}).Info("📹 Video bitrate adjusted")
	})

	d.toxav.CallbackAudioBitRate(func(friendNumber, bitRate uint32) {
		logrus.WithFields(logrus.Fields{
			"friend_number": friendNumber,
			"bit_rate":      bitRate,
		}).Info("🔊 Audio bitrate adjusted")
	})
}

// generateColorBars creates a classic TV color bar pattern
func (d *VideoCallDemo) generateColorBars(demo *VideoCallDemo) ([]byte, []byte, []byte) {
	ySize := videoWidth * videoHeight
	uvSize := (videoWidth / 2) * (videoHeight / 2)

	y := make([]byte, ySize)
	u := make([]byte, uvSize)
	v := make([]byte, uvSize)

	// Color bar definitions (Y, U, V values)
	bars := []struct{ Y, U, V uint8 }{
		{235, 128, 128}, // White
		{210, 16, 146},  // Yellow
		{170, 166, 16},  // Cyan
		{145, 54, 34},   // Green
		{106, 202, 222}, // Magenta
		{81, 90, 240},   // Red
		{41, 240, 110},  // Blue
		{16, 128, 128},  // Black
	}

	barWidth := videoWidth / len(bars)

	for row := 0; row < videoHeight; row++ {
		for col := 0; col < videoWidth; col++ {
			barIndex := col / barWidth
			if barIndex >= len(bars) {
				barIndex = len(bars) - 1
			}

			y[row*videoWidth+col] = bars[barIndex].Y
		}
	}

	// Fill U and V planes
	for row := 0; row < videoHeight/2; row++ {
		for col := 0; col < videoWidth/2; col++ {
			barIndex := (col * 2) / barWidth
			if barIndex >= len(bars) {
				barIndex = len(bars) - 1
			}

			idx := row*(videoWidth/2) + col
			u[idx] = bars[barIndex].U
			v[idx] = bars[barIndex].V
		}
	}

	return y, u, v
}

// generateMovingGradient creates an animated color gradient
func (d *VideoCallDemo) generateMovingGradient(demo *VideoCallDemo) ([]byte, []byte, []byte) {
	ySize := videoWidth * videoHeight
	uvSize := (videoWidth / 2) * (videoHeight / 2)

	y := make([]byte, ySize)
	u := make([]byte, uvSize)
	v := make([]byte, uvSize)

	for row := 0; row < videoHeight; row++ {
		for col := 0; col < videoWidth; col++ {
			// Create moving gradient based on position and time
			x := float64(col) / float64(videoWidth)
			y_pos := float64(row) / float64(videoHeight)

			// Moving wave pattern
			wave := math.Sin(x*4*math.Pi+d.animationPhase*0.1) *
				math.Cos(y_pos*2*math.Pi+d.animationPhase*0.05)

			intensity := 0.5 + 0.5*wave
			y[row*videoWidth+col] = uint8(intensity * 255)
		}
	}

	// Generate animated chrominance
	for row := 0; row < videoHeight/2; row++ {
		for col := 0; col < videoWidth/2; col++ {
			idx := row*(videoWidth/2) + col

			// Rotating color phase
			colorPhase := d.animationPhase * 0.02
			u[idx] = uint8(128 + 64*math.Sin(colorPhase))
			v[idx] = uint8(128 + 64*math.Cos(colorPhase))
		}
	}

	return y, u, v
}

// generateCheckerboard creates an animated checkerboard pattern
func (d *VideoCallDemo) generateCheckerboard(demo *VideoCallDemo) ([]byte, []byte, []byte) {
	ySize := videoWidth * videoHeight
	uvSize := (videoWidth / 2) * (videoHeight / 2)

	y := make([]byte, ySize)
	u := make([]byte, uvSize)
	v := make([]byte, uvSize)

	// Animated checker size
	checkerSize := int(16 + 8*math.Sin(d.animationPhase*0.05))

	for row := 0; row < videoHeight; row++ {
		for col := 0; col < videoWidth; col++ {
			// Checkerboard logic with animation offset
			offset := int(d.animationPhase * 0.1)
			checker := ((row+offset)/checkerSize + (col+offset)/checkerSize) % 2

			if checker == 0 {
				y[row*videoWidth+col] = 235 // White
			} else {
				y[row*videoWidth+col] = 16 // Black
			}
		}
	}

	// Subtle color for checkerboard
	for row := 0; row < videoHeight/2; row++ {
		for col := 0; col < videoWidth/2; col++ {
			idx := row*(videoWidth/2) + col
			u[idx] = 128
			v[idx] = 128
		}
	}

	return y, u, v
}

// generatePlasmaEffect creates a retro plasma animation
func (d *VideoCallDemo) generatePlasmaEffect(demo *VideoCallDemo) ([]byte, []byte, []byte) {
	ySize := videoWidth * videoHeight
	uvSize := (videoWidth / 2) * (videoHeight / 2)

	y := make([]byte, ySize)
	u := make([]byte, uvSize)
	v := make([]byte, uvSize)

	time := d.animationPhase * 0.1

	for row := 0; row < videoHeight; row++ {
		for col := 0; col < videoWidth; col++ {
			x := float64(col) / 64.0
			y_pos := float64(row) / 64.0

			// Plasma formula
			plasma := math.Sin(x+time) +
				math.Sin(y_pos+time) +
				math.Sin((x+y_pos+time)/2) +
				math.Sin(math.Sqrt(x*x+y_pos*y_pos)+time)

			intensity := (plasma + 4) / 8 // Normalize to 0-1
			y[row*videoWidth+col] = uint8(intensity * 255)
		}
	}

	// Animated color for plasma
	for row := 0; row < videoHeight/2; row++ {
		for col := 0; col < videoWidth/2; col++ {
			idx := row*(videoWidth/2) + col

			colorTime := time * 2
			u[idx] = uint8(128 + 64*math.Sin(colorTime))
			v[idx] = uint8(128 + 64*math.Sin(colorTime+math.Pi/2))
		}
	}

	return y, u, v
}

// generateTestPattern creates a technical test pattern with information
func (d *VideoCallDemo) generateTestPattern(demo *VideoCallDemo) ([]byte, []byte, []byte) {
	ySize := videoWidth * videoHeight
	uvSize := (videoWidth / 2) * (videoHeight / 2)

	y := make([]byte, ySize)
	u := make([]byte, uvSize)
	v := make([]byte, uvSize)

	// Background
	for i := range y {
		y[i] = 128 // Mid-gray
	}

	// Draw borders
	for col := 0; col < videoWidth; col++ {
		y[col] = 235                            // Top border
		y[(videoHeight-1)*videoWidth+col] = 235 // Bottom border
	}
	for row := 0; row < videoHeight; row++ {
		y[row*videoWidth] = 235              // Left border
		y[row*videoWidth+videoWidth-1] = 235 // Right border
	}

	// Center crosshair
	centerX := videoWidth / 2
	centerY := videoHeight / 2
	for i := -10; i <= 10; i++ {
		if centerX+i >= 0 && centerX+i < videoWidth {
			y[centerY*videoWidth+centerX+i] = 16
		}
		if centerY+i >= 0 && centerY+i < videoHeight {
			y[(centerY+i)*videoWidth+centerX] = 16
		}
	}

	// Frame counter in corner (simple 8x8 digit patterns)
	_ = d.frameCount % 10000 // Frame number for future use
	// This is a simplified representation - in a real implementation
	// you'd draw actual numbers using a bitmap font

	// Neutral chrominance for test pattern
	for i := range u {
		u[i] = 128
		v[i] = 128
	}

	return y, u, v
}

// generateSimpleAudio creates basic audio for video demo
func (d *VideoCallDemo) generateSimpleAudio() []int16 {
	frame := make([]int16, audioFrameSize*audioChannels)

	// Simple 1kHz tone at low volume
	freq := 1000.0
	volume := 0.1
	time := float64(d.frameCount) * float64(audioFrameSize) / audioSampleRate

	for i := 0; i < audioFrameSize; i++ {
		sample := volume * math.Sin(2*math.Pi*freq*time)
		frame[i] = int16(sample * 32767)
		time += 1.0 / audioSampleRate
	}

	return frame
}

// sendVideoFrame generates and sends a video frame
func (d *VideoCallDemo) sendVideoFrame() {
	startTime := d.timeProvider.Now()

	// Generate frame using current pattern
	pattern := d.patterns[d.currentPattern]
	y, u, v := pattern.Generator(d)

	// Send video frame to friend 0 (demo purposes)
	friendNumber := uint32(0)

	if err := d.toxav.VideoSendFrame(friendNumber, videoWidth, videoHeight, y, u, v); err != nil {
		// Only log if it's not a "no call" error (expected when no calls active)
		if !errors.Is(err, toxcore.ErrNoActiveCall) {
			logrus.WithError(err).WithField("friend_number", friendNumber).Error("Video send error")
		}
	} else {
		processingTime := d.timeProvider.Since(startTime)
		d.stats.UpdateVideoSent(processingTime)
	}

	d.frameCount++
	d.animationPhase += 1.0
}

// sendAudioFrame generates and sends an audio frame
func (d *VideoCallDemo) sendAudioFrame() {
	frame := d.generateSimpleAudio()

	friendNumber := uint32(0)

	if err := d.toxav.AudioSendFrame(friendNumber, frame, audioFrameSize, audioChannels, audioSampleRate); err != nil {
		// Only log if it's not a "no call" error
		if !errors.Is(err, toxcore.ErrNoActiveCall) {
			logrus.WithError(err).WithField("friend_number", friendNumber).Error("Audio send error")
		}
	} else {
		d.stats.UpdateAudioSent()
	}
}

// Run starts the video call demonstration
// displayDemoIntroduction shows the demo startup information and current pattern.
func (d *VideoCallDemo) displayDemoIntroduction() {
	logrus.WithField("duration", demoDuration).Info("🎬 Starting video call demo")
	logrus.Info("📋 Video demo features:")
	logrus.Info("   • Multiple video patterns (color bars, gradients, checkerboard, plasma, test)")
	logrus.Info("   • Real-time video generation and processing")
	logrus.Info("   • Video frame analysis and statistics")
	logrus.Info("   • High-quality video calling (500 kbps)")
	logrus.Info("   • Animated video effects and patterns")

	// Show current pattern
	logrus.WithFields(logrus.Fields{
		"pattern_name":        d.patterns[d.currentPattern].Name,
		"pattern_description": d.patterns[d.currentPattern].Description,
	}).Info("🎨 Current pattern")
}

// bootstrapToNetwork connects to the Tox network using bootstrap nodes.
func (d *VideoCallDemo) bootstrapToNetwork() {
	err := d.tox.Bootstrap("node.tox.biribiri.org", 33445, "F404ABAA1C99A9D37D61AB54898F56793E1DEF8BD46B1038B9D822E8460FAB67")
	if err != nil {
		logrus.WithError(err).Warn("⚠️  Bootstrap warning")
	} else {
		logrus.Info("🌐 Connected to Tox network")
	}
}

// setupTimersAndChannels creates all required tickers and signal channels for the demo loop.
func (d *VideoCallDemo) setupTimersAndChannels() (chan os.Signal, *time.Ticker, *time.Ticker, *time.Ticker, *time.Ticker, *time.Ticker) {
	// Set up graceful shutdown
	sigChan := make(chan os.Signal, 1)
	signal.Notify(sigChan, os.Interrupt)

	// Timing for frame generation - use injectable time provider for tickers
	videoTicker := d.timeProvider.NewTicker(time.Second / videoFrameRate)                                  // 33ms for 30fps
	audioTicker := d.timeProvider.NewTicker(time.Duration(audioFrameSize) * time.Second / audioSampleRate) // 10ms
	statsTicker := d.timeProvider.NewTicker(10 * time.Second)
	patternTicker := d.timeProvider.NewTicker(15 * time.Second)  // Change pattern every 15s
	toxTicker := d.timeProvider.NewTicker(50 * time.Millisecond) // Tox iteration

	return sigChan, videoTicker, audioTicker, statsTicker, patternTicker, toxTicker
}

// handleStatisticsTick processes and displays video call statistics.
func (d *VideoCallDemo) handleStatisticsTick(startTime time.Time) {
	videoSent, audioSent, received, active, avgProcessing, _ := d.stats.GetStats()
	elapsed := d.timeProvider.Since(startTime)
	logrus.WithFields(logrus.Fields{
		"elapsed":        elapsed.Round(time.Second),
		"video_sent":     videoSent,
		"avg_processing": avgProcessing,
		"audio_sent":     audioSent,
		"received":       received,
		"active_calls":   active,
	}).Info("📊 Video Stats")
}

// switchToNextPattern changes to the next video pattern and displays the change.
func (d *VideoCallDemo) switchToNextPattern() {
	d.currentPattern = (d.currentPattern + 1) % len(d.patterns)
	logrus.WithFields(logrus.Fields{
		"pattern_name":        d.patterns[d.currentPattern].Name,
		"pattern_description": d.patterns[d.currentPattern].Description,
	}).Info("🎨 Switched to pattern")
}

func (d *VideoCallDemo) Run() {
	d.initializeDemo()
	timers := d.initializeTimers()
	defer d.cleanupTimers(timers)

	d.runEventLoop(timers)
	d.shutdown()
}

// initializeDemo sets up the demo environment and displays introduction.
func (d *VideoCallDemo) initializeDemo() {
	d.displayDemoIntroduction()
	d.bootstrapToNetwork()
}

// TimerSet holds all the tickers and channels used in the event loop.
type TimerSet struct {
	sigChan       chan os.Signal
	videoTicker   *time.Ticker
	audioTicker   *time.Ticker
	statsTicker   *time.Ticker
	patternTicker *time.Ticker
	toxTicker     *time.Ticker
	startTime     time.Time
}

// initializeTimers creates and configures all timers and channels for the demo.
func (d *VideoCallDemo) initializeTimers() *TimerSet {
	sigChan, videoTicker, audioTicker, statsTicker, patternTicker, toxTicker := d.setupTimersAndChannels()
	return &TimerSet{
		sigChan:       sigChan,
		videoTicker:   videoTicker,
		audioTicker:   audioTicker,
		statsTicker:   statsTicker,
		patternTicker: patternTicker,
		toxTicker:     toxTicker,
		startTime:     d.timeProvider.Now(),
	}
}

// cleanupTimers stops all active tickers to prevent resource leaks.
func (d *VideoCallDemo) cleanupTimers(timers *TimerSet) {
	timers.videoTicker.Stop()
	timers.audioTicker.Stop()
	timers.statsTicker.Stop()
	timers.patternTicker.Stop()
	timers.toxTicker.Stop()
}

// runEventLoop executes the main event processing loop with timeout handling.
func (d *VideoCallDemo) runEventLoop(timers *TimerSet) {
	logrus.Info("▶️  Video demo running - Press Ctrl+C to stop")

	for d.active {
		if d.processEvents(timers) {
			break
		}

		if d.checkTimeout(timers.startTime) {
			break
		}

		time.Sleep(1 * time.Millisecond)
	}
}

// processEvents handles incoming events from various channels and returns true if should exit.
func (d *VideoCallDemo) processEvents(timers *TimerSet) bool {
	select {
	case <-timers.sigChan:
		return d.handleShutdownSignal()
	case <-timers.videoTicker.C:
		d.sendVideoFrame()
	case <-timers.audioTicker.C:
		d.sendAudioFrame()
	case <-timers.patternTicker.C:
		d.switchToNextPattern()
	case <-timers.statsTicker.C:
		d.handleStatisticsTick(timers.startTime)
	case <-timers.toxTicker.C:
		d.handleToxEvents()
	default:
		return false
	}
	return false
}

// handleShutdownSignal processes shutdown signal and returns true to exit loop.
func (d *VideoCallDemo) handleShutdownSignal() bool {
	logrus.Info("🛑 Shutdown signal received")
	d.active = false
	return true
}

// handleToxEvents processes Tox and ToxAV network events.
func (d *VideoCallDemo) handleToxEvents() {
	d.tox.Iterate()
	d.toxav.Iterate()
}

// checkTimeout verifies if demo duration has been exceeded and returns true if should exit.
func (d *VideoCallDemo) checkTimeout(startTime time.Time) bool {
	if d.timeProvider.Since(startTime) > demoDuration {
		logrus.WithField("duration", demoDuration).Info("⏰ Video demo completed")
		d.active = false
		return true
	}
	return false
}

// shutdown cleans up resources
func (d *VideoCallDemo) shutdown() {
	logrus.Info("🧹 Cleaning up video demo...")

	videoSent, audioSent, received, active, avgProcessing, effects := d.stats.GetStats()
	logrus.WithFields(logrus.Fields{
		"video_frames_sent":     videoSent,
		"audio_frames_sent":     audioSent,
		"frames_received":       received,
		"avg_processing_time":   avgProcessing,
		"effects_applied":       effects,
		"active_calls_at_end":   active,
		"patterns_demonstrated": len(d.patterns),
	}).Info("📈 Final video statistics")

	if d.toxav != nil {
		d.toxav.Kill()
	}
	if d.tox != nil {
		d.tox.Kill()
	}
	logrus.Info("✅ Video demo cleanup completed")
}

func main() {
	logrus.Info("📹 ToxAV Video Call Demo")
	logrus.Info("========================")

	demo, err := NewVideoCallDemo()
	if err != nil {
		logrus.WithError(err).Fatal("❌ Failed to initialize video demo")
	}

	demo.Run()
	logrus.Info("👋 Video demo completed successfully")
}

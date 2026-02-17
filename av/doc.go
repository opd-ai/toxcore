// Package av implements audio/video calling functionality for ToxAV.
//
// This package provides a pure Go implementation of ToxAV that integrates
// seamlessly with the toxcore-go infrastructure including transport, crypto,
// DHT, and friend management systems. It enables secure real-time communication
// with adaptive quality and comprehensive monitoring.
//
// # Architecture
//
// The av package consists of several integrated subsystems:
//
//   - Manager: Orchestrates multiple concurrent calls with transport integration
//   - Call: Individual call instances with RTP session management
//   - QualityMonitor: Real-time call quality assessment and reporting
//   - PerformanceOptimizer: Dynamic optimization based on system resources
//   - AdaptationSystem: Network-aware bitrate adaptation
//   - MetricsAggregator: Statistics collection and analysis
//
// # Sub-Packages
//
// The av package includes specialized sub-packages:
//
//   - av/audio: Opus codec integration for high-quality audio encoding/decoding
//   - av/video: VP8 codec with effects, scaling, and RTP packetization
//   - av/rtp: Real-Time Protocol transport for media streaming
//
// # Manager Usage
//
// Create a manager with transport and friend lookup integration:
//
//	manager := av.NewManager()
//	manager.SetTransport(transport)
//	manager.SetFriendAddressLookup(func(friendNum uint32) ([]byte, error) {
//	    return tox.GetFriendAddress(friendNum)
//	})
//	manager.Start()
//	defer manager.Stop()
//
// # Making Calls
//
// Initiate and answer calls using the Manager:
//
//	// Initiate a call
//	err := manager.Call(friendNumber, audioBitRate, videoBitRate)
//
//	// Set up callback to handle incoming calls
//	manager.SetCallCallback(func(friendNum uint32, audio, video bool) {
//	    if acceptCall {
//	        manager.Answer(friendNum, audioBitRate, videoBitRate)
//	    } else {
//	        manager.CallControl(friendNum, av.CallControlCancel)
//	    }
//	})
//
// # Sending Media
//
// Send audio and video frames during an active call:
//
//	// Send audio (Opus-encoded PCM)
//	err := manager.SendAudioFrame(friendNumber, pcmData, sampleCount,
//	    channels, samplingRate)
//
//	// Send video (YUV420 format)
//	err := manager.SendVideoFrame(friendNumber, width, height,
//	    yPlane, uPlane, vPlane, yStride, uStride, vStride)
//
// # Receiving Media
//
// Register callbacks for received media frames:
//
//	manager.SetAudioReceiveCallback(func(friendNum uint32, pcm []int16,
//	    sampleCount int, channels uint8, samplingRate uint32) {
//	    // Process received audio
//	})
//
//	manager.SetVideoReceiveCallback(func(friendNum uint32, width, height uint16,
//	    y, u, v []byte, yStride, uStride, vStride int) {
//	    // Process received video
//	})
//
// # Call Control
//
// Control active calls with standard operations:
//
//	manager.CallControl(friendNumber, av.CallControlPause)    // Pause call
//	manager.CallControl(friendNumber, av.CallControlResume)   // Resume call
//	manager.CallControl(friendNumber, av.CallControlMuteAudio)// Mute audio
//	manager.CallControl(friendNumber, av.CallControlCancel)   // End call
//
// # Quality Monitoring
//
// Monitor call quality in real-time:
//
//	manager.SetQualityCallback(func(friendNum uint32, quality av.QualityLevel) {
//	    if quality == av.QualityPoor {
//	        // Consider reducing video quality or notifying user
//	    }
//	})
//
// Quality levels range from Excellent to Unacceptable based on:
//   - Packet loss rate
//   - Network jitter
//   - Round-trip time
//   - Frame timing consistency
//
// # Adaptive Bitrate
//
// The adaptation system automatically adjusts bitrates based on network
// conditions. Configuration is available via SetAdaptationConfig:
//
//	config := av.DefaultAdaptationConfig()
//	config.MinAudioBitRate = 6000   // 6 kbps minimum
//	config.MaxAudioBitRate = 64000  // 64 kbps maximum
//	config.MinVideoBitRate = 100000 // 100 kbps minimum
//	config.MaxVideoBitRate = 1000000// 1 Mbps maximum
//	manager.SetAdaptationConfig(config)
//
// # Call States
//
// Calls progress through defined states matching the ToxAV C API:
//
//	const (
//	    CallStateNone           // No active call
//	    CallStateError          // Call error occurred
//	    CallStateFinished       // Call ended normally
//	    CallStateSendingAudio   // Sending audio
//	    CallStateSendingVideo   // Sending video
//	    CallStateAcceptingAudio // Receiving audio
//	    CallStateAcceptingVideo // Receiving video
//	)
//
// # Signaling Protocol
//
// Call signaling uses compact wire formats over the Tox transport:
//
//   - CallRequestPacket: 20 bytes (call initiation)
//   - CallResponsePacket: 21 bytes (call answer)
//   - CallControlPacket: Call control messages
//
// # RTP Transport
//
// Media streaming uses RTP (RFC 3550) with:
//
//   - Separate audio/video streams with unique SSRCs
//   - Jitter buffering for smooth playback
//   - Sequence number tracking for loss detection
//   - Timestamp synchronization for audio/video sync
//
// # Thread Safety
//
// All Manager operations are thread-safe using sync.RWMutex. Callbacks are
// invoked from the manager's iteration goroutine. Long-running operations
// in callbacks should spawn separate goroutines to avoid blocking.
//
// # Integration with Tox
//
// The Manager integrates with the main Tox event loop:
//
//	for tox.IsRunning() {
//	    tox.Iterate()
//	    manager.Iterate() // Process AV events
//	    time.Sleep(tox.IterationInterval())
//	}
//
// # Error Handling
//
// Functions return descriptive errors wrapped with context. Common errors:
//
//	var (
//	    ErrCallNotFound    // No call exists for friend
//	    ErrCallAlreadyActive // Call already in progress
//	    ErrNotConnected    // Friend not connected
//	    ErrInvalidState    // Invalid operation for current state
//	)
package av

package toxcore

import (
	"testing"

	avpkg "github.com/opd-ai/toxcore/av"
)

// BenchmarkNewToxAV measures ToxAV instance creation performance
func BenchmarkNewToxAV(b *testing.B) {
	// Create a Tox instance for ToxAV
	options := NewOptions()
	options.UDPEnabled = true
	tox, err := New(options)
	if err != nil {
		b.Fatal(err)
	}
	defer tox.Kill()

	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		toxav, err := NewToxAV(tox)
		if err != nil {
			b.Fatal(err)
		}
		toxav.Kill()
	}
}

// BenchmarkToxAVIterate measures ToxAV iteration performance
func BenchmarkToxAVIterate(b *testing.B) {
	// Setup ToxAV instance
	options := NewOptions()
	options.UDPEnabled = true
	tox, err := New(options)
	if err != nil {
		b.Fatal(err)
	}
	defer tox.Kill()

	toxav, err := NewToxAV(tox)
	if err != nil {
		b.Fatal(err)
	}
	defer toxav.Kill()

	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		toxav.Iterate()
	}
}

// BenchmarkToxAVIterationInterval measures iteration interval calculation performance
func BenchmarkToxAVIterationInterval(b *testing.B) {
	// Setup ToxAV instance
	options := NewOptions()
	options.UDPEnabled = true
	tox, err := New(options)
	if err != nil {
		b.Fatal(err)
	}
	defer tox.Kill()

	toxav, err := NewToxAV(tox)
	if err != nil {
		b.Fatal(err)
	}
	defer toxav.Kill()

	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		_ = toxav.IterationInterval()
	}
}

// BenchmarkToxAVCall measures call initiation performance
func BenchmarkToxAVCall(b *testing.B) {
	// Setup ToxAV instance
	options := NewOptions()
	options.UDPEnabled = true
	tox, err := New(options)
	if err != nil {
		b.Fatal(err)
	}
	defer tox.Kill()

	toxav, err := NewToxAV(tox)
	if err != nil {
		b.Fatal(err)
	}
	defer toxav.Kill()

	// Use typical audio/video bitrates for VoIP
	const audioBitRate = 48  // 48 kbps for Opus
	const videoBitRate = 500 // 500 kbps for video calling

	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		// Note: This will fail as friend doesn't exist, but measures API overhead
		_ = toxav.Call(1, audioBitRate, videoBitRate)
	}
}

// BenchmarkToxAVAnswer measures call answering performance
func BenchmarkToxAVAnswer(b *testing.B) {
	// Setup ToxAV instance
	options := NewOptions()
	options.UDPEnabled = true
	tox, err := New(options)
	if err != nil {
		b.Fatal(err)
	}
	defer tox.Kill()

	toxav, err := NewToxAV(tox)
	if err != nil {
		b.Fatal(err)
	}
	defer toxav.Kill()

	// Use typical audio/video bitrates for VoIP
	const audioBitRate = 48  // 48 kbps for Opus
	const videoBitRate = 500 // 500 kbps for video calling

	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		// Note: This will fail as call doesn't exist, but measures API overhead
		_ = toxav.Answer(1, audioBitRate, videoBitRate)
	}
}

// BenchmarkToxAVCallControl measures call control performance
func BenchmarkToxAVCallControl(b *testing.B) {
	// Setup ToxAV instance
	options := NewOptions()
	options.UDPEnabled = true
	tox, err := New(options)
	if err != nil {
		b.Fatal(err)
	}
	defer tox.Kill()

	toxav, err := NewToxAV(tox)
	if err != nil {
		b.Fatal(err)
	}
	defer toxav.Kill()

	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		// Note: This will fail as call doesn't exist, but measures API overhead
		_ = toxav.CallControl(1, avpkg.CallControlResume)
	}
}

// BenchmarkToxAVAudioSetBitRate measures audio bitrate setting performance
func BenchmarkToxAVAudioSetBitRate(b *testing.B) {
	// Setup ToxAV instance
	options := NewOptions()
	options.UDPEnabled = true
	tox, err := New(options)
	if err != nil {
		b.Fatal(err)
	}
	defer tox.Kill()

	toxav, err := NewToxAV(tox)
	if err != nil {
		b.Fatal(err)
	}
	defer toxav.Kill()

	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		// Note: This will fail as call doesn't exist, but measures API overhead
		_ = toxav.AudioSetBitRate(1, 48)
	}
}

// BenchmarkToxAVVideoSetBitRate measures video bitrate setting performance
func BenchmarkToxAVVideoSetBitRate(b *testing.B) {
	// Setup ToxAV instance
	options := NewOptions()
	options.UDPEnabled = true
	tox, err := New(options)
	if err != nil {
		b.Fatal(err)
	}
	defer tox.Kill()

	toxav, err := NewToxAV(tox)
	if err != nil {
		b.Fatal(err)
	}
	defer toxav.Kill()

	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		// Note: This will fail as call doesn't exist, but measures API overhead
		_ = toxav.VideoSetBitRate(1, 500)
	}
}

// BenchmarkToxAVAudioSendFrame measures audio frame sending performance
func BenchmarkToxAVAudioSendFrame(b *testing.B) {
	// Setup ToxAV instance
	options := NewOptions()
	options.UDPEnabled = true
	tox, err := New(options)
	if err != nil {
		b.Fatal(err)
	}
	defer tox.Kill()

	toxav, err := NewToxAV(tox)
	if err != nil {
		b.Fatal(err)
	}
	defer toxav.Kill()

	// Create realistic audio frame data (10ms of 48kHz stereo audio)
	const sampleRate = 48000
	const channels = 2
	const frameDurationMs = 10
	const sampleCount = (sampleRate * frameDurationMs) / 1000 * channels
	pcm := make([]int16, sampleCount)

	// Fill with dummy audio data (sine wave)
	for i := range pcm {
		pcm[i] = int16(1000) // Simple constant amplitude for benchmark
	}

	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		// Note: This will fail as call doesn't exist, but measures processing overhead
		_ = toxav.AudioSendFrame(1, pcm, sampleCount/channels, channels, sampleRate)
	}
}

// BenchmarkToxAVVideoSendFrame measures video frame sending performance
func BenchmarkToxAVVideoSendFrame(b *testing.B) {
	// Setup ToxAV instance
	options := NewOptions()
	options.UDPEnabled = true
	tox, err := New(options)
	if err != nil {
		b.Fatal(err)
	}
	defer tox.Kill()

	toxav, err := NewToxAV(tox)
	if err != nil {
		b.Fatal(err)
	}
	defer toxav.Kill()

	// Create realistic video frame data (VGA 640x480 YUV420)
	const width = 640
	const height = 480
	ySize := width * height
	uvSize := (width * height) / 4

	y := make([]byte, ySize)
	u := make([]byte, uvSize)
	v := make([]byte, uvSize)

	// Fill with dummy video data
	for i := range y {
		y[i] = 128 // Gray level
	}
	for i := range u {
		u[i] = 128 // Neutral chroma
	}
	for i := range v {
		v[i] = 128 // Neutral chroma
	}

	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		// Note: This will fail as call doesn't exist, but measures processing overhead
		_ = toxav.VideoSendFrame(1, width, height, y, u, v)
	}
}

// BenchmarkToxAVCallbackRegistration measures callback registration performance
func BenchmarkToxAVCallbackRegistration(b *testing.B) {
	// Setup ToxAV instance
	options := NewOptions()
	options.UDPEnabled = true
	tox, err := New(options)
	if err != nil {
		b.Fatal(err)
	}
	defer tox.Kill()

	toxav, err := NewToxAV(tox)
	if err != nil {
		b.Fatal(err)
	}
	defer toxav.Kill()

	// Dummy callback functions
	callCallback := func(friendNumber uint32, audioEnabled, videoEnabled bool) {}
	stateCallback := func(friendNumber uint32, state avpkg.CallState) {}
	audioCallback := func(friendNumber uint32, pcm []int16, sampleCount int, channels uint8, samplingRate uint32) {}
	videoCallback := func(friendNumber uint32, width, height uint16, y, u, v []byte, yStride, uStride, vStride int) {}

	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		toxav.CallbackCall(callCallback)
		toxav.CallbackCallState(stateCallback)
		toxav.CallbackAudioReceiveFrame(audioCallback)
		toxav.CallbackVideoReceiveFrame(videoCallback)
	}
}

// BenchmarkToxAVConcurrentOperations measures performance under concurrent load
func BenchmarkToxAVConcurrentOperations(b *testing.B) {
	// Setup ToxAV instance
	options := NewOptions()
	options.UDPEnabled = true
	tox, err := New(options)
	if err != nil {
		b.Fatal(err)
	}
	defer tox.Kill()

	toxav, err := NewToxAV(tox)
	if err != nil {
		b.Fatal(err)
	}
	defer toxav.Kill()

	// Create test data
	pcm := make([]int16, 960) // 10ms of mono 48kHz audio
	y := make([]byte, 640*480)
	u := make([]byte, 320*240)
	v := make([]byte, 320*240)

	b.ResetTimer()
	b.RunParallel(func(pb *testing.PB) {
		for pb.Next() {
			// Simulate concurrent operations
			toxav.Iterate()
			_ = toxav.IterationInterval()
			_ = toxav.AudioSendFrame(1, pcm, 480, 1, 48000)
			_ = toxav.VideoSendFrame(1, 640, 480, y, u, v)
		}
	})
}

// BenchmarkToxAVMemoryProfile measures memory allocation patterns
func BenchmarkToxAVMemoryProfile(b *testing.B) {
	// Setup ToxAV instance
	options := NewOptions()
	options.UDPEnabled = true
	tox, err := New(options)
	if err != nil {
		b.Fatal(err)
	}
	defer tox.Kill()

	b.ResetTimer()
	b.ReportAllocs()

	for i := 0; i < b.N; i++ {
		toxav, err := NewToxAV(tox)
		if err != nil {
			b.Fatal(err)
		}

		// Perform typical operations to measure allocations
		toxav.Iterate()
		_ = toxav.IterationInterval()

		toxav.Kill()
	}
}

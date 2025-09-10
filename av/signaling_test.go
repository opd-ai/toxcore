package av

import (
	"testing"
	"time"
)

func TestCallRequestPacketSerialization(t *testing.T) {
	timestamp := time.Unix(1234567890, 123456789)
	
	original := &CallRequestPacket{
		CallID:       12345,
		AudioBitRate: 48000,
		VideoBitRate: 500000,
		Timestamp:    timestamp,
	}

	// Test serialization
	data, err := SerializeCallRequest(original)
	if err != nil {
		t.Fatalf("Failed to serialize call request: %v", err)
	}

	if len(data) != 20 {
		t.Errorf("Expected serialized data length 20, got %d", len(data))
	}

	// Test deserialization
	deserialized, err := DeserializeCallRequest(data)
	if err != nil {
		t.Fatalf("Failed to deserialize call request: %v", err)
	}

	// Verify all fields match
	if deserialized.CallID != original.CallID {
		t.Errorf("CallID mismatch: expected %d, got %d", original.CallID, deserialized.CallID)
	}
	if deserialized.AudioBitRate != original.AudioBitRate {
		t.Errorf("AudioBitRate mismatch: expected %d, got %d", original.AudioBitRate, deserialized.AudioBitRate)
	}
	if deserialized.VideoBitRate != original.VideoBitRate {
		t.Errorf("VideoBitRate mismatch: expected %d, got %d", original.VideoBitRate, deserialized.VideoBitRate)
	}
	if !deserialized.Timestamp.Equal(original.Timestamp) {
		t.Errorf("Timestamp mismatch: expected %v, got %v", original.Timestamp, deserialized.Timestamp)
	}
}

func TestCallResponsePacketSerialization(t *testing.T) {
	timestamp := time.Unix(1234567890, 123456789)
	
	tests := []struct {
		name     string
		accepted bool
	}{
		{"accepted call", true},
		{"rejected call", false},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			original := &CallResponsePacket{
				CallID:       12345,
				Accepted:     tt.accepted,
				AudioBitRate: 48000,
				VideoBitRate: 500000,
				Timestamp:    timestamp,
			}

			// Test serialization
			data, err := SerializeCallResponse(original)
			if err != nil {
				t.Fatalf("Failed to serialize call response: %v", err)
			}

			if len(data) != 21 {
				t.Errorf("Expected serialized data length 21, got %d", len(data))
			}

			// Test deserialization
			deserialized, err := DeserializeCallResponse(data)
			if err != nil {
				t.Fatalf("Failed to deserialize call response: %v", err)
			}

			// Verify all fields match
			if deserialized.CallID != original.CallID {
				t.Errorf("CallID mismatch: expected %d, got %d", original.CallID, deserialized.CallID)
			}
			if deserialized.Accepted != original.Accepted {
				t.Errorf("Accepted mismatch: expected %t, got %t", original.Accepted, deserialized.Accepted)
			}
			if deserialized.AudioBitRate != original.AudioBitRate {
				t.Errorf("AudioBitRate mismatch: expected %d, got %d", original.AudioBitRate, deserialized.AudioBitRate)
			}
			if deserialized.VideoBitRate != original.VideoBitRate {
				t.Errorf("VideoBitRate mismatch: expected %d, got %d", original.VideoBitRate, deserialized.VideoBitRate)
			}
			if !deserialized.Timestamp.Equal(original.Timestamp) {
				t.Errorf("Timestamp mismatch: expected %v, got %v", original.Timestamp, deserialized.Timestamp)
			}
		})
	}
}

func TestCallControlPacketSerialization(t *testing.T) {
	timestamp := time.Unix(1234567890, 123456789)
	
	controls := []CallControl{
		CallControlResume,
		CallControlPause,
		CallControlCancel,
		CallControlMuteAudio,
		CallControlUnmuteAudio,
		CallControlHideVideo,
		CallControlShowVideo,
	}

	for _, control := range controls {
		t.Run(control.String(), func(t *testing.T) {
			original := &CallControlPacket{
				CallID:      12345,
				ControlType: control,
				Timestamp:   timestamp,
			}

			// Test serialization
			data, err := SerializeCallControl(original)
			if err != nil {
				t.Fatalf("Failed to serialize call control: %v", err)
			}

			if len(data) != 13 {
				t.Errorf("Expected serialized data length 13, got %d", len(data))
			}

			// Test deserialization
			deserialized, err := DeserializeCallControl(data)
			if err != nil {
				t.Fatalf("Failed to deserialize call control: %v", err)
			}

			// Verify all fields match
			if deserialized.CallID != original.CallID {
				t.Errorf("CallID mismatch: expected %d, got %d", original.CallID, deserialized.CallID)
			}
			if deserialized.ControlType != original.ControlType {
				t.Errorf("ControlType mismatch: expected %v, got %v", original.ControlType, deserialized.ControlType)
			}
			if !deserialized.Timestamp.Equal(original.Timestamp) {
				t.Errorf("Timestamp mismatch: expected %v, got %v", original.Timestamp, deserialized.Timestamp)
			}
		})
	}
}

func TestBitrateControlPacketSerialization(t *testing.T) {
	timestamp := time.Unix(1234567890, 123456789)
	
	original := &BitrateControlPacket{
		CallID:       12345,
		AudioBitRate: 64000,
		VideoBitRate: 1000000,
		Timestamp:    timestamp,
	}

	// Test serialization
	data, err := SerializeBitrateControl(original)
	if err != nil {
		t.Fatalf("Failed to serialize bitrate control: %v", err)
	}

	if len(data) != 20 {
		t.Errorf("Expected serialized data length 20, got %d", len(data))
	}

	// Test deserialization
	deserialized, err := DeserializeBitrateControl(data)
	if err != nil {
		t.Fatalf("Failed to deserialize bitrate control: %v", err)
	}

	// Verify all fields match
	if deserialized.CallID != original.CallID {
		t.Errorf("CallID mismatch: expected %d, got %d", original.CallID, deserialized.CallID)
	}
	if deserialized.AudioBitRate != original.AudioBitRate {
		t.Errorf("AudioBitRate mismatch: expected %d, got %d", original.AudioBitRate, deserialized.AudioBitRate)
	}
	if deserialized.VideoBitRate != original.VideoBitRate {
		t.Errorf("VideoBitRate mismatch: expected %d, got %d", original.VideoBitRate, deserialized.VideoBitRate)
	}
	if !deserialized.Timestamp.Equal(original.Timestamp) {
		t.Errorf("Timestamp mismatch: expected %v, got %v", original.Timestamp, deserialized.Timestamp)
	}
}

func TestSerializationErrorHandling(t *testing.T) {
	// Test nil packet serialization
	t.Run("nil call request", func(t *testing.T) {
		_, err := SerializeCallRequest(nil)
		if err == nil {
			t.Error("Expected error for nil call request packet")
		}
	})

	t.Run("nil call response", func(t *testing.T) {
		_, err := SerializeCallResponse(nil)
		if err == nil {
			t.Error("Expected error for nil call response packet")
		}
	})

	t.Run("nil call control", func(t *testing.T) {
		_, err := SerializeCallControl(nil)
		if err == nil {
			t.Error("Expected error for nil call control packet")
		}
	})

	t.Run("nil bitrate control", func(t *testing.T) {
		_, err := SerializeBitrateControl(nil)
		if err == nil {
			t.Error("Expected error for nil bitrate control packet")
		}
	})
}

func TestDeserializationErrorHandling(t *testing.T) {
	// Test short packet deserialization
	tests := []struct {
		name    string
		data    []byte
		fn      func([]byte) (interface{}, error)
	}{
		{
			"short call request",
			make([]byte, 19),
			func(data []byte) (interface{}, error) { return DeserializeCallRequest(data) },
		},
		{
			"short call response",
			make([]byte, 20),
			func(data []byte) (interface{}, error) { return DeserializeCallResponse(data) },
		},
		{
			"short call control",
			make([]byte, 12),
			func(data []byte) (interface{}, error) { return DeserializeCallControl(data) },
		},
		{
			"short bitrate control",
			make([]byte, 19),
			func(data []byte) (interface{}, error) { return DeserializeBitrateControl(data) },
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			_, err := tt.fn(tt.data)
			if err == nil {
				t.Errorf("Expected error for %s", tt.name)
			}
		})
	}
}

// Benchmark tests for performance validation
func BenchmarkCallRequestSerialization(b *testing.B) {
	packet := &CallRequestPacket{
		CallID:       12345,
		AudioBitRate: 48000,
		VideoBitRate: 500000,
		Timestamp:    time.Now(),
	}

	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		_, err := SerializeCallRequest(packet)
		if err != nil {
			b.Fatal(err)
		}
	}
}

func BenchmarkCallRequestDeserialization(b *testing.B) {
	packet := &CallRequestPacket{
		CallID:       12345,
		AudioBitRate: 48000,
		VideoBitRate: 500000,
		Timestamp:    time.Now(),
	}
	
	data, err := SerializeCallRequest(packet)
	if err != nil {
		b.Fatal(err)
	}

	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		_, err := DeserializeCallRequest(data)
		if err != nil {
			b.Fatal(err)
		}
	}
}

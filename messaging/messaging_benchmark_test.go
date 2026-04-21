package messaging

import (
	"fmt"
	"testing"
)

// BenchmarkSendMessage measures the throughput of SendMessage under various
// message sizes.
func BenchmarkSendMessage(b *testing.B) {
	sizes := []struct {
		name string
		size int
	}{
		{"small_64B", 64},
		{"medium_512B", 512},
		{"large_1024B", 1024},
		{"max_1372B", 1372},
	}

	for _, s := range sizes {
		b.Run(s.name, func(b *testing.B) {
			mm := NewMessageManager()
			transport := &mockTransport{}
			mm.SetTransport(transport)

			text := make([]byte, s.size)
			for i := range text {
				text[i] = 'x'
			}
			payload := string(text)

			b.ResetTimer()
			b.ReportAllocs()
			for i := 0; i < b.N; i++ {
				_, err := mm.SendMessage(1, payload, MessageTypeNormal)
				if err != nil {
					b.Fatalf("SendMessage failed: %v", err)
				}
			}
		})
	}
}

// BenchmarkProcessPendingMessages measures the processing throughput of
// a queue filled with pending messages.
func BenchmarkProcessPendingMessages(b *testing.B) {
	queueSizes := []int{10, 100, 500}

	for _, qSize := range queueSizes {
		b.Run(fmt.Sprintf("queue_%d", qSize), func(b *testing.B) {
			b.ReportAllocs()

			for i := 0; i < b.N; i++ {
				b.StopTimer()
				mm := NewMessageManager()
				transport := &mockTransport{}
				mm.SetTransport(transport)

				// Pre-fill with messages in Pending state.
				for j := 0; j < qSize; j++ {
					_, err := mm.SendMessage(uint32(j%10), "benchmark message", MessageTypeNormal)
					if err != nil {
						b.Fatalf("SendMessage failed: %v", err)
					}
				}
				b.StartTimer()

				mm.ProcessPendingMessages()
			}
		})
	}
}

// BenchmarkMessageCreation measures the allocation cost of creating messages.
func BenchmarkMessageCreation(b *testing.B) {
	b.ReportAllocs()
	for i := 0; i < b.N; i++ {
		m := NewMessage(1, "hello world", MessageTypeNormal)
		_ = m
	}
}

// BenchmarkGetDeliveryStatus measures the lookup cost of delivery status
// when many messages are tracked.
func BenchmarkGetDeliveryStatus(b *testing.B) {
	const tracked = 1000

	mm := NewMessageManager()
	transport := &mockTransport{}
	mm.SetTransport(transport)

	ids := make([]uint32, 0, tracked)
	for i := 0; i < tracked; i++ {
		msg, err := mm.SendMessage(1, "bench", MessageTypeNormal)
		if err != nil {
			b.Fatalf("SendMessage failed: %v", err)
		}
		ids = append(ids, msg.GetID())
	}

	b.ResetTimer()
	b.ReportAllocs()
	for i := 0; i < b.N; i++ {
		id := ids[i%len(ids)]
		_ = mm.GetDeliveryStatus(1, id)
	}
}

// BenchmarkHandleDeliveryReceipt measures the cost of processing delivery
// receipts for messages in flight.
func BenchmarkHandleDeliveryReceipt(b *testing.B) {
	const tracked = 100

	mm := NewMessageManager()
	transport := &mockTransport{}
	mm.SetTransport(transport)

	ids := make([]uint32, 0, tracked)
	for i := 0; i < tracked; i++ {
		msg, err := mm.SendMessage(1, "bench", MessageTypeNormal)
		if err != nil {
			b.Fatalf("SendMessage failed: %v", err)
		}
		ids = append(ids, msg.GetID())
	}

	b.ResetTimer()
	b.ReportAllocs()
	for i := 0; i < b.N; i++ {
		id := ids[i%len(ids)]
		mm.HandleDeliveryReceipt(1, id, 1)
	}
}

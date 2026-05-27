package transport

import (
"sync"
"testing"
"time"
)

func TestPacketCountDebug(t *testing.T) {
transport1 := newSignalingMockTransport("127.0.0.1:8080")
transport2 := NewMockTransport("127.0.0.1:9090")
vn := NewVersionNegotiator([]ProtocolVersion{ProtocolLegacy, ProtocolNoiseIK}, ProtocolNoiseIK, 2*time.Second)

const callerCount = 8
results := make(chan ProtocolVersion, callerCount)
errs := make(chan error, callerCount)

var wg sync.WaitGroup
wg.Add(callerCount)
for i := 0; i < callerCount; i++ {
go func() {
defer wg.Done()
version, err := vn.NegotiateProtocol(transport1, transport2.LocalAddr())
if err != nil {
errs <- err
return
}
results <- version
}()
}

select {
case <-transport1.sentPacket:
case <-time.After(1 * time.Second):
t.Fatal("timed out waiting for negotiation packet to be sent")
}

// Wait a bit to see if more packets are sent
time.Sleep(200 * time.Millisecond)

packetCount := len(transport1.GetPackets())
t.Logf("Packets sent after first signal + 200ms: %d", packetCount)

vn.handleResponse(transport2.LocalAddr(), []ProtocolVersion{ProtocolLegacy, ProtocolNoiseIK})

done := make(chan struct{})
go func() {
wg.Wait()
close(done)
}()

select {
case <-done:
t.Log("All goroutines completed successfully")
case <-time.After(3 * time.Second):
finalPacketCount := len(transport1.GetPackets())
t.Fatalf("timed out waiting for concurrent negotiations to complete, packets: %d", finalPacketCount)
}

close(results)
close(errs)

for err := range errs {
t.Fatalf("concurrent negotiation failed: %v", err)
}

resultCount := 0
for version := range results {
resultCount++
if version != ProtocolNoiseIK {
t.Fatalf("expected negotiated version %d, got %d", ProtocolNoiseIK, version)
}
}

if resultCount != callerCount {
t.Fatalf("expected %d negotiation results, got %d", callerCount, resultCount)
}

finalPackets := len(transport1.GetPackets())
t.Logf("Final packet count: %d", finalPackets)
if finalPackets != 1 {
t.Fatalf("expected exactly 1 negotiation packet to be sent, got %d", finalPackets)
}
}

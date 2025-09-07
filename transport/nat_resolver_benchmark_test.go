package transport

import (
	"context"
	"testing"
)

func BenchmarkNATTraversal_DetectPublicAddress(b *testing.B) {
	nt := NewNATTraversal()

	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		_, err := nt.detectPublicAddress()
		if err != nil {
			// Expected in some environments, don't fail the benchmark
			continue
		}
	}
}

func BenchmarkNATTraversal_CalculateAddressScore(b *testing.B) {
	nt := NewNATTraversal()
	capabilities := NetworkCapabilities{
		SupportsDirectConnection: true,
		IsPrivateSpace:           false,
		RequiresProxy:            false,
		SupportsNAT:              true,
	}

	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		nt.calculateAddressScore(capabilities)
	}
}

func BenchmarkNATTraversal_AddressResolver_ResolvePublicAddress(b *testing.B) {
	nt := NewNATTraversal()
	addr := &mockAddr{network: "tcp", address: "8.8.8.8:80"}

	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		nt.addressResolver.ResolvePublicAddress(context.TODO(), addr)
	}
}

func BenchmarkNATTraversal_NetworkDetector_DetectCapabilities(b *testing.B) {
	nt := NewNATTraversal()
	addr := &mockAddr{network: "tcp", address: "8.8.8.8:80"}

	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		nt.networkDetector.DetectCapabilities(addr)
	}
}

func BenchmarkNATTraversal_Integration_DetectAndResolve(b *testing.B) {
	nt := NewNATTraversal()

	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		// Simulate the full pipeline: interface -> capabilities -> resolve
		interfaces, err := nt.getActiveInterfaces()
		if err != nil {
			continue
		}

		for _, iface := range interfaces {
			addr := nt.getAddressFromInterface(iface)
			if addr == nil {
				continue
			}

			// Detect capabilities
			capabilities := nt.networkDetector.DetectCapabilities(addr)

			// Calculate score
			_ = nt.calculateAddressScore(capabilities)

			// Resolve public address
			_, _ = nt.addressResolver.ResolvePublicAddress(context.TODO(), addr)

			break // Only test with first valid interface
		}
	}
}

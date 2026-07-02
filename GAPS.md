# Implementation Gaps — 2026-07-02

## End-to-End Runtime Bitrate Control Is Not Fully Implemented
- **Intended Behavior**: ToxAV bitrate update APIs should propagate effective bitrate changes across active calls, including signaling to peers and encoder reconfiguration.
- **Current State**: `AudioSetBitRate` and `VideoSetBitRate` update local call state only and explicitly do not transmit `BitrateControlPacket` or reconfigure encoders (`/home/runner/work/toxcore/toxcore/toxav.go:888-941`, `/home/runner/work/toxcore/toxcore/toxav.go:956-1010`).
- **Blocked Goal**: Fully functional adaptive bitrate behavior in live A/V sessions.
- **Implementation Path**: Implement control-packet emission, consume/update remote bitrate state, apply new rates to active audio/video encoders, and extend tests to verify cross-peer bitrate effect.
- **Dependencies**: Existing signaling packet serializers/deserializers in `av/signaling.go`; active call management in `av/manager.go`.
- **Effort**: medium

## Name Resolver Registration/Renewal Contract Is Stubbed
- **Intended Behavior**: `nameresolver.Resolver` contract includes lookup, register, and renew operations.
- **Current State**: `NmcdResolver.RegisterName` and `NmcdResolver.RenewName` return `ErrRegistrationNotSupported` unconditionally in Phase 1 (`/home/runner/work/toxcore/toxcore/nameresolver/nmcd.go:151-160`).
- **Blocked Goal**: Full Namecoin lifecycle support from this repository’s resolver abstraction.
- **Implementation Path**: Implement nmcd-backed registration and renewal transaction flows, including signing, submission, expiry handling, and integration tests for success/failure paths.
- **Dependencies**: nmcd client capabilities and secure key handling for signing.
- **Effort**: large

## Nym Listener Path Is Structurally Exposed but Not Implemented
- **Intended Behavior**: Transport abstraction (`NetworkTransport`) exposes listener semantics expected to support inbound network service patterns.
- **Current State**: `NymTransport.Listen` validates address and always returns unsupported error (`/home/runner/work/toxcore/toxcore/transport/nym_transport_impl.go:90-100`).
- **Blocked Goal**: Inbound/service-hosting parity for Nym transport within shared transport interfaces.
- **Implementation Path**: Integrate Nym service-node/listener support or split transport capability interfaces so unsupported operations are not exposed.
- **Dependencies**: Availability of Nym SDK/server-side interfaces and design decision on per-transport capability contracts.
- **Effort**: large

## Lokinet Listener/Packet Paths Are Exposed but Non-Operational
- **Intended Behavior**: Lokinet backend should satisfy advertised transport capabilities or expose a reduced capability contract.
- **Current State**: `LokinetTransport.Listen` and `LokinetTransport.DialPacket` always return unsupported errors (`/home/runner/work/toxcore/toxcore/transport/lokinet_transport_impl.go:81-92`, `/home/runner/work/toxcore/toxcore/transport/lokinet_transport_impl.go:147-156`).
- **Blocked Goal**: Full inbound and packet-mode support over Lokinet through common transport abstractions.
- **Implementation Path**: Implement SNApp/listener and packet-mode support, or refactor API contracts to encode capability differences and prevent unreachable calls.
- **Dependencies**: Lokinet daemon/SNApp capabilities and transport interface refactor if capability-splitting is chosen.
- **Effort**: large

## Audio Streaming Demo Adapter Is Not End-to-End Wired
- **Intended Behavior**: Demo should provide runnable reference of send/receive integration with the AV transport path.
- **Current State**: Demo adapter `Send` is a hard stub and `RegisterHandler` is a no-op (`/home/runner/work/toxcore/toxcore/examples/audio_streaming_demo/main.go:207-218`).
- **Blocked Goal**: Reliable example-driven onboarding for audio streaming integration.
- **Implementation Path**: Connect demo adapter to real transport send/handler registration, or explicitly downgrade demo scope in docs to non-runnable conceptual sample.
- **Dependencies**: Chosen demo transport backend and callback loop wiring.
- **Effort**: small

## Connectivity Documentation Drift in Address Support Comments
- **Intended Behavior**: Connectivity-support comments and status helpers should match actual transport behavior.
- **Current State**: Comment says Nym connectivity is stub-only while method currently reports supported outbound connectivity (`/home/runner/work/toxcore/toxcore/transport/address.go:220-241`).
- **Blocked Goal**: Accurate maintainability and user/operator understanding of support matrix.
- **Implementation Path**: Update stale comments and add tests that assert comment-level support claims reflected in status helper outputs.
- **Dependencies**: None.
- **Effort**: small

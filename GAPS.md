# Implementation Gaps — 2026-05-29

## SOCKS5 UDP proxying is best-effort rather than strict
- **Stated Goal**: README proxy configuration says SOCKS5 UDP can be enabled with `UDPProxyEnabled: true`, and multi-network transport promises proxy routing options.
- **Current State**: `transport.NewProxyTransport` disables UDP proxying if `NewSOCKS5UDPAssociation` fails, and `ProxyTransport.Send` then sends UDP directly through the underlying transport.
- **Impact**: Users who configure SOCKS5 UDP for privacy can still expose direct UDP traffic if association setup fails.
- **Closing the Gap**: Treat explicit UDP proxy setup failure as a hard error, or add an explicit opt-in insecure fallback flag with clear documentation and tests.

## Direct video encoder APIs do not validate strides before slicing
- **Stated Goal**: ToxAV video APIs and `av/video` package documentation present error-returning processing of YUV420 frames.
- **Current State**: `RealVP8Encoder.Encode` calls `packPlane`, which slices source planes using stride-derived bounds without validating stride consistency or backing buffer length.
- **Impact**: Applications using the exported video package directly can crash on invalid or strided frames instead of receiving an error.
- **Closing the Gap**: Add stride-aware validation to `VideoFrame` processing and make plane packing return errors that propagate through encoder and processor APIs.

## Nym transport is documented as implemented but still carries a not-implemented sentinel
- **Stated Goal**: README lists Nym `.nym` as dial-only multi-network transport, and `NymTransport` comments state Dial and DialPacket are implemented.
- **Current State**: `transport/nym_transport_impl.go` still defines `ErrNymNotImplemented` and `Listen` returns it for service hosting; this is partly aligned with dial-only README language but inconsistent with the broader transport implementation wording.
- **Impact**: Users may overestimate listener/service support or misunderstand whether Nym integration is complete.
- **Closing the Gap**: Clarify docs and API comments that only outbound stream and framed packet dialing are implemented via a local SOCKS5 Nym client, while inbound Nym service hosting is unsupported.

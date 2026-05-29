# Implementation Gaps — 2026-05-29

## Restartable Async Messaging Service
- **Stated Goal**: Asynchronous offline messaging is a core feature for real-world operation.
- **Current State**: `AsyncManager.Stop()` closes `stopChan`, but `Start()` does not recreate it (`/tmp/workspace/opd-ai/toxcore/async/manager.go`), so Start→Stop→Start leaves background loops immediately exiting.
- **Impact**: Async retrieval/discovery can appear started but be non-functional after lifecycle restart.
- **Closing the Gap**: Make Start/Stop lifecycle restart-safe by recreating stop channels and adding explicit restart regression tests.

## Reliable Local Discovery Lifecycle
- **Stated Goal**: LAN broadcast + mDNS fallback peer discovery should support resilient local connectivity.
- **Current State**: `LANDiscovery` and `MDNSDiscovery` close stop channels/cancel contexts on stop and do not recreate them on restart (`/tmp/workspace/opd-ai/toxcore/dht/local_discovery.go`, `/tmp/workspace/opd-ai/toxcore/dht/mdns_discovery.go`).
- **Impact**: Discovery can silently stop after a restart cycle while APIs still report successful start.
- **Closing the Gap**: Reinitialize lifecycle controls (`stopChan`, contexts) on each fresh start and add Start→Stop→Start tests for both discovery paths.

## Safe Public Constructor Error Semantics
- **Stated Goal**: Library-grade API usability for embedding applications.
- **Current State**: Exported bootstrap constructors panic on nil routing table (`/tmp/workspace/opd-ai/toxcore/dht/bootstrap.go:126`, `:159`, `:203`) instead of returning validation errors.
- **Impact**: Invalid caller input can terminate host processes unexpectedly.
- **Closing the Gap**: Replace panic-based validation in exported constructors with explicit error returns and update call sites/docs accordingly.

## Security Maintenance Feedback Loop
- **Stated Goal**: Strong cryptographic and secure transport posture.
- **Current State**: Dependency advisory exposure (`golang.org/x/crypto`, `golang.org/x/net`) is not currently validated in this environment due blocked vulnerability feed access; no local evidence of automated vuln-check gating in this audit run.
- **Impact**: Known upstream vulnerabilities can remain unnoticed during normal development workflows.
- **Closing the Gap**: Add CI `govulncheck ./...` (network-enabled) and enforce upgrade policy for vulnerable dependency ranges.

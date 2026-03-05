# Implementation Plan: Priority 2 — Function Length Reduction

## Implementation Log

**2026-03-05**: Completed `examples/toxav_audio_call/main.go:setupCallbacks` refactoring
- Extracted 70-line `setupCallbacks()` function into 8 smaller functions (all ≤16 lines, complexity ≤4.9)
- Functions created: `setupIncomingCallCallback`, `setupCallStateCallback`, `setupAudioCallbacks`, `setupAudioReceiveCallback`, `setupAudioBitrateCallback`, `setupVideoCallbacks`
- Complexity: All new functions ≤4.9 (well below threshold of 10)
- Validation: Zero regressions in unchanged code, documentation 92.77% (unchanged), overall trend improving
- Result: Functions over 30 lines reduced from 120 → 119

**2026-03-05**: Completed `examples/toxav_video_call/main.go:setupCallbacks` refactoring
- Extracted 104-line `setupCallbacks()` function (complexity 12.7) into 9 smaller functions (all ≤17 lines, complexity ≤4.4)
- Functions created: `setupIncomingCallCallback`, `answerVideoCall`, `setupCallStateCallback`, `handleCallFinished`, `setupVideoReceiveCallback`, `analyzeVideoFrame`, `calculateChannelAverage`, `setupAudioReceiveCallback`, `setupBitrateCallbacks`
- Complexity: `setupCallbacks()` reduced from 12.7 → 1.3 (89.8% improvement); all new functions ≤4.4 (well below threshold of 10)
- Validation: Zero regressions in target file, documentation 92.77% (unchanged), overall trend improving
- Result: Functions over 30 lines reduced from 124 → 123; functions over complexity 10 reduced from 19 → 18

**2026-03-05**: Completed `examples/file_transfer_demo/main.go` refactoring
- Extracted 84-line `main()` function (complexity 12.2) into 11 smaller functions (all ≤17 lines, complexity ≤3.1)
- Functions created: `configureLogging`, `createWorkingDirectory`, `createTestFile`, `createTransport`, `resolveFriendAddress`, `initiateFileTransfer`, `setupTransferCallbacks`, `startFileTransfer`, `sendFirstChunk`, `displayUsageNotes`
- Complexity: `main()` reduced from 12.2 → 1.3 (89.3% improvement); all new functions ≤3.1 (well below threshold of 10)
- Validation: Zero regressions, duplication 1.740% (unchanged), documentation 92.77% (unchanged)
- Result: Functions over 30 lines reduced from 125 → 124

**2026-03-05**: Completed `examples/audio_streaming_demo/main.go` refactoring
- Extracted 97-line `main()` function (complexity 12) into 10 smaller functions (all ≤17 lines, complexity ≤4)
- Functions created: `createUDPTransport`, `createRTPIntegration`, `createToxAVManager`, `startCall`, `setupMediaPipeline`, `sendAudioFrames`, `displayCallStatistics`, `endCall`, `displaySummary`
- Complexity: `main()` reduced from 12 → 1 (91.7% improvement); all new functions ≤4 (well below threshold of 10)
- Validation: Zero regressions, documentation 92.77% (unchanged)
- Result: Functions over 30 lines reduced from 127 → 126

**2026-03-05**: Completed `examples/address_demo/main.go` refactoring
- Extracted 101-line `main()` function into 7 smaller functions (all ≤16 lines)
- Functions created: `demonstrateIPv4Address`, `demonstratePublicIPv4Address`, `demonstrateMultiNetworkAddresses`, `createNetworkAddress`, `printNetworkAddressInfo`, `demonstrateAddressTypeDetection`
- Complexity: All new functions ≤3.1 (well below threshold of 10)
- Validation: Zero regressions, duplication 1.77% → 1.78% (negligible), documentation 92.77% (unchanged)
- Result: Functions over 30 lines reduced from 128 → 127

**2026-03-05**: Completed `examples/av_quality_monitor/main.go` refactoring
- Extracted 87-line `main()` function (complexity 7.0) into 12 smaller functions (all ≤15 lines, complexity ≤4.4)
- Functions created: `printHeader`, `initializeAggregator`, `createCallProfiles`, `createExcellentProfile`, `createGoodProfile`, `createFairProfile`, `createVariableProfile`, `startCallSimulations`, `waitForInterrupt`, `cleanupSimulations`, `displayFinalMetrics`, `simulateCallWithProfile`
- Complexity: `main()` reduced from 7.0 → 3.1 (55.7% improvement); all new functions ≤4.4 (well below threshold of 10)
- Validation: Zero regressions, duplication 1.741% → 1.741% (unchanged), documentation 92.77% (unchanged)
- Result: Functions over 30 lines reduced from 126 → 125

**2026-03-05**: Completed `examples/async_demo/main.go:demoStorageMaintenance` refactoring
- Extracted 91-line `demoStorageMaintenance()` function (complexity 15.3) into 8 smaller functions (all ≤17 lines, complexity ≤5.7)
- Functions created: `printStorageMaintenanceWarning`, `generateStorageDemoKeyPairs`, `storeInitialTestMessages`, `storeRawMessage`, `displayInitialStorageStats`, `storeAdditionalMessages`, `performCleanupDemonstration`, `displayFinalStorageStats`
- Complexity: `demoStorageMaintenance()` reduced from 15.3 → 1.3 (91.5% improvement); all new functions ≤5.7 (well below threshold of 10)
- Validation: Zero regressions in target file, documentation 92.77% (unchanged), overall trend improving
- Result: Functions over 30 lines reduced from 130 → 129 (note: baseline adjusted from audit data)

**2026-03-05**: Completed `examples/vp8_codec_demo/main.go:runPerformanceTest` refactoring
- Extracted 62-line `runPerformanceTest()` function (complexity 10.6) into 8 smaller functions (all ≤11 lines, complexity ≤4.9)
- Functions created: `definePerformanceTests`, `runSinglePerformanceTest`, `createPerformanceProcessor`, `warmUpProcessor`, `measureEncodePerformance`, `measureDecodePerformance`, `displayPerformanceResults`
- Complexity: `runPerformanceTest()` reduced from 10.6 → 3.1 (70.8% improvement); all new functions ≤4.9 (well below threshold of 10)
- Validation: Zero regressions, documentation 92.77% (unchanged), overall trend improving
- Result: Functions over 30 lines reduced from 121 → 120; functions over complexity 10 reduced from 121 → 120

**2026-03-05**: Completed `examples/multi_transport_demo/main.go:demonstrateIPTransport` refactoring
- Extracted 50-line `demonstrateIPTransport()` function (complexity 10.9) into 7 smaller functions (all ≤17 lines, complexity ≤5.7)
- Functions created: `createTCPListener`, `createUDPConnection`, `startEchoServer`, `echoData`, `testTCPConnection`, `sendMessage`, `receiveMessage`
- Complexity: `demonstrateIPTransport()` reduced from 10.9 → 4.4 (59.6% improvement); all new functions ≤5.7 (well below threshold of 10)
- Validation: Zero regressions in unchanged code, duplication 1.74% (unchanged), documentation 92.77% (unchanged), overall trend improving
- Result: Functions over 30 lines reduced from 119 → 118

**2026-03-05**: Completed `examples/toxav_effects_processing/main.go:handleAudioCommand` refactoring
- Extracted 49-line `handleAudioCommand()` function (complexity 12.7) into 5 smaller functions (all ≤16 lines, complexity ≤4.4)
- Functions created: `handleGainCommand`, `handleNoiseCommand`, `handleAGCCommand`, `handleResetCommand`
- Complexity: `handleAudioCommand()` reduced from 12.7 → 4.4 (65.4% improvement); all new functions ≤4.4 (well below threshold of 10)
- Validation: Zero regressions in target file, documentation 92.77% (unchanged), overall trend improving
- Result: Functions over 30 lines reduced from 118 → 117

## Phase Overview
- **Objective**: Reduce function length violations (>30 lines) to meet production readiness gate
- **Source Document**: ROADMAP.md (Priority 2: High — Function Length)
- **Prerequisites**: Priority 1 (Duplication) now passing at 1.77% (gate: <5%)
- **Estimated Scope**: Large — 128 functions above length threshold, concentrated in examples/ with some library code

## Metrics Summary
- **Complexity Hotspots**: 57 functions with overall complexity >9.0 (threshold)
- **Duplication Ratio**: 1.77% (below 5% gate — PASSING)
- **Documentation Coverage**: 92.77% (above 80% gate — PASSING)
- **Package Coupling**: `main` (6.0), `toxcore` (6.0), `async` (3.5), `crypto` (3.0), `transport` (3.0) — highest coupling packages

## Implementation Steps

### Step 1: Refactor High-Impact Example Main Functions
- **Deliverable**: Split monolithic `main()` functions into sub-functions with ≤30 lines each
- **Dependencies**: None
- **Metric Justification**: Top 5 longest functions are all example `main()` functions (101, 97, 87, 84, 82 lines)
- **Files**:
  - `examples/address_demo/main.go:main` — 101 lines → extract demo steps into named functions
  - `examples/audio_streaming_demo/main.go:main` — 97 lines, complexity 16.6 → extract audio setup, streaming loop, cleanup
  - `examples/av_quality_monitor/main.go:main` — 87 lines → extract monitoring setup and display logic
  - `examples/file_transfer_demo/main.go:main` — 84 lines, complexity 12.2 → extract transfer setup and progress handling

### Step 2: Refactor Example Callback Handlers
- **Deliverable**: Split callback setup functions by callback type into discrete functions
- **Dependencies**: Step 1 (establishes refactoring pattern for examples)
- **Metric Justification**: `setupCallbacks` functions range 44-82 lines with complexity 7.0-12.7
- **Files**:
  - `examples/toxav_video_call/main.go:setupCallbacks` — 82 lines, complexity 12.7 → split by callback type (audio/video/state)
  - `examples/toxav_audio_call/main.go:setupCallbacks` — 51 lines, complexity 10.1 → split by callback type
  - `examples/toxav_effects_processing/main.go:handleAudioCommand` — 49 lines, complexity 12.7 → extract per-command handlers
  - `examples/toxav_basic_call/main.go:setupCallbacks` — 44 lines → split by callback type

### Step 3: Refactor Secondary Example Functions
- **Deliverable**: Extract helper functions from long demo/utility functions
- **Dependencies**: Steps 1-2 (shared example patterns)
- **Metric Justification**: 8 additional example functions between 40-67 lines
- **Files**:
  - `examples/async_demo/main.go:demoStorageMaintenance` — 67 lines, complexity 15.3 → extract storage operation branches
  - `examples/vp8_codec_demo/main.go:main` — 65 lines, complexity 11.4 → extract codec initialization and demo loop
  - `examples/multi_transport_demo/main.go:demonstrateIPTransport` — 50 lines, complexity 10.9 → extract transport setup
  - `examples/vp8_codec_demo/main.go:runPerformanceTest` — 47 lines, complexity 10.6 → extract benchmark phases
  - `examples/audio_effects_demo/main.go:demonstrateProcessorIntegration` — 43 lines, complexity 10.9
  - `examples/av_quality_monitor/main.go:simulateCall` — 43 lines, complexity 11.1

### Step 4: Refactor Library Functions
- **Deliverable**: Extract helper functions from library code exceeding 30 lines
- **Dependencies**: None (independent of example refactoring)
- **Metric Justification**: Library functions affect API surface and should be prioritized for maintainability
- **Files**:
  - `av/audio/effects.go:NewNoiseSuppressionEffect` — 44 lines, complexity 5.7 → extract validation and initialization
  - `file/transfer.go:Start` — 42 lines, complexity 5.7 → extract state validation and transfer initialization

### Step 5: Address Remaining Example Functions (31-43 lines)
- **Deliverable**: Refactor functions in 31-43 line range to under 30 lines where practical
- **Dependencies**: Steps 1-4 (may use extracted helpers)
- **Metric Justification**: ~100+ functions in this range; prioritize those with complexity >9.0
- **Priority targets**:
  - Functions with complexity >9.0 in this range
  - Functions in frequently-referenced example files

## Technical Specifications
- **Refactoring approach**: Extract method pattern — identify logical blocks (setup, execution, cleanup) and extract to named functions
- **Naming convention**: Extracted functions should use descriptive names prefixed with context (e.g., `setupAudioCallbacks`, `initializeStreamingLoop`)
- **No behavioral changes**: Refactoring must preserve existing functionality; extract helpers should be unexported where appropriate
- **Test coverage**: Example code does not have test coverage to maintain; library refactoring should preserve existing test compatibility

## Validation Criteria
- [ ] `go-stats-generator analyze . --skip-tests | grep "Functions > 30 lines"` shows 0 violations
- [ ] `go-stats-generator analyze . --skip-tests --format json | jq '[.functions[] | select(.lines.code > 30)] | length'` returns 0
- [ ] `go-stats-generator diff baseline.json final.json` shows no regressions in:
  - Documentation coverage (must remain ≥92.77%)
  - Duplication ratio (must remain ≤1.77%)
  - Package coupling scores
- [ ] All existing tests pass: `go test ./...`
- [ ] Example programs compile and run: `go build ./examples/...`
- [ ] Complexity hotspots reduced: target ≤30 functions with complexity >9.0 after length refactoring

## Known Gaps
- **Gap 1: Example test coverage** — Examples lack automated tests; manual verification required after refactoring
  - **Impact**: Cannot automatically verify behavioral correctness of refactored examples
  - **Metrics Context**: Examples contribute 18/20 of the longest functions (90%)
  - **Resolution**: Manual smoke testing of each refactored example or add example integration tests

- **Gap 2: Complexity reduction coupling** — Function length and complexity are correlated; addressing length may not fully address complexity
  - **Impact**: Some functions may still exceed complexity threshold after length reduction
  - **Metrics Context**: 57 functions exceed complexity 9.0; overlap with long functions is ~40%
  - **Resolution**: Priority 5 (Complexity) phase will address remaining complexity violations after length reduction

## Progress Tracking

### Phase 1 Target Files (Priority Order)

| File | Function | Lines | Complexity | Status |
|------|----------|-------|------------|--------|
| examples/address_demo/main.go | main | 101 | 7.0 | ✅ Complete (2026-03-05) |
| examples/audio_streaming_demo/main.go | main | 97 | 16.6 | ✅ Complete (2026-03-05) |
| examples/av_quality_monitor/main.go | main | 87 | 7.0 | ✅ Complete (2026-03-05) |
| examples/file_transfer_demo/main.go | main | 84 | 12.2 | ✅ Complete (2026-03-05) |
| examples/toxav_video_call/main.go | setupCallbacks | 82 | 12.7 | ✅ Complete (2026-03-05) |
| examples/async_demo/main.go | demoStorageMaintenance | 67 | 15.3 | ✅ Complete (2026-03-05) |
| examples/vp8_codec_demo/main.go | runPerformanceTest | 62 | 10.6 | ✅ Complete (2026-03-05) |
| examples/toxav_audio_call/main.go | setupCallbacks | 70 | 10.1 | ✅ Complete (2026-03-05) |
| examples/multi_transport_demo/main.go | demonstrateIPTransport | 50 | 10.9 | ✅ Complete (2026-03-05) |
| examples/toxav_effects_processing/main.go | handleAudioCommand | 49 | 12.7 | ✅ Complete (2026-03-05) |

### Library Code Targets

| File | Function | Lines | Complexity | Status |
|------|----------|-------|------------|--------|
| av/audio/effects.go | NewNoiseSuppressionEffect | 44 | 5.7 | Pending |
| file/transfer.go | Start | 42 | 5.7 | Pending |

## Appendix: Metrics Snapshot

```
Generated: 2026-03-05
Tool: go-stats-generator v1.0.0 (--skip-tests)
Files Processed: 177

Current Gate Status:
  Duplication:    1.77%  ✅ PASS (gate: <5%)
  Documentation:  92.77% ✅ PASS (gate: ≥80%)
  Length (>30):   128    ❌ FAIL (gate: 0)
  Complexity (>9): 57    ❌ FAIL (gate: 0)

Target After This Phase:
  Length (>30):   0      ✅ PASS
  Complexity (>9): ~30   (partial improvement expected)
```

---

*Plan generated: 2026-03-05 | Source: ROADMAP.md Priority 2 | Metrics: go-stats-generator v1.0.0*

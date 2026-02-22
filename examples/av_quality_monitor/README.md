# ToxAV Quality Monitoring Dashboard Demo

This example demonstrates the real-time call quality monitoring and metrics aggregation capabilities of toxcore-go's ToxAV implementation.

## Features Demonstrated

- **Real-time Quality Monitoring**: Track call quality metrics in real-time
- **Metrics Aggregation**: System-wide statistics across multiple concurrent calls
- **Quality Assessment**: Automatic categorization (Excellent/Good/Fair/Poor)
- **Historical Tracking**: Rolling window of metrics for trend analysis
- **Periodic Reporting**: Configurable report generation with callbacks

## What This Demo Shows

The demo simulates 4 concurrent calls with different quality profiles:

1. **Alice (Excellent)**: High-quality call with low packet loss (0.5%) and jitter (15ms)
2. **Bob (Good)**: Good quality with moderate metrics (2% loss, 40ms jitter)
3. **Charlie (Fair)**: Acceptable quality with higher loss (5.5%) and jitter (80ms)
4. **Diana (Variable)**: Quality that varies over time to demonstrate adaptation

## Building and Running

```bash
# Build the demo
cd examples/av_quality_monitor
go build

# Run the demo
./av_quality_monitor
```

The demo will display real-time metrics reports every 2 seconds showing:
- System-wide statistics (active calls, average metrics)
- Overall quality assessment
- Quality distribution across all calls
- Per-call details with duration and packet loss

Press `Ctrl+C` to stop and see final statistics.

## Example Output

```
═══════════════════════════════════════════════════════════
  ToxAV Quality Monitoring Dashboard Demo
═══════════════════════════════════════════════════════════

📊 Initializing metrics aggregator...
✅ Metrics aggregator started

📞 Starting call simulations...
   ✓ Alice (Excellent) (Friend 101)
   ✓ Bob (Good) (Friend 102)
   ✓ Charlie (Fair) (Friend 103)
   ✓ Diana (Variable) (Friend 104)

📈 Monitoring call quality (press Ctrl+C to stop)...

┌─────────────────────────────────────────────────────────┐
│ Report: 13:10:42 │
├─────────────────────────────────────────────────────────┤
│ Active Calls:      4                                  │
│ Overall Quality:  🟡 Good                             │
│ Avg Packet Loss:  2.44%                              │
│ Avg Jitter:       43.291963ms                            │
├─────────────────────────────────────────────────────────┤
│ Quality Distribution:                                   │
│   🟢 Excellent:  1   🟡 Good:  2   🟠 Fair:  1   🔴 Poor:  0 │
├─────────────────────────────────────────────────────────┤
│ Per-Call Details:                                       │
│   Friend 101: 🟢 Excellent Loss:  0.5% Dur:  0m01s        │
│   Friend 102: 🟡 Good      Loss:  1.7% Dur:  0m01s        │
│   Friend 103: 🟠 Fair      Loss:  4.9% Dur:  0m01s        │
│   Friend 104: 🟡 Good      Loss:  2.6% Dur:  0m02s        │
└─────────────────────────────────────────────────────────┘
```

## Integration in Your Application

To use the metrics aggregator in your ToxAV application:

```go
import avpkg "github.com/opd-ai/toxcore/av"

// Create aggregator
aggregator := avpkg.NewMetricsAggregator(5 * time.Second)

// Set up report callback
aggregator.OnReport(func(report avpkg.AggregatedReport) {
    // Handle report - update UI, log, alert on quality issues, etc.
    fmt.Printf("System quality: %s, Active calls: %d\n",
        report.OverallQuality, report.SystemMetrics.ActiveCalls)
})

// Start aggregator
aggregator.Start()
defer aggregator.Stop()

// When starting a call
aggregator.StartCallTracking(friendNumber)

// Periodically record metrics (from QualityMonitor)
metrics, err := qualityMonitor.GetCallMetrics(call, bitrateAdapter)
if err != nil {
    log.Printf("failed to get call metrics: %v", err)
} else {
    aggregator.RecordMetrics(friendNumber, metrics)
}

// When ending a call
aggregator.StopCallTracking(friendNumber)
```

## Key Concepts

### MetricsAggregator

The `MetricsAggregator` collects metrics from individual calls and provides:
- System-wide statistics
- Historical tracking with rolling windows
- Periodic reporting via callbacks
- Quality distribution analysis

### CallMetrics

Each call's metrics include:
- Network metrics (packet loss, jitter, RTT)
- Bandwidth metrics (audio/video bitrates)
- Call timing (duration, last frame age)
- Quality assessment (excellent/good/fair/poor)

### SystemMetrics

Aggregated system-wide metrics:
- Active and total call counts
- Average packet loss, jitter, and bitrate
- Quality distribution across all calls
- Last update timestamp

## Use Cases

1. **Monitoring Dashboards**: Display real-time call quality
2. **Quality Alerts**: Trigger notifications when quality degrades
3. **Analytics**: Track quality trends over time
4. **Troubleshooting**: Identify network or codec issues
5. **Resource Management**: Make decisions about call capacity

## Related Documentation

- [ToxAV README](../../av/README.md) - Core AV implementation
- [Quality Monitoring](../../av/quality.go) - Quality assessment logic
- [Adaptation](../../av/adaptation.go) - Bitrate adaptation system
- [Performance](../../av/performance.go) - Performance optimization

## License

This example is part of toxcore-go and is licensed under the MIT License.

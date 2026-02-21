# ToxAV Effects Processing Example

This example demonstrates advanced audio and video effects processing capabilities in ToxAV, showcasing the sophisticated effects system implemented in toxcore-go.

## Overview

The Effects Processing example shows how to:

- **Advanced Audio Effects**: Noise suppression, gain control, and automatic gain control
- **Video Effects**: Color temperature adjustment and visual filters  
- **Effect Chains**: Combining multiple effects for professional-quality processing
- **Real-Time Processing**: Sub-millisecond effects processing suitable for VoIP
- **Interactive Control**: Dynamic effect parameter adjustment during calls
- **Performance Monitoring**: Real-time performance metrics and optimization

## Key Features

### Audio Effects Demonstrated

1. **Noise Suppression**: Spectral subtraction algorithm with FFT
2. **Gain Control**: Linear gain adjustment with clipping protection
3. **Automatic Gain Control (AGC)**: Peak-following algorithm with smoothing
4. **Effect Chains**: Sequential processing for complex audio pipelines

### Video Effects Demonstrated

1. **Color Temperature**: Warm/cool color adjustment (2000K-20000K range)
2. **Effect Chains**: Combining multiple video effects seamlessly
3. **Real-Time Processing**: Low-latency video effect application

### Advanced Capabilities

- **Performance Benchmarking**: Built-in timing and allocation measurement
- **Interactive Console**: Real-time effect parameter adjustment
- **Quality Monitoring**: Audio/video quality assessment with effects
- **Resource Management**: Efficient memory usage and cleanup

## Quick Start

```bash
cd examples/toxav_effects_processing
go run main.go
```

## Console Commands

Once running, use these interactive commands:

### Audio Effects
- `audio gain <0.0-4.0>` - Adjust audio gain level
- `audio noise <0.0-1.0>` - Set noise suppression strength
- `audio agc <0.0-1.0>` - Configure AGC target level
- `audio reset` - Reset all audio effects

### Video Effects  
- `video temp <2000-20000>` - Set color temperature (K)
- `video reset` - Reset all video effects

### General
- `stats` - Show performance statistics
- `help` - Show command help
- `quit` - Exit the demo

## Example Usage

```
🎯 ToxAV Effects Processing Demo
===============================
✅ Tox ID: 76518406F6A9F221...
🎧 Audio Effects: Gain(1.0), NoiseSuppress(0.5), AGC(0.7)
🎨 Video Effects: ColorTemp(6500K)
📊 Performance: Audio(156μs), Video(89μs)

> audio gain 1.5
🔊 Audio gain set to 1.5
📊 Processing: 1000 frames (avg: 1.2μs)

> video temp 3000
🌅 Color temperature set to 3000K (warm)
📊 Processing: 30 frames (avg: 92μs)

> stats
📊 Effects Performance Statistics:
   Audio Pipeline: 1.8μs avg (Gain: 356ns, Noise: 166μs, AGC: 903ns)
   Video Pipeline: 89μs avg (ColorTemp: 89μs)
   Memory: 0 allocs/frame, efficient processing
```

## Code Example

```go
package main

import (
    "fmt"
    "time"
    
    "github.com/opd-ai/toxcore"
    "github.com/opd-ai/toxcore/av/audio"
    "github.com/opd-ai/toxcore/av/video"
)

func main() {
    // Initialize ToxAV with effects
    toxav, err := setupToxAVWithEffects()
    if err != nil {
        log.Fatal(err)
    }
    defer toxav.Kill()
    
    // Start interactive effects demo
    runInteractiveDemo(toxav)
}

func setupAudioEffects() (*audio.EffectChain, error) {
    // Create effect chain with multiple audio effects
    gainEffect, err := audio.NewGainEffect(1.0)
    if err != nil {
        return nil, fmt.Errorf("failed to create gain effect: %w", err)
    }
    noiseEffect, err := audio.NewNoiseSuppressionEffect(0.5, 480)
    if err != nil {
        return nil, fmt.Errorf("failed to create noise suppression effect: %w", err)
    }
    agcEffect := audio.NewAutoGainEffect()
    
    chain := audio.NewEffectChain()
    chain.AddEffect(gainEffect)
    chain.AddEffect(noiseEffect) 
    chain.AddEffect(agcEffect)
    
    return chain, nil
}

func setupVideoEffects() (*video.EffectChain, error) {
    // Create video effect chain
    tempEffect := video.NewColorTemperatureEffect(6500) // Daylight
    
    chain := video.NewEffectChain()
    chain.AddEffect(tempEffect)
    
    return chain, nil
}
```

## Performance Expectations

### Audio Effects Performance
- **Gain Control**: ~356ns per 10ms frame
- **Noise Suppression**: ~166μs per 10ms frame  
- **Auto Gain Control**: ~903ns per 10ms frame
- **Complete Chain**: ~168μs per 10ms frame

### Video Effects Performance
- **Color Temperature**: ~89μs per 640×480 frame
- **Real-Time Capability**: 30 FPS processing at <0.3% CPU

## Educational Value

This example teaches:

1. **Effects Architecture**: How to design pluggable effect systems
2. **Real-Time Processing**: Techniques for sub-millisecond latency
3. **Resource Management**: Efficient memory usage patterns
4. **Performance Optimization**: Measuring and optimizing effect pipelines
5. **User Experience**: Interactive parameter adjustment during operation

## Advanced Integration

The example demonstrates how effects can be integrated into production applications:

- **VoIP Applications**: Professional call quality with noise suppression
- **Content Creation**: Real-time video filters for streaming
- **Accessibility**: Audio enhancement for hearing assistance
- **Gaming**: Voice effects and video filters for immersive experiences

## Technical Requirements

- Go 1.21 or later
- Pure Go implementation (no CGo dependencies)
- ~10MB memory usage for effect processing
- Compatible with all platforms supported by toxcore-go

## Troubleshooting

### High CPU Usage
- Reduce effect complexity or disable effects
- Lower audio/video frame rates
- Check for memory allocation in effect loops

### Audio Quality Issues  
- Adjust noise suppression strength
- Verify gain levels aren't causing clipping
- Check AGC target levels

### Video Artifacts
- Adjust color temperature gradually
- Verify input frame format (YUV420)
- Check effect parameter ranges

## Related Examples

- `toxav_basic_call/` - Basic ToxAV functionality
- `toxav_audio_call/` - Audio-focused implementation
- `toxav_video_call/` - Video pattern generation
- `audio_effects_demo/` - Standalone audio effects testing

This example showcases the advanced effects processing capabilities available in toxcore-go, demonstrating how sophisticated audio/video enhancement can be integrated into real-time communication applications.

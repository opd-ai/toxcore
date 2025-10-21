package main

import (
	"fmt"
	"log"

	"github.com/opd-ai/toxforge/av/video"
)

func main() {
	fmt.Println("ToxAV Video Effects Demo - Color Temperature Adjustment")
	fmt.Println("======================================================")

	// Create a video processor
	processor := video.NewProcessor()
	defer processor.Close()

	// Get the effect chain
	effectChain := processor.GetEffectChain()

	// Demonstrate different color temperature effects
	fmt.Println("\n1. Adding warm color temperature effect (+50)")
	warmEffect := video.NewColorTemperatureEffect(50)
	effectChain.AddEffect(warmEffect)
	fmt.Printf("   Effect name: %s\n", warmEffect.GetName())

	fmt.Println("\n2. Adding cool color temperature effect (-30)")
	coolEffect := video.NewColorTemperatureEffect(-30)
	effectChain.AddEffect(coolEffect)
	fmt.Printf("   Effect name: %s\n", coolEffect.GetName())

	fmt.Println("\n3. Adding neutral color temperature effect (0)")
	neutralEffect := video.NewColorTemperatureEffect(0)
	effectChain.AddEffect(neutralEffect)
	fmt.Printf("   Effect name: %s\n", neutralEffect.GetName())

	fmt.Printf("\nTotal effects in chain: %d\n", effectChain.GetEffectCount())

	// Create a test frame for demonstration
	frame := &video.VideoFrame{
		Width:  640,
		Height: 480,
		Y:      make([]byte, 640*480),
		U:      make([]byte, 640*480/4),
		V:      make([]byte, 640*480/4),
	}

	// Fill with test pattern
	for i := range frame.Y {
		frame.Y[i] = 128 // Gray
	}
	for i := range frame.U {
		frame.U[i] = 128 // Neutral chroma
		frame.V[i] = 128 // Neutral chroma
	}

	fmt.Println("\n4. Applying effects to test frame...")
	result, err := effectChain.Apply(frame)
	if err != nil {
		log.Fatal("Failed to apply effects:", err)
	}

	fmt.Printf("   Original frame: %dx%d, Y[0]=%d, U[0]=%d, V[0]=%d\n",
		frame.Width, frame.Height, frame.Y[0], frame.U[0], frame.V[0])
	fmt.Printf("   Processed frame: %dx%d, Y[0]=%d, U[0]=%d, V[0]=%d\n",
		result.Width, result.Height, result.Y[0], result.U[0], result.V[0])

	fmt.Println("\n5. Clearing effect chain...")
	effectChain.Clear()
	fmt.Printf("   Effects remaining: %d\n", effectChain.GetEffectCount())

	fmt.Println("\nDemo completed successfully!")
	fmt.Println("\nUsage in video calling applications:")
	fmt.Println("- Use positive values (+1 to +100) for warmer colors (more red/yellow)")
	fmt.Println("- Use negative values (-1 to -100) for cooler colors (more blue)")
	fmt.Println("- Use 0 for neutral/no adjustment")
	fmt.Println("- Values are automatically clamped to the valid range")
}

package main

import (
	"fmt"

	"github.com/pion/opus"
)

func main() {
	// Test Decoder
	decoder := opus.NewDecoder()
	fmt.Printf("Decoder created: %T\n", decoder)

	// Try to find Encoder
	// encoder := opus.NewEncoder() // Let's see if this exists
}

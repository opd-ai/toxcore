// Package common provides shared utilities for toxcore examples.

package common

import (
	"os"
	"os/signal"
	"syscall"
)

// SetupSignalHandler creates a channel that receives os.Interrupt and SIGTERM
// signals. This provides a standard way to handle graceful shutdown in examples.
//
// Usage:
//
//	sigChan := common.SetupSignalHandler()
//	// In main loop:
//	select {
//	case <-sigChan:
//	    fmt.Println("Shutting down...")
//	    return
//	// ... other cases
//	}
func SetupSignalHandler() chan os.Signal {
	sigChan := make(chan os.Signal, 1)
	signal.Notify(sigChan, os.Interrupt, syscall.SIGTERM)
	return sigChan
}

// SetupInterruptHandler is a simpler version that only handles os.Interrupt.
// Use this for demos that don't need to handle SIGTERM.
func SetupInterruptHandler() chan os.Signal {
	sigChan := make(chan os.Signal, 1)
	signal.Notify(sigChan, os.Interrupt)
	return sigChan
}

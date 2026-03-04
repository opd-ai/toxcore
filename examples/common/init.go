// Package common provides shared initialization utilities for toxcore examples.
// It consolidates common patterns like Tox/ToxAV instance creation to reduce
// code duplication across example applications.
package common

import (
	"fmt"
	"log"

	"github.com/opd-ai/toxcore"
)

// InitConfig holds configuration for Tox initialization.
type InitConfig struct {
	Name          string
	StatusMessage string
	UDPEnabled    bool
}

// DefaultInitConfig returns a sensible default configuration.
func DefaultInitConfig() InitConfig {
	return InitConfig{
		Name:          "ToxAV Demo",
		StatusMessage: "Running ToxAV Demo",
		UDPEnabled:    true,
	}
}

// InitToxWithAV creates and initializes both Tox and ToxAV instances using the
// provided configuration. Returns the initialized instances and a cleanup function
// that should be called via defer to properly shut down both instances.
//
// Example usage:
//
//	tox, toxav, cleanup, err := common.InitToxWithAV(common.InitConfig{
//	    Name:          "My Demo",
//	    StatusMessage: "Running My Demo",
//	    UDPEnabled:    true,
//	})
//	if err != nil {
//	    log.Fatal(err)
//	}
//	defer cleanup()
func InitToxWithAV(cfg InitConfig) (*toxcore.Tox, *toxcore.ToxAV, func(), error) {
	options := toxcore.NewOptions()
	options.UDPEnabled = cfg.UDPEnabled

	tox, err := toxcore.New(options)
	if err != nil {
		return nil, nil, nil, fmt.Errorf("failed to create Tox instance: %w", err)
	}

	if err := tox.SelfSetName(cfg.Name); err != nil {
		log.Printf("Warning: Failed to set name: %v", err)
	}

	if err := tox.SelfSetStatusMessage(cfg.StatusMessage); err != nil {
		log.Printf("Warning: Failed to set status message: %v", err)
	}

	toxav, err := toxcore.NewToxAV(tox)
	if err != nil {
		tox.Kill()
		return nil, nil, nil, fmt.Errorf("failed to create ToxAV instance: %w", err)
	}

	cleanup := func() {
		toxav.Kill()
		tox.Kill()
	}

	return tox, toxav, cleanup, nil
}

package main

import (
	"context"
	"flag"
	"fmt"
	"os"
	"os/signal"
	"syscall"

	"github.com/opd-ai/toxcore/bootstrap"
	"github.com/sirupsen/logrus"
)

func main() {
	var (
		port    = flag.Uint("port", 33445, "UDP port for the clearnet bootstrap service")
		onion   = flag.Bool("onion", false, "Enable Tor onion service (requires Tor daemon with control port)")
		i2p     = flag.Bool("i2p", false, "Enable I2P service (requires I2P router with SAM bridge)")
		verbose = flag.Bool("v", false, "Enable verbose logging")
	)
	flag.Parse()

	if *verbose {
		logrus.SetLevel(logrus.DebugLevel)
	} else {
		logrus.SetLevel(logrus.WarnLevel)
	}

	cfg := bootstrap.DefaultConfig()
	cfg.ClearnetEnabled = true
	cfg.ClearnetPort = uint16(*port)
	cfg.OnionEnabled = *onion
	cfg.I2PEnabled = *i2p

	srv, err := bootstrap.New(cfg)
	if err != nil {
		fmt.Fprintf(os.Stderr, "Failed to create bootstrap server: %v\n", err)
		os.Exit(1)
	}

	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	fmt.Println("Starting bootstrap server…")
	if *onion {
		fmt.Println("  Waiting for Tor onion service to be published (may take 30–90 s)…")
	}
	if *i2p {
		fmt.Println("  Waiting for I2P tunnel to be established (may take 2–5 min)…")
	}

	if err := srv.Start(ctx); err != nil {
		fmt.Fprintf(os.Stderr, "Failed to start bootstrap server: %v\n", err)
		os.Exit(1)
	}
	defer srv.Stop() //nolint:errcheck

	fmt.Println()
	fmt.Println("Bootstrap server is running.")
	fmt.Println()
	fmt.Printf("  Public key : %s\n", srv.GetPublicKeyHex())
	if addr := srv.GetClearnetAddr(); addr != "" {
		fmt.Printf("  Clearnet   : %s\n", addr)
	}
	if addr := srv.GetOnionAddr(); addr != "" {
		fmt.Printf("  Onion      : %s\n", addr)
	}
	if addr := srv.GetI2PAddr(); addr != "" {
		fmt.Printf("  I2P        : %s\n", addr)
	}
	fmt.Println()
	fmt.Println("Press Ctrl-C to stop.")

	// Block until SIGINT or SIGTERM.
	sigCh := make(chan os.Signal, 1)
	signal.Notify(sigCh, syscall.SIGINT, syscall.SIGTERM)
	<-sigCh

	fmt.Println("\nShutting down…")
}

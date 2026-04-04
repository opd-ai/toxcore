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
	cfg, verbose := parseFlags()
	configureLogging(verbose)

	srv, err := createBootstrapServer(cfg)
	if err != nil {
		os.Exit(1)
	}

	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	printStartupMessages(cfg)

	if err := startServer(srv, ctx); err != nil {
		os.Exit(1)
	}
	defer srv.Stop() //nolint:errcheck

	printServerInfo(srv)
	waitForShutdown()
	fmt.Println("\nShutting down…")
}

// parseFlags parses command-line flags and returns configuration.
func parseFlags() (*bootstrap.Config, bool) {
	var (
		port    = flag.Uint("port", 33445, "UDP port for the clearnet bootstrap service")
		onion   = flag.Bool("onion", false, "Enable Tor onion service (requires Tor daemon with control port)")
		i2p     = flag.Bool("i2p", false, "Enable I2P service (requires I2P router with SAM bridge)")
		verbose = flag.Bool("v", false, "Enable verbose logging")
	)
	flag.Parse()

	cfg := bootstrap.DefaultConfig()
	cfg.ClearnetEnabled = true
	cfg.ClearnetPort = uint16(*port)
	cfg.OnionEnabled = *onion
	cfg.I2PEnabled = *i2p

	return cfg, *verbose
}

// configureLogging sets the log level based on verbosity.
func configureLogging(verbose bool) {
	if verbose {
		logrus.SetLevel(logrus.DebugLevel)
	} else {
		logrus.SetLevel(logrus.WarnLevel)
	}
}

// createBootstrapServer creates a new bootstrap server with the given config.
func createBootstrapServer(cfg *bootstrap.Config) (*bootstrap.Server, error) {
	srv, err := bootstrap.New(cfg)
	if err != nil {
		fmt.Fprintf(os.Stderr, "Failed to create bootstrap server: %v\n", err)
		return nil, err
	}
	return srv, nil
}

// printStartupMessages prints startup status messages.
func printStartupMessages(cfg *bootstrap.Config) {
	fmt.Println("Starting bootstrap server…")
	if cfg.OnionEnabled {
		fmt.Println("  Waiting for Tor onion service to be published (may take 30–90 s)…")
	}
	if cfg.I2PEnabled {
		fmt.Println("  Waiting for I2P tunnel to be established (may take 2–5 min)…")
	}
}

// startServer starts the bootstrap server.
func startServer(srv *bootstrap.Server, ctx context.Context) error {
	if err := srv.Start(ctx); err != nil {
		fmt.Fprintf(os.Stderr, "Failed to start bootstrap server: %v\n", err)
		return err
	}
	return nil
}

// printServerInfo prints the server addresses and public key.
func printServerInfo(srv *bootstrap.Server) {
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
}

// waitForShutdown blocks until SIGINT or SIGTERM is received.
func waitForShutdown() {
	sigCh := make(chan os.Signal, 1)
	signal.Notify(sigCh, syscall.SIGINT, syscall.SIGTERM)
	<-sigCh
}

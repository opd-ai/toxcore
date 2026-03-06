package bootstrap

import (
	"context"
	"encoding/hex"
	"fmt"
	"net"
	"strconv"
	"sync"
	"time"

	toxcore "github.com/opd-ai/toxcore"
	"github.com/opd-ai/toxcore/crypto"
	"github.com/opd-ai/toxcore/dht"
	"github.com/opd-ai/toxcore/transport"
	"github.com/sirupsen/logrus"
)

// Server runs a Tox DHT bootstrap node that can simultaneously accept connections
// over clearnet (UDP), Tor onion services, and I2P. All three endpoints share the
// same cryptographic identity so clients can verify they are talking to the same node.
//
// A Server is single-use: once Stop() has been called, Start() will return an error.
// Create a new Server via New() to restart.
//
// Usage:
//
//	srv, err := bootstrap.New(bootstrap.DefaultConfig())
//	if err != nil { log.Fatal(err) }
//
//	if err := srv.Start(context.Background()); err != nil { log.Fatal(err) }
//	defer srv.Stop()
//
//	log.Println("Clearnet:", srv.GetClearnetAddr())
//	log.Println("Onion:   ", srv.GetOnionAddr())
//	log.Println("I2P:     ", srv.GetI2PAddr())
//	log.Println("Pubkey:  ", srv.GetPublicKeyHex())
type Server struct {
	config *Config
	logger *logrus.Logger

	// Shared cryptographic identity across all network endpoints.
	keyPair *crypto.KeyPair

	// Clearnet
	clearnetTox  *toxcore.Tox
	clearnetAddr string // "0.0.0.0:PORT" once started

	// Onion (Tor)
	onionNetTransport transport.NetworkTransport // TorTransport – closed in cleanup
	onionTransport    transport.Transport
	onionManager      *dht.BootstrapManager
	onionAddr         string // ".onion:port" once listener is established
	onionRoutingTbl   *dht.RoutingTable

	// I2P
	i2pNetTransport transport.NetworkTransport // I2PTransport – closed in cleanup
	i2pTransport    transport.Transport
	i2pManager      *dht.BootstrapManager
	i2pAddr         string // "*.b32.i2p:port" once listener is established
	i2pRoutingTbl   *dht.RoutingTable

	mu       sync.RWMutex
	running  bool
	stopped  bool         // true once Stop() has been called; Start() returns an error
	stopChan chan struct{} // re-created on each Start()
	wg       sync.WaitGroup

	ctx    context.Context
	cancel context.CancelFunc
}

// New creates a new bootstrap Server from the given Config.
// Call Start to begin serving connections.
func New(config *Config) (*Server, error) {
	if config == nil {
		config = DefaultConfig()
	}

	logger := config.Logger
	if logger == nil {
		logger = logrus.StandardLogger()
	}

	// Generate or restore the shared key pair.
	var keyPair *crypto.KeyPair
	var err error
	if len(config.SecretKey) == 32 {
		var sk [32]byte
		copy(sk[:], config.SecretKey)
		keyPair, err = crypto.FromSecretKey(sk)
	} else {
		keyPair, err = crypto.GenerateKeyPair()
	}
	if err != nil {
		return nil, fmt.Errorf("bootstrap: failed to initialise key pair: %w", err)
	}

	return &Server{
		config:  config,
		logger:  logger,
		keyPair: keyPair,
	}, nil
}

// Start initialises and starts all configured network endpoints.
// It blocks until all endpoints are ready or until config.StartupTimeout elapses.
// The provided context (treated as context.Background() when nil) can be used to
// cancel a long startup (e.g. Tor tunnel establishment), but the server continues
// running until Stop is called.
//
// Start may be called at most once. After Stop() returns, Start() will return an
// error; create a new Server with New() to restart.
func (s *Server) Start(ctx context.Context) error {
	if ctx == nil {
		ctx = context.Background()
	}

	s.mu.Lock()
	defer s.mu.Unlock()

	if s.stopped {
		return fmt.Errorf("bootstrap server has been stopped and cannot be restarted; create a new Server with New()")
	}
	if s.running {
		return fmt.Errorf("bootstrap server already running")
	}

	// (Re-)initialise per-run state so each Start() gets a fresh stop channel.
	s.stopChan = make(chan struct{})
	s.ctx, s.cancel = context.WithCancel(context.Background())

	if s.config.ClearnetEnabled {
		if err := s.startClearnet(); err != nil {
			s.stopRunningGoroutines()
			s.cleanup()
			return fmt.Errorf("bootstrap: clearnet startup failed: %w", err)
		}
	}

	if s.config.OnionEnabled {
		if err := s.startOnion(ctx); err != nil {
			s.stopRunningGoroutines()
			s.cleanup()
			return fmt.Errorf("bootstrap: onion startup failed: %w", err)
		}
	}

	if s.config.I2PEnabled {
		if err := s.startI2P(ctx); err != nil {
			s.stopRunningGoroutines()
			s.cleanup()
			return fmt.Errorf("bootstrap: I2P startup failed: %w", err)
		}
	}

	s.running = true

	s.logger.WithFields(logrus.Fields{
		"clearnet": s.clearnetAddr,
		"onion":    s.onionAddr,
		"i2p":      s.i2pAddr,
		"pubkey":   hex.EncodeToString(s.keyPair.Public[:]),
	}).Info("Bootstrap server started")

	return nil
}

// stopRunningGoroutines signals all background loops launched so far to stop
// and waits for them to finish. Used to clean up after a partial startup failure.
// Must be called with s.mu held. Goroutines spawned by this server do not
// acquire s.mu, so it is safe to wait while the lock is held.
func (s *Server) stopRunningGoroutines() {
	close(s.stopChan)
	s.cancel()
	s.wg.Wait()
}

// Stop gracefully shuts down all network endpoints and waits for background
// goroutines to finish. After Stop returns, the Server cannot be restarted.
func (s *Server) Stop() error {
	s.mu.Lock()
	defer s.mu.Unlock()

	if !s.running {
		return nil
	}

	s.running = false
	s.stopped = true

	// Signal all background loops to stop.
	close(s.stopChan)
	s.cancel()
	// Goroutines spawned by this server do not acquire s.mu, so it is safe
	// to wait while holding the lock.
	s.wg.Wait()

	s.cleanup()

	s.logger.Info("Bootstrap server stopped")
	return nil
}

// GetPublicKey returns the 32-byte Ed25519 public key that identifies this
// bootstrap node across all network endpoints.
func (s *Server) GetPublicKey() [32]byte {
	return s.keyPair.Public
}

// GetPublicKeyHex returns the public key as a lowercase hex string, suitable
// for publishing in bootstrap node lists.
func (s *Server) GetPublicKeyHex() string {
	return hex.EncodeToString(s.keyPair.Public[:])
}

// GetPrivateKey returns the 32-byte secret key of this bootstrap node.
// Store it securely and pass it back via Config.SecretKey to keep the same
// identity across server restarts.
func (s *Server) GetPrivateKey() []byte {
	key := make([]byte, 32)
	copy(key, s.keyPair.Private[:])
	return key
}

// GetClearnetAddr returns the "host:port" address of the UDP clearnet endpoint,
// or an empty string if clearnet is disabled or not yet started.
// The host is always "0.0.0.0" because toxcore binds the UDP socket on all
// interfaces. To determine your public IP, use an external lookup.
func (s *Server) GetClearnetAddr() string {
	s.mu.RLock()
	defer s.mu.RUnlock()
	return s.clearnetAddr
}

// GetOnionAddr returns the ".onion:port" address of the Tor hidden service
// endpoint, or an empty string if onion mode is disabled or not yet started.
func (s *Server) GetOnionAddr() string {
	s.mu.RLock()
	defer s.mu.RUnlock()
	return s.onionAddr
}

// GetI2PAddr returns the I2P destination address (*.b32.i2p) of the I2P
// endpoint, or an empty string if I2P mode is disabled or not yet started.
func (s *Server) GetI2PAddr() string {
	s.mu.RLock()
	defer s.mu.RUnlock()
	return s.i2pAddr
}

// IsRunning reports whether the server is currently active.
func (s *Server) IsRunning() bool {
	s.mu.RLock()
	defer s.mu.RUnlock()
	return s.running
}

// ─── Clearnet ────────────────────────────────────────────────────────────────

func (s *Server) startClearnet() error {
	port := s.config.ClearnetPort

	s.logger.WithField("port", port).Info("Starting clearnet bootstrap server")

	// Build a Tox instance with our shared key injected as savedata so the
	// public key is deterministic across restarts.
	opts := toxcore.NewOptions()
	opts.UDPEnabled = true
	opts.IPv6Enabled = false
	opts.LocalDiscovery = false
	opts.StartPort = port
	opts.EndPort = port
	opts.MinBootstrapNodes = 1
	opts.SavedataType = toxcore.SaveDataTypeSecretKey
	opts.SavedataData = s.keyPair.Private[:]

	tox, err := toxcore.New(opts)
	if err != nil {
		return fmt.Errorf("failed to create Tox instance: %w", err)
	}

	s.clearnetTox = tox

	// Verify key consistency: toxcore must have reproduced our public key.
	if tox.GetSelfPublicKey() != s.keyPair.Public {
		tox.Kill()
		s.clearnetTox = nil
		return fmt.Errorf("key mismatch: toxcore public key does not match generated key pair")
	}

	// toxcore always binds UDP to 0.0.0.0; record the address for GetClearnetAddr().
	s.clearnetAddr = net.JoinHostPort("0.0.0.0", strconv.Itoa(int(port)))

	// Start the Tox event loop in a background goroutine.
	s.wg.Add(1)
	go s.clearnetLoop()

	return nil
}

func (s *Server) clearnetLoop() {
	defer s.wg.Done()

	interval := s.config.IterationInterval
	ticker := time.NewTicker(interval)
	defer ticker.Stop()

	for {
		select {
		case <-s.stopChan:
			return
		case <-s.ctx.Done():
			return
		case <-ticker.C:
			s.clearnetTox.Iterate()
		}
	}
}

// ─── Onion (Tor) ─────────────────────────────────────────────────────────────

func (s *Server) startOnion(ctx context.Context) error {
	s.logger.Info("Starting onion bootstrap server")

	torTransport := transport.NewTorTransport()

	// Derive the listen port from configuration.
	listenPort := s.config.ClearnetPort
	if listenPort == 0 {
		listenPort = DefaultClearnetPort
	}
	listenAddr := fmt.Sprintf("toxcore-bootstrap.onion:%d", listenPort)

	// Listen() creates the hidden service; it may block while Tor publishes the
	// descriptor (typically 30–90 s). The caller passes a startup context.
	startCtx, cancel := context.WithTimeout(ctx, s.config.StartupTimeout)
	defer cancel()

	listenerCh := make(chan net.Listener, 1)
	errCh := make(chan error, 1)

	go func() {
		ln, err := torTransport.Listen(listenAddr)
		if err != nil {
			errCh <- err
			return
		}
		listenerCh <- ln
	}()

	var listener net.Listener
	select {
	case <-startCtx.Done():
		torTransport.Close() //nolint:errcheck
		return fmt.Errorf("onion service startup timed out: %w", startCtx.Err())
	case err := <-errCh:
		torTransport.Close() //nolint:errcheck
		return fmt.Errorf("onion service listen failed: %w", err)
	case listener = <-listenerCh:
	}

	s.onionAddr = listener.Addr().String()
	s.onionNetTransport = torTransport
	s.logger.WithField("onion_addr", s.onionAddr).Info("Onion listener ready")

	tcpT, manager, routingTbl, err := s.buildOverlayServer(listener)
	if err != nil {
		if closeErr := torTransport.Close(); closeErr != nil {
			s.logger.WithError(closeErr).Warn("Failed to close Tor transport after overlay server error")
		}
		return err
	}
	s.onionTransport = tcpT
	s.onionManager = manager
	s.onionRoutingTbl = routingTbl

	return nil
}

// ─── I2P ─────────────────────────────────────────────────────────────────────

func (s *Server) startI2P(ctx context.Context) error {
	s.logger.WithField("sam_addr", s.config.I2PSAMAddr).Info("Starting I2P bootstrap server")

	i2pTransport := transport.NewI2PTransportWithSAMAddr(s.config.I2PSAMAddr)

	// Derive the listen port from configuration.
	listenPort := s.config.ClearnetPort
	if listenPort == 0 {
		listenPort = DefaultClearnetPort
	}
	listenAddr := fmt.Sprintf("toxcore-bootstrap.b32.i2p:%d", listenPort)

	startCtx, cancel := context.WithTimeout(ctx, s.config.StartupTimeout)
	defer cancel()

	listenerCh := make(chan net.Listener, 1)
	errCh := make(chan error, 1)

	go func() {
		ln, err := i2pTransport.Listen(listenAddr)
		if err != nil {
			errCh <- err
			return
		}
		listenerCh <- ln
	}()

	var listener net.Listener
	select {
	case <-startCtx.Done():
		i2pTransport.Close() //nolint:errcheck
		return fmt.Errorf("I2P listener startup timed out: %w", startCtx.Err())
	case err := <-errCh:
		i2pTransport.Close() //nolint:errcheck
		return fmt.Errorf("I2P listener creation failed: %w", err)
	case listener = <-listenerCh:
	}

	s.i2pAddr = listener.Addr().String()
	s.i2pNetTransport = i2pTransport
	s.logger.WithField("i2p_addr", s.i2pAddr).Info("I2P listener ready")

	tcpT, manager, routingTbl, err := s.buildOverlayServer(listener)
	if err != nil {
		if closeErr := i2pTransport.Close(); closeErr != nil {
			s.logger.WithError(closeErr).Warn("Failed to close I2P transport after overlay server error")
		}
		return err
	}
	s.i2pTransport = tcpT
	s.i2pManager = manager
	s.i2pRoutingTbl = routingTbl

	return nil
}

// ─── Shared overlay server logic ─────────────────────────────────────────────

// buildOverlayServer wires a net.Listener from an overlay network (Tor or I2P)
// into a TCPTransport-backed DHT bootstrap manager and starts its keepalive loop.
// It returns the created transport, manager, and routing table, or an error.
func (s *Server) buildOverlayServer(listener net.Listener) (
	transport.Transport, *dht.BootstrapManager, *dht.RoutingTable, error,
) {
	tcpT, err := transport.NewTCPTransportFromListener(listener)
	if err != nil {
		listener.Close() //nolint:errcheck
		return nil, nil, nil, fmt.Errorf("failed to create TCP transport from listener: %w", err)
	}

	var nospam [4]byte
	toxID := crypto.NewToxID(s.keyPair.Public, nospam)
	routingTbl := dht.NewRoutingTable(*toxID, 8)
	manager := dht.NewBootstrapManagerWithKeyPair(*toxID, s.keyPair, tcpT, routingTbl)

	// Register DHT packet handlers so the manager responds to bootstrap requests.
	for _, pt := range dhtPacketTypes() {
		tcpT.RegisterHandler(pt, func(pkt *transport.Packet, addr net.Addr) error {
			return manager.HandlePacket(pkt, addr)
		})
	}

	// The TCP transport handles incoming connections internally; we only need
	// a lightweight keepalive loop here.
	s.wg.Add(1)
	go s.overlayLoop(tcpT)

	return tcpT, manager, routingTbl, nil
}

// dhtPacketTypes returns the packet types that a bootstrap server must handle.
func dhtPacketTypes() []transport.PacketType {
	return []transport.PacketType{
		transport.PacketPingRequest,
		transport.PacketPingResponse,
		transport.PacketGetNodes,
		transport.PacketSendNodes,
		transport.PacketVersionNegotiation,
		transport.PacketNoiseHandshake,
	}
}

// overlayLoop keeps the overlay transport alive until the server stops.
func (s *Server) overlayLoop(t transport.Transport) {
	defer s.wg.Done()
	defer t.Close() //nolint:errcheck

	<-s.stopChan
}

// ─── Cleanup ─────────────────────────────────────────────────────────────────

// cleanup releases all resources. Must be called with s.mu held.
func (s *Server) cleanup() {
	if s.clearnetTox != nil {
		s.clearnetTox.Kill()
		s.clearnetTox = nil
	}
	s.clearnetAddr = ""

	// Close the TCP transport (closes the listener) then the underlying network
	// transport (closes onramp resources/goroutines).
	if s.onionTransport != nil {
		s.onionTransport.Close() //nolint:errcheck
		s.onionTransport = nil
	}
	if s.onionNetTransport != nil {
		s.onionNetTransport.Close() //nolint:errcheck
		s.onionNetTransport = nil
	}

	if s.i2pTransport != nil {
		s.i2pTransport.Close() //nolint:errcheck
		s.i2pTransport = nil
	}
	if s.i2pNetTransport != nil {
		s.i2pNetTransport.Close() //nolint:errcheck
		s.i2pNetTransport = nil
	}
}

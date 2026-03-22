// Package toxcore implements the Tox protocol with concurrent event processing.
//
// This file implements the concurrent iteration pipeline system that decouples
// DHT maintenance, friend connections, and message processing into separate
// goroutines for improved throughput.
package toxcore

import (
	"context"
	"sync"
	"sync/atomic"
	"time"

	"github.com/sirupsen/logrus"
)

// PipelineType identifies different processing pipelines.
type PipelineType uint8

const (
	// PipelineDHT handles DHT maintenance tasks.
	PipelineDHT PipelineType = iota
	// PipelineFriends handles friend connection management.
	PipelineFriends
	// PipelineMessages handles message processing.
	PipelineMessages
)

// PipelineConfig holds configuration for iteration pipelines.
type PipelineConfig struct {
	// DHTInterval is the interval between DHT maintenance runs (default: 6s).
	DHTInterval time.Duration
	// FriendInterval is the interval between friend connection checks (default: 12s).
	FriendInterval time.Duration
	// MessageInterval is the interval between message processing runs (default: 50ms).
	MessageInterval time.Duration
	// EnableConcurrent enables concurrent pipeline execution (default: false for backward compat).
	EnableConcurrent bool
}

// DefaultPipelineConfig returns the default pipeline configuration.
func DefaultPipelineConfig() PipelineConfig {
	return PipelineConfig{
		DHTInterval:      6 * time.Second,
		FriendInterval:   12 * time.Second,
		MessageInterval:  50 * time.Millisecond,
		EnableConcurrent: false,
	}
}

// IterationPipelines manages concurrent processing pipelines for the Tox instance.
// It decouples DHT maintenance, friend connections, and messaging into separate
// goroutines, coordinated via channels.
//
//export ToxIterationPipelines
type IterationPipelines struct {
	tox    *Tox
	config PipelineConfig

	// Channels for pipeline coordination
	dhtTrigger     chan struct{}
	friendsTrigger chan struct{}
	msgTrigger     chan struct{}

	// Pipeline state
	running atomic.Bool
	wg      sync.WaitGroup

	// Context for clean shutdown
	ctx    context.Context
	cancel context.CancelFunc

	// Stats
	stats PipelineStats
}

// PipelineStats tracks pipeline execution statistics.
type PipelineStats struct {
	DHTRuns         atomic.Uint64
	FriendRuns      atomic.Uint64
	MessageRuns     atomic.Uint64
	DHTDuration     atomic.Int64 // nanoseconds
	FriendDuration  atomic.Int64
	MessageDuration atomic.Int64
}

// NewIterationPipelines creates a new concurrent iteration pipeline manager.
//
//export ToxNewIterationPipelines
func NewIterationPipelines(tox *Tox, config *PipelineConfig) *IterationPipelines {
	if config == nil {
		defaultConfig := DefaultPipelineConfig()
		config = &defaultConfig
	}

	ctx, cancel := context.WithCancel(tox.ctx)

	p := &IterationPipelines{
		tox:            tox,
		config:         *config,
		dhtTrigger:     make(chan struct{}, 1),
		friendsTrigger: make(chan struct{}, 1),
		msgTrigger:     make(chan struct{}, 1),
		ctx:            ctx,
		cancel:         cancel,
	}

	logrus.WithFields(logrus.Fields{
		"function":         "NewIterationPipelines",
		"dht_interval":     config.DHTInterval,
		"friend_interval":  config.FriendInterval,
		"message_interval": config.MessageInterval,
		"concurrent":       config.EnableConcurrent,
	}).Info("Created iteration pipelines")

	return p
}

// Start begins the concurrent pipeline processing.
// Call this instead of the Iterate() loop when concurrent processing is enabled.
//
//export ToxPipelinesStart
func (p *IterationPipelines) Start() {
	if p.running.Swap(true) {
		return // Already running
	}

	logrus.WithField("function", "Start").Info("Starting iteration pipelines")

	if p.config.EnableConcurrent {
		// Start separate goroutines for each pipeline
		p.wg.Add(3)
		go p.runDHTPipeline()
		go p.runFriendsPipeline()
		go p.runMessagesPipeline()
	} else {
		// Single-threaded mode - uses the traditional Iterate() pattern
		p.wg.Add(1)
		go p.runSequentialPipeline()
	}
}

// Stop stops all pipeline processing and waits for completion.
//
//export ToxPipelinesStop
func (p *IterationPipelines) Stop() {
	if !p.running.Swap(false) {
		return // Not running
	}

	p.cancel()
	p.wg.Wait()

	logrus.WithField("function", "Stop").Info("Iteration pipelines stopped")
}

// TriggerDHT triggers an immediate DHT maintenance run.
//
//export ToxPipelinesTriggerDHT
func (p *IterationPipelines) TriggerDHT() {
	select {
	case p.dhtTrigger <- struct{}{}:
	default:
		// Already pending
	}
}

// TriggerFriends triggers an immediate friend connections check.
//
//export ToxPipelinesTriggerFriends
func (p *IterationPipelines) TriggerFriends() {
	select {
	case p.friendsTrigger <- struct{}{}:
	default:
		// Already pending
	}
}

// TriggerMessages triggers an immediate message processing run.
//
//export ToxPipelinesTriggerMessages
func (p *IterationPipelines) TriggerMessages() {
	select {
	case p.msgTrigger <- struct{}{}:
	default:
		// Already pending
	}
}

// Stats returns the current pipeline statistics.
func (p *IterationPipelines) Stats() (dhtRuns, friendRuns, msgRuns uint64, dhtDur, friendDur, msgDur time.Duration) {
	return p.stats.DHTRuns.Load(),
		p.stats.FriendRuns.Load(),
		p.stats.MessageRuns.Load(),
		time.Duration(p.stats.DHTDuration.Load()),
		time.Duration(p.stats.FriendDuration.Load()),
		time.Duration(p.stats.MessageDuration.Load())
}

// runDHTPipeline runs the DHT maintenance pipeline.
func (p *IterationPipelines) runDHTPipeline() {
	defer p.wg.Done()

	ticker := time.NewTicker(p.config.DHTInterval)
	defer ticker.Stop()

	logrus.WithField("function", "runDHTPipeline").Debug("DHT pipeline started")

	for {
		select {
		case <-p.ctx.Done():
			return
		case <-ticker.C:
			p.executeDHT()
		case <-p.dhtTrigger:
			p.executeDHT()
		}
	}
}

// runFriendsPipeline runs the friend connections pipeline.
func (p *IterationPipelines) runFriendsPipeline() {
	defer p.wg.Done()

	ticker := time.NewTicker(p.config.FriendInterval)
	defer ticker.Stop()

	logrus.WithField("function", "runFriendsPipeline").Debug("Friends pipeline started")

	for {
		select {
		case <-p.ctx.Done():
			return
		case <-ticker.C:
			p.executeFriends()
		case <-p.friendsTrigger:
			p.executeFriends()
		}
	}
}

// runMessagesPipeline runs the message processing pipeline.
func (p *IterationPipelines) runMessagesPipeline() {
	defer p.wg.Done()

	ticker := time.NewTicker(p.config.MessageInterval)
	defer ticker.Stop()

	logrus.WithField("function", "runMessagesPipeline").Debug("Messages pipeline started")

	for {
		select {
		case <-p.ctx.Done():
			return
		case <-ticker.C:
			p.executeMessages()
		case <-p.msgTrigger:
			p.executeMessages()
		}
	}
}

// runSequentialPipeline runs all pipelines sequentially (backward compatible mode).
func (p *IterationPipelines) runSequentialPipeline() {
	defer p.wg.Done()

	ticker := time.NewTicker(p.config.MessageInterval)
	defer ticker.Stop()

	dhtCounter := uint64(0)
	friendCounter := uint64(0)
	dhtMod := uint64(p.config.DHTInterval / p.config.MessageInterval)
	friendMod := uint64(p.config.FriendInterval / p.config.MessageInterval)

	logrus.WithField("function", "runSequentialPipeline").Debug("Sequential pipeline started")

	for {
		select {
		case <-p.ctx.Done():
			return
		case <-ticker.C:
			dhtCounter++
			friendCounter++

			// DHT at configured interval
			if dhtCounter >= dhtMod {
				p.executeDHT()
				dhtCounter = 0
			}

			// Friends at configured interval
			if friendCounter >= friendMod {
				p.executeFriends()
				friendCounter = 0
			}

			// Messages every tick
			p.executeMessages()

		case <-p.dhtTrigger:
			p.executeDHT()
		case <-p.friendsTrigger:
			p.executeFriends()
		case <-p.msgTrigger:
			p.executeMessages()
		}
	}
}

// executeDHT runs DHT maintenance with timing.
func (p *IterationPipelines) executeDHT() {
	start := time.Now()
	p.tox.doDHTMaintenance()
	duration := time.Since(start)

	p.stats.DHTRuns.Add(1)
	p.stats.DHTDuration.Store(int64(duration))

	logrus.WithFields(logrus.Fields{
		"function": "executeDHT",
		"duration": duration,
	}).Debug("DHT maintenance completed")
}

// executeFriends runs friend connection management with timing.
func (p *IterationPipelines) executeFriends() {
	start := time.Now()
	p.tox.doFriendConnections()
	p.tox.retryPendingFriendRequests()
	duration := time.Since(start)

	p.stats.FriendRuns.Add(1)
	p.stats.FriendDuration.Store(int64(duration))

	logrus.WithFields(logrus.Fields{
		"function": "executeFriends",
		"duration": duration,
	}).Debug("Friend connections completed")
}

// executeMessages runs message processing with timing.
func (p *IterationPipelines) executeMessages() {
	start := time.Now()
	p.tox.doMessageProcessing()
	duration := time.Since(start)

	p.stats.MessageRuns.Add(1)
	p.stats.MessageDuration.Store(int64(duration))
}

// IsRunning returns true if the pipelines are currently running.
func (p *IterationPipelines) IsRunning() bool {
	return p.running.Load()
}

// IsConcurrent returns true if concurrent mode is enabled.
func (p *IterationPipelines) IsConcurrent() bool {
	return p.config.EnableConcurrent
}

// --- Tox methods for pipeline integration ---

// EnableConcurrentIteration enables concurrent iteration pipelines.
// This replaces the traditional Iterate() loop with parallel goroutines
// for DHT, friends, and messaging.
//
//export ToxEnableConcurrentIteration
func (t *Tox) EnableConcurrentIteration(config *PipelineConfig) *IterationPipelines {
	pipelines := NewIterationPipelines(t, config)
	return pipelines
}

// RunWithPipelines starts the Tox instance with concurrent pipelines.
// This is an alternative to the manual Iterate() loop.
//
//export ToxRunWithPipelines
func (t *Tox) RunWithPipelines(config *PipelineConfig) *IterationPipelines {
	pipelines := NewIterationPipelines(t, config)
	pipelines.Start()
	return pipelines
}

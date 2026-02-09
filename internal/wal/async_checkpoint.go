package wal

import (
	"errors"
	"sync"
	"time"
)

// CheckpointFunc is the function called to perform the actual checkpoint
type CheckpointFunc func() error

// AsyncCheckpointer handles periodic and requested checkpoints
type AsyncCheckpointer struct {
	checkpointFunc CheckpointFunc
	interval       time.Duration
	stopCh         chan struct{}
	reqCh          chan chan error
	wg             sync.WaitGroup
	running        bool
	mu             sync.Mutex
}

// NewAsyncCheckpointer creates a new async checkpointer
func NewAsyncCheckpointer(interval time.Duration, fn CheckpointFunc) *AsyncCheckpointer {
	return &AsyncCheckpointer{
		checkpointFunc: fn,
		interval:       interval,
		stopCh:         make(chan struct{}),
		reqCh:          make(chan chan error),
	}
}

// Start starts the background checkpointer goroutine
func (ac *AsyncCheckpointer) Start() {
	ac.mu.Lock()
	defer ac.mu.Unlock()

	if ac.running {
		return
	}

	ac.running = true
	ac.wg.Add(1)
	go ac.run()
}

// Stop stops the background checkpointer and waits for it to exit
func (ac *AsyncCheckpointer) Stop() {
	ac.mu.Lock()
	if !ac.running {
		ac.mu.Unlock()
		return
	}
	ac.running = false
	ac.mu.Unlock()

	close(ac.stopCh)
	ac.wg.Wait()
}

// RequestCheckpoint triggers a checkpoint and returns a channel for the result
func (ac *AsyncCheckpointer) RequestCheckpoint() <-chan error {
	errCh := make(chan error, 1)

	// If not running, return error immediately
	ac.mu.Lock()
	if !ac.running {
		ac.mu.Unlock()
		errCh <- errors.New("checkpointer not running")
		return errCh
	}
	ac.mu.Unlock()

	select {
	case ac.reqCh <- errCh:
		// Request sent
	case <-ac.stopCh:
		errCh <- errors.New("checkpointer stopped")
	}
	return errCh
}

func (ac *AsyncCheckpointer) run() {
	defer ac.wg.Done()

	var ticker *time.Ticker
	var tickerCh <-chan time.Time

	if ac.interval > 0 {
		ticker = time.NewTicker(ac.interval)
		tickerCh = ticker.C
		defer ticker.Stop()
	}

	for {
		select {
		case <-ac.stopCh:
			return

		case <-tickerCh:
			// Auto-checkpoint
			// We ignore errors for auto-checkpoints but they should be logged by the caller/func if needed
			_ = ac.checkpointFunc()

		case errCh := <-ac.reqCh:
			// Requested checkpoint
			err := ac.checkpointFunc()
			errCh <- err
			// Reset ticker to avoid immediate auto-checkpoint after manual one
			if ticker != nil {
				ticker.Reset(ac.interval)
			}
		}
	}
}

package wal

import (
	"errors"
	"sync/atomic"
	"testing"
	"time"
)

func TestAsyncCheckpointer_Lifecycle(t *testing.T) {
	var called int32
	fn := func() error {
		atomic.AddInt32(&called, 1)
		return nil
	}

	ac := NewAsyncCheckpointer(50*time.Millisecond, fn)
	ac.Start()

	// Wait for at least one auto-checkpoint
	time.Sleep(150 * time.Millisecond)

	ac.Stop()

	if atomic.LoadInt32(&called) == 0 {
		t.Error("Expected auto-checkpoint to run")
	}
}

func TestAsyncCheckpointer_Request(t *testing.T) {
	var called int32
	fn := func() error {
		atomic.AddInt32(&called, 1)
		return nil
	}

	ac := NewAsyncCheckpointer(0, fn) // No auto-checkpoint
	ac.Start()

	errCh := ac.RequestCheckpoint()
	err := <-errCh
	if err != nil {
		t.Errorf("Unexpected error: %v", err)
	}

	ac.Stop()

	if atomic.LoadInt32(&called) != 1 {
		t.Errorf("Expected 1 call, got %d", atomic.LoadInt32(&called))
	}
}

func TestAsyncCheckpointer_ErrorPropagated(t *testing.T) {
	expectedErr := errors.New("checkpoint failed")
	fn := func() error {
		return expectedErr
	}

	ac := NewAsyncCheckpointer(0, fn)
	ac.Start()

	errCh := ac.RequestCheckpoint()
	err := <-errCh
	if err == nil || err.Error() != expectedErr.Error() {
		t.Errorf("Expected error %v, got %v", expectedErr, err)
	}

	ac.Stop()
}

func TestAsyncCheckpointer_StopStopsTicker(t *testing.T) {
	var called int32
	fn := func() error {
		atomic.AddInt32(&called, 1)
		return nil
	}

	ac := NewAsyncCheckpointer(10*time.Millisecond, fn)
	ac.Start()
	time.Sleep(50 * time.Millisecond) // Let it run a bit
	ac.Stop()

	countAfterStop := atomic.LoadInt32(&called)
	time.Sleep(50 * time.Millisecond) // Should not run anymore

	if atomic.LoadInt32(&called) > countAfterStop {
		t.Error("Checkpointer continued running after Stop()")
	}
}

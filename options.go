package joydb

import "time"

type config struct {
	walEnabled         bool
	walSyncInterval    time.Duration
	checkpointInterval time.Duration
	debug              bool
}

func defaultConfig() config {
	return config{walEnabled: true, checkpointInterval: 5 * time.Second}
}

// Option configures a Store.
type Option func(*config)

func WithoutWAL() Option {
	return func(cfg *config) { cfg.walEnabled = false }
}

func WithWALSyncInterval(interval time.Duration) Option {
	return func(cfg *config) { cfg.walSyncInterval = interval }
}

func WithCheckpointInterval(interval time.Duration) Option {
	return func(cfg *config) { cfg.checkpointInterval = interval }
}

func WithDebugLogging() Option {
	return func(cfg *config) { cfg.debug = true }
}

package retry

import (
	"time"
)

// Config defines the config for addon.
type Config struct {
	// InitialInterval defines the initial time interval for backoff algorithm.
	//
	// Optional. Default: 1 * time.Second
	InitialInterval time.Duration

	// MaxBackoffTime defines maximum time duration for backoff algorithm. When
	// the algorithm is reached this time, rest of the retries will be maximum
	// 32 seconds.
	//
	// Optional. Default: 32 * time.Second
	MaxBackoffTime time.Duration

	// Multiplier defines multiplier number of the backoff algorithm.
	//
	// Optional. Default: 2.0
	Multiplier float64

	// MaxRetryCount defines maximum retry count for the backoff algorithm.
	//
	// Optional. Default: 10
	MaxRetryCount int

	// currentInterval tracks the current waiting time.
	//
	// Optional. Default: 1 * time.Second
	currentInterval time.Duration
}

// DefaultConfig is the default config for retry.
var DefaultConfig = Config{
	InitialInterval: 1 * time.Second,
	MaxBackoffTime:  32 * time.Second,
	Multiplier:      2.0,
	MaxRetryCount:   10,
	currentInterval: 1 * time.Second,
}

// configDefault sets the config values if they are not set.
func configDefault(config ...Config) Config {
	if len(config) == 0 {
		return DefaultConfig
	}
	cfg := config[0]
	if cfg.InitialInterval == 0 {
		cfg.InitialInterval = DefaultConfig.InitialInterval
	}
	if cfg.MaxBackoffTime == 0 {
		cfg.MaxBackoffTime = DefaultConfig.MaxBackoffTime
	}
	if cfg.Multiplier <= 0 {
		cfg.Multiplier = DefaultConfig.Multiplier
	}
	if cfg.MaxRetryCount <= 0 {
		cfg.MaxRetryCount = DefaultConfig.MaxRetryCount
	}
	if cfg.currentInterval != cfg.InitialInterval {
		cfg.currentInterval = DefaultConfig.currentInterval
	}
	return cfg
}

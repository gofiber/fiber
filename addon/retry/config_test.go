package retry

import (
	"testing"
	"time"

	"github.com/stretchr/testify/require"
)

func TestConfigDefault(t *testing.T) {
	t.Parallel()
	custom := Config{
		InitialInterval: 2 * time.Second,
		MaxBackoffTime:  64 * time.Second,
		Multiplier:      3.0,
		MaxRetryCount:   5,
		currentInterval: 2 * time.Second,
	}
	// defaultsWith is DefaultConfig with the two intervals replaced.
	defaultsWith := func(initial, current time.Duration) Config {
		cfg := DefaultConfig
		cfg.InitialInterval, cfg.currentInterval = initial, current
		return cfg
	}

	for _, tc := range []struct {
		name string
		cfg  []Config
		want Config
	}{
		{"no config", nil, DefaultConfig},
		{"custom", []Config{custom}, custom},
		{"partial and negative", []Config{{Multiplier: -1, MaxRetryCount: 0}}, DefaultConfig},
		{"custom initial interval", []Config{{InitialInterval: 5 * time.Second}}, defaultsWith(5*time.Second, 5*time.Second)},
		{"custom current interval", []Config{{currentInterval: 3 * time.Second}}, defaultsWith(DefaultConfig.InitialInterval, 3*time.Second)},
		{
			"current interval and initial differ",
			[]Config{{InitialInterval: 5 * time.Second, currentInterval: 3 * time.Second}},
			defaultsWith(5*time.Second, 3*time.Second),
		},
	} {
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()
			require.Equal(t, tc.want, configDefault(tc.cfg...))
		})
	}
}

func TestNewExponentialBackoff_Config(t *testing.T) {
	t.Parallel()
	backoff := NewExponentialBackoff(Config{InitialInterval: 4 * time.Second})
	require.Equal(t, 4*time.Second, backoff.InitialInterval)
	require.Equal(t, 4*time.Second, backoff.currentInterval)
}

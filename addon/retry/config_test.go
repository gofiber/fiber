package retry

import (
	"testing"
	"time"

	"github.com/stretchr/testify/require"
)

func TestConfigDefault_NoConfig(t *testing.T) {
	t.Parallel()
	cfg := configDefault()
	require.Equal(t, DefaultConfig, cfg)
}

func TestConfigDefault_Custom(t *testing.T) {
	t.Parallel()
	custom := Config{
		InitialInterval: 2 * time.Second,
		MaxBackoffTime:  64 * time.Second,
		Multiplier:      3.0,
		MaxRetryCount:   5,
		currentInterval: 2 * time.Second,
	}
	cfg := configDefault(custom)
	require.Equal(t, custom, cfg)
}

func TestConfigDefault_PartialAndNegative(t *testing.T) {
	t.Parallel()
	cfg := configDefault(Config{Multiplier: -1, MaxRetryCount: 0})
	require.Equal(t, DefaultConfig, cfg)
}

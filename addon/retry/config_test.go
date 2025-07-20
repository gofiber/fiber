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

func TestConfigDefault_CustomInitialInterval(t *testing.T) {
	t.Parallel()

	cfg := configDefault(Config{InitialInterval: 5 * time.Second})

	require.Equal(t, 5*time.Second, cfg.currentInterval)
	require.Equal(t, 5*time.Second, cfg.InitialInterval)
}

func TestConfigDefault_CustomCurrentInterval(t *testing.T) {
	t.Parallel()

	cfg := configDefault(Config{currentInterval: 3 * time.Second})

	require.Equal(t, 3*time.Second, cfg.currentInterval)
	require.Equal(t, DefaultConfig.InitialInterval, cfg.InitialInterval)
}

func TestConfigDefault_CurrentIntervalAndInitialDiffer(t *testing.T) {
	t.Parallel()

	cfg := configDefault(Config{InitialInterval: 5 * time.Second, currentInterval: 3 * time.Second})

	require.Equal(t, 5*time.Second, cfg.InitialInterval)
	require.Equal(t, 3*time.Second, cfg.currentInterval)
}

func TestNewExponentialBackoff_Config(t *testing.T) {
	t.Parallel()

	backoff := NewExponentialBackoff(Config{InitialInterval: 4 * time.Second})

	require.Equal(t, 4*time.Second, backoff.InitialInterval)
	require.Equal(t, 4*time.Second, backoff.currentInterval)
}

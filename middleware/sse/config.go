package sse

import (
	"time"

	"github.com/gofiber/fiber/v3"
)

// Handler writes events to a single SSE stream.
type Handler func(c fiber.Ctx, stream *Stream) error

// Config defines the config for the SSE handler.
type Config struct {
	// Handler writes events to the stream.
	//
	// Required.
	Handler Handler

	// OnClose is called after the stream handler returns or the client disconnects.
	//
	// Optional. Default: nil
	OnClose func(c fiber.Ctx, err error)

	// Retry controls the reconnection delay sent to clients.
	// Values less than or equal to zero disable the initial retry field.
	//
	// Optional. Default: 0
	Retry time.Duration

	// HeartbeatInterval controls comment heartbeats used to keep intermediaries
	// from closing idle streams and to detect disconnected clients.
	// When DisableHeartbeat is false, values less than or equal to zero are
	// replaced by the default interval.
	//
	// Optional. Default: 15 * time.Second
	HeartbeatInterval time.Duration

	// DisableHeartbeat disables automatic comment heartbeats.
	//
	// Optional. Default: false
	DisableHeartbeat bool
}

// ConfigDefault is the default config.
var ConfigDefault = Config{
	Handler:           nil,
	OnClose:           nil,
	Retry:             0,
	HeartbeatInterval: 15 * time.Second,
	DisableHeartbeat:  false,
}

// Helper function to set default values.
func configDefault(config ...Config) Config {
	if len(config) < 1 {
		return ConfigDefault
	}

	cfg := config[0]
	if !cfg.DisableHeartbeat && cfg.HeartbeatInterval <= 0 {
		cfg.HeartbeatInterval = ConfigDefault.HeartbeatInterval
	}
	return cfg
}

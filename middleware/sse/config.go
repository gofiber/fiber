package sse

import (
	"time"

	"github.com/gofiber/fiber/v3"
)

// Config defines the configuration for the SSE middleware.
//
// The SSE middleware is terminal: it hijacks the response stream and
// never calls c.Next(). Placing handlers after sse.New() in the chain
// results in undefined behavior because Fiber releases the fiber.Ctx
// before the stream writer runs.
type Config struct {
	// OnConnect is called when a new client connects, before the SSE
	// stream begins. Use it for authentication, topic selection, and
	// connection limits. Set conn.Topics and conn.Metadata here.
	// Return a non-nil error to reject the connection (sends 403).
	//
	// Optional. Default: nil
	OnConnect func(c fiber.Ctx, conn *Connection) error

	// OnDisconnect is called after a client disconnects.
	//
	// Optional. Default: nil
	OnDisconnect func(conn *Connection)

	// OnPause is called when a connection is paused (browser tab hidden).
	//
	// Optional. Default: nil
	OnPause func(conn *Connection)

	// OnResume is called when a connection is resumed (browser tab visible).
	//
	// Optional. Default: nil
	OnResume func(conn *Connection)

	// Replayer enables Last-Event-ID replay. If nil, replay is disabled.
	//
	// Optional. Default: nil
	Replayer Replayer

	// Bridges declares external pub/sub sources (Redis, NATS, etc.) that
	// feed events into the hub. Bridges start automatically when the first
	// handler is mounted and stop on Shutdown.
	//
	// Optional. Default: nil
	Bridges []BridgeConfig

	// FlushInterval is how often batched (P1) and coalesced (P2) events
	// are flushed to clients. Instant (P0) events bypass this.
	//
	// Optional. Default: 2s
	FlushInterval time.Duration

	// HeartbeatInterval is how often a comment is sent to idle connections
	// to detect disconnects and prevent proxy timeouts.
	//
	// Optional. Default: 30s
	HeartbeatInterval time.Duration

	// MaxLifetime is the maximum duration a single SSE connection can
	// stay open. After this, the connection is closed gracefully.
	// Set to -1 for unlimited.
	//
	// Optional. Default: 30m
	MaxLifetime time.Duration

	// SendBufferSize is the per-connection channel buffer. If full,
	// events are dropped and the client should reconnect.
	//
	// Optional. Default: 256
	SendBufferSize int

	// RetryMS is the reconnection interval hint sent to clients via the
	// retry: directive on connect.
	//
	// Optional. Default: 3000
	RetryMS int
}

// ConfigDefault is the default config.
var ConfigDefault = Config{
	FlushInterval:     2 * time.Second,
	SendBufferSize:    256,
	HeartbeatInterval: 30 * time.Second,
	MaxLifetime:       30 * time.Minute,
	RetryMS:           3000,
}

func configDefault(config ...Config) Config {
	if len(config) < 1 {
		return ConfigDefault
	}

	cfg := config[0]

	if cfg.FlushInterval <= 0 {
		cfg.FlushInterval = ConfigDefault.FlushInterval
	}
	if cfg.SendBufferSize <= 0 {
		cfg.SendBufferSize = ConfigDefault.SendBufferSize
	}
	if cfg.HeartbeatInterval <= 0 {
		cfg.HeartbeatInterval = ConfigDefault.HeartbeatInterval
	}
	if cfg.MaxLifetime == 0 {
		cfg.MaxLifetime = ConfigDefault.MaxLifetime
	}
	if cfg.RetryMS <= 0 {
		cfg.RetryMS = ConfigDefault.RetryMS
	}

	return cfg
}

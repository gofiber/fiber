package sse

import (
	"context"
	"time"

	"github.com/gofiber/fiber/v3/log"
)

// bridgeRetryDelay is how long the hub waits before retrying a failed
// SubscriberBridge.Subscribe call.
const bridgeRetryDelay = 3 * time.Second

// SubscriberBridge adapts an external pub/sub system (Redis, NATS, Kafka,
// etc.) so incoming messages can be forwarded into the hub as SSE events.
//
// Implementations must block until ctx is canceled and return ctx.Err()
// so the hub can distinguish intentional shutdown from subscriber failure.
type SubscriberBridge interface {
	// Subscribe listens on channel and invokes onMessage for each received
	// payload. It must return when ctx is canceled.
	Subscribe(ctx context.Context, channel string, onMessage func(payload string)) error
}

// BridgeConfig wires a SubscriberBridge into the hub. Populate one of these
// for each external channel you want to forward events from.
type BridgeConfig struct {
	// Subscriber is the pub/sub implementation. Required.
	Subscriber SubscriberBridge

	// Transform optionally transforms the raw payload into a fully-formed
	// Event. Return nil to skip the message. If Transform is nil, the
	// payload is used as Event.Data with the defaults below.
	Transform func(payload string) *Event

	// Channel is the pub/sub channel to subscribe to. Required.
	Channel string

	// Topic is the SSE topic forwarded events are tagged with.
	// Defaults to Channel if empty.
	Topic string

	// EventType is the SSE event: field set on forwarded events.
	EventType string

	// CoalesceKey for PriorityCoalesced events.
	CoalesceKey string

	// TTL for forwarded events. Zero means no expiration.
	TTL time.Duration

	// Priority for forwarded events. PriorityInstant (0) is the default.
	Priority Priority
}

// runBridge consumes a single BridgeConfig, publishing incoming payloads
// until ctx is canceled. Retries on Subscribe errors with bridgeRetryDelay.
func (h *Hub) runBridge(ctx context.Context, cfg BridgeConfig) { //nolint:gocritic // hugeParam: value semantics preferred
	topic := cfg.Topic
	if topic == "" {
		topic = cfg.Channel
	}

	for {
		select {
		case <-ctx.Done():
			return
		default:
		}

		err := cfg.Subscriber.Subscribe(ctx, cfg.Channel, func(payload string) {
			if event := h.buildBridgeEvent(&cfg, topic, payload); event != nil {
				h.Publish(*event)
			}
		})

		if ctx.Err() != nil {
			return
		}

		// Any early return — error or unexpected nil from a well-behaved
		// subscriber — is treated as retryable. Without the backoff on
		// nil, a misbehaving subscriber that returns immediately would
		// spin this loop hot.
		if err != nil {
			logBridgeError(cfg.Channel, err)
		}
		select {
		case <-time.After(bridgeRetryDelay):
		case <-ctx.Done():
			return
		}
	}
}

// buildBridgeEvent creates an Event from a raw pub/sub payload.
// When Transform is set, the transform function controls all event fields;
// only missing Topics and Type are filled from config defaults.
// When Transform is not set, the event is built entirely from config defaults.
func (*Hub) buildBridgeEvent(cfg *BridgeConfig, topic, payload string) *Event {
	if cfg.Transform != nil {
		transformed := cfg.Transform(payload)
		if transformed == nil {
			return nil
		}
		event := *transformed
		if len(event.Topics) == 0 {
			event.Topics = []string{topic}
		}
		if event.Type == "" {
			event.Type = cfg.EventType
		}
		return &event
	}

	return &Event{
		Type:        cfg.EventType,
		Data:        payload,
		Topics:      []string{topic},
		Priority:    cfg.Priority,
		CoalesceKey: cfg.CoalesceKey,
		TTL:         cfg.TTL,
	}
}

// logBridgeError logs a bridge subscriber error. Retries continue after
// bridgeRetryDelay regardless of error type.
func logBridgeError(channel string, err error) {
	log.Warnf("sse: bridge subscriber error, retrying channel=%s error=%v", channel, err)
}

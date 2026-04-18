package sse

import (
	"context"
	"time"

	"github.com/gofiber/fiber/v3/log"
)

// PubSubSubscriber abstracts a pub/sub system (Redis, NATS, etc.) for
// auto-fan-out from an external message broker into the SSE hub.
type PubSubSubscriber interface {
	// Subscribe listens on the given channel and sends received messages
	// to the provided callback. It blocks until ctx is canceled.
	Subscribe(ctx context.Context, channel string, onMessage func(payload string)) error
}

// FanOutConfig configures auto-fan-out from an external pub/sub to the hub.
type FanOutConfig struct {
	// Subscriber is the pub/sub implementation (Redis, NATS, etc.).
	Subscriber PubSubSubscriber

	// Transform optionally transforms the raw pub/sub message before
	// publishing to the hub. Return nil to skip the message.
	Transform func(payload string) *Event

	// Channel is the pub/sub channel to subscribe to.
	Channel string

	// Topic is the SSE topic to publish events to. If empty, Channel is used.
	Topic string

	// EventType is the SSE event type. Required.
	EventType string

	// CoalesceKey for PriorityCoalesced events.
	CoalesceKey string

	// TTL for events. Zero means no expiration.
	TTL time.Duration

	// Priority for delivered events. Note: PriorityInstant is 0 (the zero value),
	// so it is always the default if not set explicitly.
	Priority Priority
}

// FanOut starts a goroutine that subscribes to an external pub/sub channel
// and automatically publishes received messages to the SSE hub.
// Returns a cancel function to stop the fan-out.
func (h *Hub) FanOut(cfg FanOutConfig) context.CancelFunc { //nolint:gocritic // hugeParam: public API, value semantics preferred
	if cfg.Subscriber == nil {
		panic("sse: FanOut requires a non-nil Subscriber")
	}

	ctx, cancel := context.WithCancel(context.Background())

	topic := cfg.Topic
	if topic == "" {
		topic = cfg.Channel
	}

	go func() {
		for {
			select {
			case <-ctx.Done():
				return
			default:
			}

			err := cfg.Subscriber.Subscribe(ctx, cfg.Channel, func(payload string) {
				event := h.buildFanOutEvent(&cfg, topic, payload)
				if event != nil {
					h.Publish(*event)
				}
			})

			if err != nil && ctx.Err() == nil {
				h.logFanOutError(cfg.Channel, err)
				select {
				case <-time.After(3 * time.Second):
				case <-ctx.Done():
					return
				}
			}
		}
	}()

	return cancel
}

// buildFanOutEvent creates an Event from a raw pub/sub payload.
// When Transform is set, the transform function controls all event fields;
// only missing Topics and Type are filled in from the config defaults.
// When Transform is not set, the event is built entirely from config defaults.
func (*Hub) buildFanOutEvent(cfg *FanOutConfig, topic, payload string) *Event {
	if cfg.Transform != nil {
		transformed := cfg.Transform(payload)
		if transformed == nil {
			return nil
		}
		event := *transformed
		// Only fill in missing Topics and Type — Transform controls everything else.
		if len(event.Topics) == 0 {
			event.Topics = []string{topic}
		}
		if event.Type == "" {
			event.Type = cfg.EventType
		}
		return &event
	}

	// Non-transform: build entirely from config defaults.
	event := Event{
		Type:        cfg.EventType,
		Data:        payload,
		Topics:      []string{topic},
		Priority:    cfg.Priority,
		CoalesceKey: cfg.CoalesceKey,
		TTL:         cfg.TTL,
	}

	return &event
}

// logFanOutError logs a fan-out subscriber error.
func (*Hub) logFanOutError(channel string, err error) {
	log.Warnf("sse: fan-out subscriber error, retrying channel=%s error=%v", channel, err)
}

// FanOutMulti starts multiple fan-out goroutines at once.
// Returns a single cancel function that stops all of them.
func (h *Hub) FanOutMulti(configs ...FanOutConfig) context.CancelFunc {
	ctx, cancel := context.WithCancel(context.Background())

	for _, cfg := range configs {
		innerCancel := h.FanOut(cfg)
		go func() {
			<-ctx.Done()
			innerCancel()
		}()
	}

	return cancel
}

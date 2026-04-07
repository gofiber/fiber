package sse

import (
	"time"
)

// InvalidationEvent is a lightweight signal telling the client to refetch
// a specific resource.
type InvalidationEvent struct {
	// Hint is optional extra data for the client.
	Hint map[string]any `json:"hint,omitempty"`

	// Resource is what changed (e.g., "orders", "products").
	Resource string `json:"resource"`

	// Action is what happened (e.g., "created", "updated", "deleted").
	Action string `json:"action"`

	// ResourceID is the specific item that changed (optional).
	ResourceID string `json:"resource_id,omitempty"`
}

// Invalidate publishes a cache invalidation signal to all connections
// subscribed to the given topic.
func (h *Hub) Invalidate(topic, resourceID, action string) {
	h.Publish(Event{
		Type:   "invalidate",
		Topics: []string{topic},
		Data: InvalidationEvent{
			Resource:   topic,
			Action:     action,
			ResourceID: resourceID,
		},
		Priority: PriorityInstant,
	})
}

// InvalidateForTenant publishes a tenant-scoped cache invalidation signal.
func (h *Hub) InvalidateForTenant(tenantID, topic, resourceID, action string) {
	h.Publish(Event{
		Type:   "invalidate",
		Topics: []string{topic},
		Group:  map[string]string{"tenant_id": tenantID},
		Data: InvalidationEvent{
			Resource:   topic,
			Action:     action,
			ResourceID: resourceID,
		},
		Priority: PriorityInstant,
	})
}

// InvalidateWithHint publishes an invalidation signal with extra data hints.
func (h *Hub) InvalidateWithHint(topic, resourceID, action string, hint map[string]any) {
	h.Publish(Event{
		Type:   "invalidate",
		Topics: []string{topic},
		Data: InvalidationEvent{
			Resource:   topic,
			Action:     action,
			ResourceID: resourceID,
			Hint:       hint,
		},
		Priority: PriorityInstant,
	})
}

// InvalidateForTenantWithHint publishes a tenant-scoped invalidation signal
// with extra data hints.
func (h *Hub) InvalidateForTenantWithHint(tenantID, topic, resourceID, action string, hint map[string]any) {
	h.Publish(Event{
		Type:   "invalidate",
		Topics: []string{topic},
		Group:  map[string]string{"tenant_id": tenantID},
		Data: InvalidationEvent{
			Resource:   topic,
			Action:     action,
			ResourceID: resourceID,
			Hint:       hint,
		},
		Priority: PriorityInstant,
	})
}

// Signal publishes a simple refresh signal.
func (h *Hub) Signal(topic string) {
	h.Publish(Event{
		Type:        "signal",
		Topics:      []string{topic},
		Data:        map[string]string{"signal": "refresh"},
		Priority:    PriorityCoalesced,
		CoalesceKey: "signal:" + topic,
	})
}

// SignalForTenant publishes a tenant-scoped refresh signal.
func (h *Hub) SignalForTenant(tenantID, topic string) {
	h.Publish(Event{
		Type:        "signal",
		Topics:      []string{topic},
		Group:       map[string]string{"tenant_id": tenantID},
		Data:        map[string]string{"signal": "refresh"},
		Priority:    PriorityCoalesced,
		CoalesceKey: "signal:" + topic + ":" + tenantID,
	})
}

// SignalThrottled publishes a signal with a TTL.
func (h *Hub) SignalThrottled(topic string, ttl time.Duration) {
	h.Publish(Event{
		Type:        "signal",
		Topics:      []string{topic},
		Data:        map[string]string{"signal": "refresh"},
		Priority:    PriorityCoalesced,
		CoalesceKey: "signal:" + topic,
		TTL:         ttl,
	})
}

package sse

import (
	"maps"
)

// DomainEvent publishes a domain event to the hub. This is the primary
// method for triggering real-time UI updates from your backend code.
//
// Parameters:
//   - resource: what changed ("orders", "products", "customers")
//   - action: what happened ("created", "updated", "deleted", "refresh")
//   - resourceID: specific item ID (empty for collection-level events)
//   - tenantID: tenant scope (empty for global events)
//   - hint: optional small payload (nil if not needed)
func (h *Hub) DomainEvent(resource, action, resourceID, tenantID string, hint map[string]any) {
	evt := InvalidationEvent{
		Resource:   resource,
		Action:     action,
		ResourceID: resourceID,
		Hint:       hint,
	}

	event := Event{
		Type:     "invalidate",
		Topics:   []string{resource},
		Data:     evt,
		Priority: PriorityInstant,
	}

	if tenantID != "" {
		event.Group = map[string]string{"tenant_id": tenantID}
	}

	h.Publish(event)
}

// Progress publishes a progress update for a long-running operation.
// Uses PriorityCoalesced — if progress goes 5%→8% in one flush
// window, only 8% is sent to the client.
func (h *Hub) Progress(topic, resourceID, tenantID string, current, total int, hint ...map[string]any) {
	pct := 0
	if total > 0 {
		pct = (current * 100) / total
	}

	data := map[string]any{
		"resource_id": resourceID,
		"current":     current,
		"total":       total,
		"pct":         pct,
	}
	if len(hint) > 0 && hint[0] != nil {
		maps.Copy(data, hint[0])
	}

	event := Event{
		Type:        "progress",
		Topics:      []string{topic},
		Data:        data,
		Priority:    PriorityCoalesced,
		CoalesceKey: "progress:" + topic + ":" + resourceID,
	}

	if tenantID != "" {
		event.Group = map[string]string{"tenant_id": tenantID}
	}

	h.Publish(event)
}

// Complete publishes a completion signal for a long-running operation.
// Uses PriorityInstant — completion always delivers immediately.
func (h *Hub) Complete(topic, resourceID, tenantID string, success bool, hint map[string]any) { //nolint:revive // flag-parameter: public API toggle
	action := "completed"
	if !success {
		action = "failed"
	}

	data := map[string]any{
		"resource_id": resourceID,
		"status":      action,
	}
	maps.Copy(data, hint)

	event := Event{
		Type:     "complete",
		Topics:   []string{topic},
		Data:     data,
		Priority: PriorityInstant,
	}

	if tenantID != "" {
		event.Group = map[string]string{"tenant_id": tenantID}
	}

	h.Publish(event)
}

// DomainEventSpec describes a single domain event within a batch.
type DomainEventSpec struct {
	Hint       map[string]any `json:"hint,omitempty"`
	Resource   string         `json:"resource"`
	Action     string         `json:"action"`
	ResourceID string         `json:"resource_id,omitempty"`
}

// BatchDomainEvents publishes multiple domain events as a single SSE frame.
// The event is delivered to any connection subscribed to ANY of the resources
// in the batch. This is by design — batches target clients subscribed to
// multiple topics (e.g., a dashboard). Clients should filter the specs array
// locally by resource if they only care about a subset.
func (h *Hub) BatchDomainEvents(tenantID string, specs []DomainEventSpec) {
	if len(specs) == 0 {
		return
	}
	topicSet := make(map[string]struct{})
	for _, s := range specs {
		topicSet[s.Resource] = struct{}{}
	}
	topics := make([]string, 0, len(topicSet))
	for t := range topicSet {
		topics = append(topics, t)
	}
	batchEvt := Event{
		Type:     "batch",
		Topics:   topics,
		Data:     specs,
		Priority: PriorityInstant,
	}
	if tenantID != "" {
		batchEvt.Group = map[string]string{"tenant_id": tenantID}
	}
	h.Publish(batchEvt)
}

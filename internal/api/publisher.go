package api

import (
	"context"
	"encoding/json"
	"log/slog"

	"github.com/t0mer/cylon/internal/store"
)

// WSMessage is the envelope broadcast over /ws so the UI can dispatch by type.
type WSMessage struct {
	Type  string       `json:"type"`
	Event *store.Event `json:"event,omitempty"`
}

// Publisher persists a domain event and broadcasts it to the live feed. It is
// the seam the gateway, tags, and orchestrator use to surface activity; it
// satisfies any local "emitter" interface with a Publish(ctx, store.Event) method.
type Publisher struct {
	events *store.EventRepo
	hub    *Hub
	log    *slog.Logger
}

// NewPublisher wires an event repository and the WS hub together.
func NewPublisher(events *store.EventRepo, hub *Hub, log *slog.Logger) *Publisher {
	if log == nil {
		log = slog.Default()
	}
	return &Publisher{events: events, hub: hub, log: log}
}

// Publish appends the event to the store and broadcasts it. Persistence failures
// are logged but never block the caller (events are best-effort telemetry).
func (p *Publisher) Publish(ctx context.Context, e store.Event) {
	id, err := p.events.Append(ctx, e)
	if err != nil {
		p.log.Warn("persisting event", "err", err)
	} else {
		e.ID = id
	}
	b, err := json.Marshal(WSMessage{Type: "event", Event: &e})
	if err != nil {
		return
	}
	p.hub.Broadcast(b)
}

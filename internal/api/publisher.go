package api

import (
	"context"
	"encoding/json"
	"log/slog"

	"github.com/t0mer/cylon/internal/metrics"
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
	events  *store.EventRepo
	hub     *Hub
	metrics *metrics.Metrics
	log     *slog.Logger
}

// NewPublisher wires an event repository, the WS hub, and (optionally) metrics
// together. metrics may be nil.
func NewPublisher(events *store.EventRepo, hub *Hub, m *metrics.Metrics, log *slog.Logger) *Publisher {
	if log == nil {
		log = slog.Default()
	}
	return &Publisher{events: events, hub: hub, metrics: m, log: log}
}

// Publish appends the event to the store, updates metrics, and broadcasts it.
// Persistence failures are logged but never block the caller (events are
// best-effort telemetry).
func (p *Publisher) Publish(ctx context.Context, e store.Event) {
	id, err := p.events.Append(ctx, e)
	if err != nil {
		p.log.Warn("persisting event", "err", err)
	} else {
		e.ID = id
	}
	p.record(e)
	b, err := json.Marshal(WSMessage{Type: "event", Event: &e})
	if err != nil {
		return
	}
	p.hub.Broadcast(b)
}

func (p *Publisher) record(e store.Event) {
	if p.metrics == nil {
		return
	}
	switch {
	case e.Kind == "join":
		result := e.Result
		if result == "" {
			result = "success"
		}
		p.metrics.Joins.WithLabelValues(result).Inc()
	case e.Direction == "up" && e.Kind == "data":
		p.metrics.Uplinks.WithLabelValues("data").Inc()
	case e.Direction == "down":
		p.metrics.Downlinks.WithLabelValues("A").Inc()
	}
}

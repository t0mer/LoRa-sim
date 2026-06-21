// Package sim is the orchestrator and scenario engine. It manages in-process
// tags (loaded from the database), connecting each to the gateway over loopback
// TCP, and exposes scenario primitives (join_all, uplink, burst). Tag activity
// is surfaced as events through the Emitter.
package sim

import (
	"context"
	"encoding/hex"
	"fmt"
	"log/slog"
	"sync"

	"github.com/t0mer/cylon/internal/store"
	"github.com/t0mer/cylon/internal/tag"
	"github.com/t0mer/cylon/internal/transport"
)

// Emitter publishes domain events (persisted + broadcast). *api.Publisher
// satisfies it; the interface lives here to avoid importing the api package.
type Emitter interface {
	Publish(ctx context.Context, e store.Event)
}

// Orchestrator runs in-process tags against the gateway's TCP listener.
type Orchestrator struct {
	store  *store.Store
	gwAddr string
	emit   Emitter
	log    *slog.Logger

	mu      sync.Mutex
	running map[int64]*runningTag
}

type runningTag struct {
	client *tag.Client
	conn   *transport.Conn
	cancel context.CancelFunc
}

// New creates an orchestrator that dials gwAddr (the gateway tag-facing TCP
// address) for each in-process tag.
func New(st *store.Store, gwAddr string, emit Emitter, log *slog.Logger) *Orchestrator {
	if log == nil {
		log = slog.Default()
	}
	return &Orchestrator{
		store:   st,
		gwAddr:  gwAddr,
		emit:    emit,
		log:     log,
		running: make(map[int64]*runningTag),
	}
}

// Start brings a tag online: it dials the gateway, starts the read loop, sends
// hello, and joins if the tag has no session yet. Starting an already-running
// tag is a no-op.
func (o *Orchestrator) Start(ctx context.Context, tagID int64) error {
	o.mu.Lock()
	if _, ok := o.running[tagID]; ok {
		o.mu.Unlock()
		return nil
	}
	o.mu.Unlock()

	tg, err := o.store.Tags().Get(ctx, tagID)
	if err != nil {
		return err
	}
	dev, err := tag.NewDevice(*tg, o.store)
	if err != nil {
		return err
	}
	conn, err := transport.Dial(o.gwAddr)
	if err != nil {
		return fmt.Errorf("dialing gateway: %w", err)
	}

	client := tag.NewClient(dev, conn)
	client.DownlinkHook = func(dl *tag.Downlink) {
		o.emitDownlink(context.Background(), tagID, dl)
	}

	runCtx, cancel := context.WithCancel(context.Background())
	go func() {
		if err := client.Run(runCtx); err != nil && runCtx.Err() == nil {
			o.log.Debug("tag run ended", "tag_id", tagID, "err", err)
		}
	}()
	if err := client.Hello(); err != nil {
		cancel()
		conn.Close()
		return err
	}

	sess, err := o.store.Sessions().Get(ctx, tagID)
	if err != nil {
		cancel()
		conn.Close()
		return err
	}
	if !sess.Joined {
		if err := client.Join(ctx); err != nil {
			cancel()
			conn.Close()
			o.emit.Publish(ctx, store.Event{TagID: &tagID, Direction: "up", Kind: "join", Result: "error"})
			return fmt.Errorf("join: %w", err)
		}
		if s, err := o.store.Sessions().Get(ctx, tagID); err == nil {
			o.emit.Publish(ctx, store.Event{TagID: &tagID, Direction: "up", Kind: "join", Result: "success", Decoded: `{"dev_addr":"` + s.DevAddr + `"}`})
		}
	}

	o.mu.Lock()
	o.running[tagID] = &runningTag{client: client, conn: conn, cancel: cancel}
	o.mu.Unlock()
	return nil
}

// Uplink sends one uplink from a running tag, emitting an event.
func (o *Orchestrator) Uplink(ctx context.Context, tagID int64, override []byte) error {
	o.mu.Lock()
	rt, ok := o.running[tagID]
	o.mu.Unlock()
	if !ok {
		return fmt.Errorf("tag %d is not running", tagID)
	}
	fcnt, err := rt.client.SendUplink(ctx, override)
	if err != nil {
		return err
	}
	f := int64(fcnt)
	fport := int64(rt.client.FPort())
	o.emit.Publish(ctx, store.Event{TagID: &tagID, Direction: "up", Kind: "data", FCnt: &f, FPort: &fport})
	return nil
}

// JoinAll starts every enabled tag concurrently (each joins as needed).
func (o *Orchestrator) JoinAll(ctx context.Context) (started int, errs int) {
	tags, err := o.store.Tags().List(ctx)
	if err != nil {
		return 0, 1
	}
	var wg sync.WaitGroup
	var mu sync.Mutex
	for _, t := range tags {
		if !t.Enabled {
			continue
		}
		id := t.ID
		wg.Add(1)
		go func() {
			defer wg.Done()
			if err := o.Start(ctx, id); err != nil {
				mu.Lock()
				errs++
				mu.Unlock()
				return
			}
			mu.Lock()
			started++
			mu.Unlock()
		}()
	}
	wg.Wait()
	return started, errs
}

// Burst sends count uplinks across the running tags (round-robin), with up to
// atOnce uplinks in flight at a time.
func (o *Orchestrator) Burst(ctx context.Context, count, atOnce int) error {
	o.mu.Lock()
	ids := make([]int64, 0, len(o.running))
	for id := range o.running {
		ids = append(ids, id)
	}
	o.mu.Unlock()
	if len(ids) == 0 {
		return fmt.Errorf("no running tags")
	}
	if atOnce <= 0 {
		atOnce = 1
	}

	sem := make(chan struct{}, atOnce)
	var wg sync.WaitGroup
	for i := 0; i < count; i++ {
		id := ids[i%len(ids)]
		wg.Add(1)
		sem <- struct{}{}
		go func() {
			defer wg.Done()
			defer func() { <-sem }()
			if err := o.Uplink(ctx, id, nil); err != nil {
				o.log.Debug("burst uplink", "tag_id", id, "err", err)
			}
		}()
	}
	wg.Wait()
	return nil
}

// Running returns the ids of currently running tags.
func (o *Orchestrator) Running() []int64 {
	o.mu.Lock()
	defer o.mu.Unlock()
	ids := make([]int64, 0, len(o.running))
	for id := range o.running {
		ids = append(ids, id)
	}
	return ids
}

// Stop disconnects a running tag.
func (o *Orchestrator) Stop(tagID int64) {
	o.mu.Lock()
	rt, ok := o.running[tagID]
	delete(o.running, tagID)
	o.mu.Unlock()
	if ok {
		rt.cancel()
		rt.conn.Close()
	}
}

// StopAll disconnects every running tag.
func (o *Orchestrator) StopAll() {
	o.mu.Lock()
	all := o.running
	o.running = make(map[int64]*runningTag)
	o.mu.Unlock()
	for _, rt := range all {
		rt.cancel()
		rt.conn.Close()
	}
}

func (o *Orchestrator) emitDownlink(ctx context.Context, tagID int64, dl *tag.Downlink) {
	kind := "data"
	if len(dl.MACCommands) > 0 {
		kind = "macdown"
	} else if dl.ACK && len(dl.Payload) == 0 {
		kind = "ack"
	}
	fcnt := int64(dl.FCnt)
	fport := int64(dl.FPort)
	o.emit.Publish(ctx, store.Event{
		TagID: &tagID, Direction: "down", Kind: kind,
		FCnt: &fcnt, FPort: &fport, PayloadHex: hex.EncodeToString(dl.Payload),
	})
}

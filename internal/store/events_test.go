package store

import (
	"context"
	"path/filepath"
	"testing"
)

func i64(v int64) *int64 { return &v }

func TestEventAppendAndList(t *testing.T) {
	s := newEncStore(t, filepath.Join(t.TempDir(), "c.db"))
	ctx := context.Background()
	tg, _ := s.Tags().Create(ctx, sampleTag())

	for i := 0; i < 3; i++ {
		if _, err := s.Events().Append(ctx, Event{
			TagID: &tg.ID, Direction: "up", Kind: "data",
			Freq: i64(868100000), DR: i64(5), FCnt: i64(int64(i)), FPort: i64(10),
			PayloadHex: "aabb",
		}); err != nil {
			t.Fatalf("Append: %v", err)
		}
	}
	if _, err := s.Events().Append(ctx, Event{Direction: "down", Kind: "join"}); err != nil {
		t.Fatalf("Append down: %v", err)
	}

	all, err := s.Events().List(ctx, EventFilter{})
	if err != nil {
		t.Fatalf("List: %v", err)
	}
	if len(all) != 4 {
		t.Fatalf("List = %d events, want 4", len(all))
	}
	// Newest first.
	if all[0].Direction != "down" {
		t.Errorf("first event direction = %q, want down (newest)", all[0].Direction)
	}
	if all[0].TagID != nil {
		t.Errorf("down event TagID = %v, want nil", all[0].TagID)
	}
	// Nullable round-trip on a data event.
	last := all[3]
	if last.Freq == nil || *last.Freq != 868100000 || last.FPort == nil || *last.FPort != 10 {
		t.Errorf("data event fields: %+v", last)
	}
}

func TestEventFilterByTagAndDirection(t *testing.T) {
	s := newEncStore(t, filepath.Join(t.TempDir(), "c.db"))
	ctx := context.Background()
	a, _ := s.Tags().Create(ctx, sampleTag())
	bTag := sampleTag()
	bTag.DevEUI = "0202020202020202"
	b, _ := s.Tags().Create(ctx, bTag)

	s.Events().Append(ctx, Event{TagID: &a.ID, Direction: "up", Kind: "data"})
	s.Events().Append(ctx, Event{TagID: &a.ID, Direction: "down", Kind: "data"})
	s.Events().Append(ctx, Event{TagID: &b.ID, Direction: "up", Kind: "data"})

	got, err := s.Events().List(ctx, EventFilter{TagID: &a.ID})
	if err != nil {
		t.Fatal(err)
	}
	if len(got) != 2 {
		t.Errorf("by tag a = %d, want 2", len(got))
	}

	up, _ := s.Events().List(ctx, EventFilter{Direction: "up"})
	if len(up) != 2 {
		t.Errorf("by direction up = %d, want 2", len(up))
	}
}

func TestEventPagination(t *testing.T) {
	s := newEncStore(t, filepath.Join(t.TempDir(), "c.db"))
	ctx := context.Background()
	for i := 0; i < 5; i++ {
		s.Events().Append(ctx, Event{Direction: "up", Kind: "data"})
	}
	page1, _ := s.Events().List(ctx, EventFilter{Limit: 2})
	if len(page1) != 2 {
		t.Fatalf("page1 = %d, want 2", len(page1))
	}
	page2, _ := s.Events().List(ctx, EventFilter{Limit: 2, BeforeID: page1[1].ID})
	if len(page2) != 2 {
		t.Fatalf("page2 = %d, want 2", len(page2))
	}
	if page2[0].ID >= page1[1].ID {
		t.Errorf("pagination overlap: page2[0].ID=%d >= page1[1].ID=%d", page2[0].ID, page1[1].ID)
	}
}

func TestEventPrune(t *testing.T) {
	s := newEncStore(t, filepath.Join(t.TempDir(), "c.db"))
	ctx := context.Background()
	for i := 0; i < 10; i++ {
		s.Events().Append(ctx, Event{Direction: "up", Kind: "data"})
	}
	deleted, err := s.Events().Prune(ctx, 4)
	if err != nil {
		t.Fatalf("Prune: %v", err)
	}
	if deleted != 6 {
		t.Errorf("pruned %d, want 6", deleted)
	}
	remaining, _ := s.Events().List(ctx, EventFilter{Limit: 100})
	if len(remaining) != 4 {
		t.Errorf("remaining %d, want 4", len(remaining))
	}
}

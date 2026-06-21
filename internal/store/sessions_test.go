package store

import (
	"context"
	"path/filepath"
	"testing"

	"github.com/t0mer/cylon/internal/db"
	"github.com/t0mer/cylon/internal/secret"
)

func TestDevNonceMonotonic(t *testing.T) {
	s := newEncStore(t, filepath.Join(t.TempDir(), "c.db"))
	ctx := context.Background()
	tg, _ := s.Tags().Create(ctx, sampleTag())

	var last uint16
	for i := 0; i < 5; i++ {
		n, err := s.Sessions().NextDevNonce(ctx, tg.ID)
		if err != nil {
			t.Fatalf("NextDevNonce: %v", err)
		}
		if n != last+1 {
			t.Fatalf("DevNonce = %d, want %d (monotonic +1)", n, last+1)
		}
		last = n
	}
}

func TestDevNoncePersistsAcrossReopen(t *testing.T) {
	path := filepath.Join(t.TempDir(), "c.db")
	ctx := context.Background()

	// First store instance: advance the nonce.
	s1 := newEncStore(t, path)
	tg, _ := s1.Tags().Create(ctx, sampleTag())
	var lastNonce uint16
	for i := 0; i < 3; i++ {
		lastNonce, _ = s1.Sessions().NextDevNonce(ctx, tg.ID)
	}
	s1.DB().Close()

	// Second store instance on the same file: nonce must continue, never reset.
	database, err := db.Open(path)
	if err != nil {
		t.Fatalf("reopen: %v", err)
	}
	t.Cleanup(func() { database.Close() })
	cipher, _ := secret.New(storeTestKey)
	s2 := New(database, cipher)

	got, err := s2.Sessions().Get(ctx, tg.ID)
	if err != nil {
		t.Fatalf("Get after reopen: %v", err)
	}
	if got.DevNonce != lastNonce {
		t.Errorf("persisted DevNonce = %d, want %d", got.DevNonce, lastNonce)
	}
	next, _ := s2.Sessions().NextDevNonce(ctx, tg.ID)
	if next != lastNonce+1 {
		t.Errorf("DevNonce after reopen = %d, want %d (no rollback)", next, lastNonce+1)
	}
}

func TestSaveJoinResultRoundTrip(t *testing.T) {
	s := newEncStore(t, filepath.Join(t.TempDir(), "c.db"))
	ctx := context.Background()
	tg, _ := s.Tags().Create(ctx, sampleTag())

	js := JoinState{
		DevAddr:     "01020304",
		NwkSKey:     "aaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaa",
		AppSKey:     "bbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbb",
		RxDelay:     1,
		RX1DROffset: 0,
		RX2DR:       3,
		RX2Freq:     869525000,
	}
	if err := s.Sessions().SaveJoinResult(ctx, tg.ID, js); err != nil {
		t.Fatalf("SaveJoinResult: %v", err)
	}

	got, err := s.Sessions().Get(ctx, tg.ID)
	if err != nil {
		t.Fatalf("Get: %v", err)
	}
	if !got.Joined {
		t.Error("session not marked joined")
	}
	if got.NwkSKey != js.NwkSKey || got.AppSKey != js.AppSKey {
		t.Errorf("session keys round-trip mismatch")
	}
	if got.DevAddr != "01020304" || got.RX2Freq != 869525000 {
		t.Errorf("session fields mismatch: %+v", got)
	}

	// Session keys must be encrypted at rest.
	var rawNwk string
	s.DB().QueryRow(`SELECT nwk_skey FROM sessions WHERE tag_id = ?`, tg.ID).Scan(&rawNwk)
	if rawNwk == js.NwkSKey {
		t.Errorf("nwk_skey stored in plaintext")
	}
}

func TestTakeFCntUp(t *testing.T) {
	s := newEncStore(t, filepath.Join(t.TempDir(), "c.db"))
	ctx := context.Background()
	tg, _ := s.Tags().Create(ctx, sampleTag())

	for want := uint32(0); want < 3; want++ {
		got, err := s.Sessions().TakeFCntUp(ctx, tg.ID)
		if err != nil {
			t.Fatalf("TakeFCntUp: %v", err)
		}
		if got != want {
			t.Errorf("TakeFCntUp = %d, want %d", got, want)
		}
	}
}

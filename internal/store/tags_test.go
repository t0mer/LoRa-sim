package store

import (
	"context"
	"path/filepath"
	"testing"

	"github.com/t0mer/cylon/internal/db"
	"github.com/t0mer/cylon/internal/secret"
)

const storeTestKey = "000102030405060708090a0b0c0d0e0f101112131415161718191a1b1c1d1e1f"

func newEncStore(t *testing.T, path string) *Store {
	t.Helper()
	database, err := db.Open(path)
	if err != nil {
		t.Fatalf("db.Open: %v", err)
	}
	t.Cleanup(func() { database.Close() })
	if err := db.Migrate(database, "up"); err != nil {
		t.Fatalf("migrate: %v", err)
	}
	cipher, err := secret.New(storeTestKey)
	if err != nil {
		t.Fatalf("secret.New: %v", err)
	}
	return New(database, cipher)
}

func sampleTag() NewTag {
	return NewTag{
		DevEUI:      "0101010101010101",
		JoinEUI:     "0202020202020202",
		AppKey:      "00112233445566778899aabbccddeeff",
		Class:       "A",
		Region:      "EU868",
		SubBand:     2,
		DefaultDR:   5,
		FPort:       10,
		PayloadType: "counter",
		Enabled:     true,
	}
}

func TestTagCreateGet(t *testing.T) {
	s := newEncStore(t, filepath.Join(t.TempDir(), "c.db"))
	ctx := context.Background()

	created, err := s.Tags().Create(ctx, sampleTag())
	if err != nil {
		t.Fatalf("Create: %v", err)
	}
	if created.ID == 0 {
		t.Fatal("created tag has zero id")
	}

	got, err := s.Tags().Get(ctx, created.ID)
	if err != nil {
		t.Fatalf("Get: %v", err)
	}
	if got.AppKey != "00112233445566778899aabbccddeeff" {
		t.Errorf("AppKey round-trip = %q", got.AppKey)
	}
	if got.DevEUI != "0101010101010101" || got.Region != "EU868" {
		t.Errorf("tag fields mismatch: %+v", got)
	}

	// A 1:1 session row must exist after Create.
	if _, err := s.Sessions().Get(ctx, created.ID); err != nil {
		t.Errorf("session not created with tag: %v", err)
	}
}

func TestAppKeyEncryptedAtRest(t *testing.T) {
	path := filepath.Join(t.TempDir(), "c.db")
	s := newEncStore(t, path)
	ctx := context.Background()
	created, err := s.Tags().Create(ctx, sampleTag())
	if err != nil {
		t.Fatalf("Create: %v", err)
	}

	// Read the raw column directly: it must NOT contain the plaintext key.
	var raw string
	if err := s.DB().QueryRow(`SELECT app_key FROM tags WHERE id = ?`, created.ID).Scan(&raw); err != nil {
		t.Fatalf("raw read: %v", err)
	}
	if raw == "00112233445566778899aabbccddeeff" {
		t.Errorf("app_key stored in plaintext: %q", raw)
	}
	if len(raw) < 4 || raw[:3] != "v1:" {
		t.Errorf("app_key not in encrypted form: %q", raw)
	}
}

func TestGetByDevEUIAndDelete(t *testing.T) {
	s := newEncStore(t, filepath.Join(t.TempDir(), "c.db"))
	ctx := context.Background()
	created, _ := s.Tags().Create(ctx, sampleTag())

	got, err := s.Tags().GetByDevEUI(ctx, "0101010101010101")
	if err != nil || got.ID != created.ID {
		t.Fatalf("GetByDevEUI = (%v, %v)", got, err)
	}
	if err := s.Tags().Delete(ctx, created.ID); err != nil {
		t.Fatalf("Delete: %v", err)
	}
	if _, err := s.Tags().Get(ctx, created.ID); err != ErrNotFound {
		t.Errorf("Get after delete = %v, want ErrNotFound", err)
	}
	// Session cascade-deleted.
	if _, err := s.Sessions().Get(ctx, created.ID); err != ErrNotFound {
		t.Errorf("session after tag delete = %v, want ErrNotFound", err)
	}
}

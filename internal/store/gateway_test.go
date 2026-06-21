package store

import (
	"context"
	"path/filepath"
	"testing"

	"github.com/t0mer/cylon/internal/db"
	"github.com/t0mer/cylon/internal/secret"
)

func newTestStore(t *testing.T) *Store {
	t.Helper()
	path := filepath.Join(t.TempDir(), "cylon.db")
	database, err := db.Open(path)
	if err != nil {
		t.Fatalf("db.Open: %v", err)
	}
	t.Cleanup(func() { database.Close() })
	if err := db.Migrate(database, "up"); err != nil {
		t.Fatalf("migrate: %v", err)
	}
	return New(database, secret.NewInsecureDisabled())
}

func TestGatewayGetNotFound(t *testing.T) {
	s := newTestStore(t)
	if _, err := s.Gateway().Get(context.Background()); err != ErrNotFound {
		t.Errorf("Get on empty db = %v, want ErrNotFound", err)
	}
}

func TestEnsureEUIGeneratesAndPersists(t *testing.T) {
	s := newTestStore(t)
	ctx := context.Background()

	g1, err := s.Gateway().EnsureEUI(ctx, "", "")
	if err != nil {
		t.Fatalf("EnsureEUI: %v", err)
	}
	if len(g1.EUI) != 16 {
		t.Fatalf("generated EUI %q invalid", g1.EUI)
	}

	// Second call must return the SAME persisted EUI (stable across restarts).
	g2, err := s.Gateway().EnsureEUI(ctx, "", "")
	if err != nil {
		t.Fatalf("EnsureEUI second: %v", err)
	}
	if g2.EUI != g1.EUI {
		t.Errorf("EUI changed across calls: %q -> %q", g1.EUI, g2.EUI)
	}
}

func TestEnsureEUIConfiguredOverrideWins(t *testing.T) {
	s := newTestStore(t)
	ctx := context.Background()

	// Generate one first.
	g1, _ := s.Gateway().EnsureEUI(ctx, "", "")

	// Now a configured EUI must override and persist.
	g2, err := s.Gateway().EnsureEUI(ctx, "AABBCCDDEEFF0011", "")
	if err != nil {
		t.Fatalf("EnsureEUI override: %v", err)
	}
	if g2.EUI != "aabbccddeeff0011" {
		t.Errorf("override EUI = %q, want aabbccddeeff0011", g2.EUI)
	}
	if g2.EUI == g1.EUI {
		t.Errorf("override did not change EUI")
	}

	got, _ := s.Gateway().Get(ctx)
	if got.EUI != "aabbccddeeff0011" {
		t.Errorf("persisted EUI = %q, want aabbccddeeff0011", got.EUI)
	}
}

func TestSetEUIRejectsInvalid(t *testing.T) {
	s := newTestStore(t)
	if _, err := s.Gateway().SetEUI(context.Background(), "nothex"); err == nil {
		t.Errorf("SetEUI(nothex) = nil, want error")
	}
}

func TestGatewayDefaults(t *testing.T) {
	s := newTestStore(t)
	g, err := s.Gateway().EnsureEUI(context.Background(), "", "")
	if err != nil {
		t.Fatalf("EnsureEUI: %v", err)
	}
	if g.Region != "EU868" {
		t.Errorf("Region = %q, want EU868 default", g.Region)
	}
	if g.SubBand != 2 {
		t.Errorf("SubBand = %d, want 2 default", g.SubBand)
	}
	if g.ConnectionMode != "cups" {
		t.Errorf("ConnectionMode = %q, want cups default", g.ConnectionMode)
	}
}

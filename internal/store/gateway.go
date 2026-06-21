package store

import (
	"context"
	"database/sql"
	"errors"
	"fmt"

	"github.com/t0mer/cylon/internal/gateway"
)

// Gateway is the single logical gateway row.
type Gateway struct {
	EUI            string
	Region         string
	SubBand        int
	ConnectionMode string
	CreatedAt      string
	UpdatedAt      string
}

// GatewayRepo reads and writes the single gateway row (id = 1).
type GatewayRepo struct {
	s *Store
}

// Get returns the gateway row, or ErrNotFound if it does not exist yet.
func (r *GatewayRepo) Get(ctx context.Context) (*Gateway, error) {
	var g Gateway
	err := r.s.db.QueryRowContext(ctx,
		`SELECT eui, region, sub_band, connection_mode, created_at, updated_at
		   FROM gateway WHERE id = 1`,
	).Scan(&g.EUI, &g.Region, &g.SubBand, &g.ConnectionMode, &g.CreatedAt, &g.UpdatedAt)
	if errors.Is(err, sql.ErrNoRows) {
		return nil, ErrNotFound
	}
	if err != nil {
		return nil, fmt.Errorf("querying gateway: %w", err)
	}
	return &g, nil
}

// create inserts the id=1 gateway row with the given (already-normalized) EUI.
func (r *GatewayRepo) create(ctx context.Context, eui string) (*Gateway, error) {
	now := r.s.clock()
	r.s.wmu.Lock()
	_, err := r.s.db.ExecContext(ctx,
		`INSERT INTO gateway (id, eui, created_at, updated_at) VALUES (1, ?, ?, ?)`,
		eui, now, now)
	r.s.wmu.Unlock()
	if err != nil {
		return nil, fmt.Errorf("inserting gateway: %w", err)
	}
	return r.Get(ctx)
}

// SetEUI updates the gateway EUI, creating the row if absent. The EUI is
// normalized and validated before persisting.
func (r *GatewayRepo) SetEUI(ctx context.Context, eui string) (*Gateway, error) {
	norm, err := gateway.NormalizeEUI(eui)
	if err != nil {
		return nil, err
	}
	if _, err := r.Get(ctx); errors.Is(err, ErrNotFound) {
		return r.create(ctx, norm)
	} else if err != nil {
		return nil, err
	}

	now := r.s.clock()
	r.s.wmu.Lock()
	_, err = r.s.db.ExecContext(ctx,
		`UPDATE gateway SET eui = ?, updated_at = ? WHERE id = 1`, norm, now)
	r.s.wmu.Unlock()
	if err != nil {
		return nil, fmt.Errorf("updating gateway eui: %w", err)
	}
	return r.Get(ctx)
}

// EnsureEUI resolves the gateway EUI for startup. A configured EUI (from config
// or env) wins and is persisted; otherwise an existing row is reused; otherwise
// a fresh EUI is generated from the optional prefix and persisted.
func (r *GatewayRepo) EnsureEUI(ctx context.Context, configured, prefix string) (*Gateway, error) {
	if configured != "" {
		return r.SetEUI(ctx, configured)
	}
	if g, err := r.Get(ctx); err == nil {
		return g, nil
	} else if !errors.Is(err, ErrNotFound) {
		return nil, err
	}
	eui, err := gateway.GenerateEUI(prefix)
	if err != nil {
		return nil, err
	}
	return r.create(ctx, eui)
}

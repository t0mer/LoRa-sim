package store

import (
	"context"
	"database/sql"
	"errors"
	"fmt"
)

// Tag is a provisioned simulated device. AppKey is returned decrypted for
// internal simulator use; the API layer is responsible for masking it.
type Tag struct {
	ID            int64
	DevEUI        string
	JoinEUI       string
	AppKey        string
	Class         string
	Region        string
	SubBand       int
	DefaultDR     int
	FPort         int
	PayloadType   string
	PayloadConfig string
	Schedule      string
	Enabled       bool
	CreatedAt     string
	UpdatedAt     string
}

// NewTag is the input for creating a tag.
type NewTag struct {
	DevEUI        string
	JoinEUI       string
	AppKey        string
	Class         string
	Region        string
	SubBand       int
	DefaultDR     int
	FPort         int
	PayloadType   string
	PayloadConfig string
	Schedule      string
	Enabled       bool
}

// TagRepo reads and writes tag rows and their 1:1 session rows.
type TagRepo struct {
	s *Store
}

func tagAAD(devEUI, column string) []byte {
	return []byte("tag|" + devEUI + "|" + column)
}

// Create inserts a tag (with its AppKey encrypted at rest) and an empty session
// row, returning the stored tag.
func (r *TagRepo) Create(ctx context.Context, in NewTag) (*Tag, error) {
	encKey, err := r.s.cipher.Encrypt(in.AppKey, tagAAD(in.DevEUI, "app_key"))
	if err != nil {
		return nil, fmt.Errorf("encrypting app_key: %w", err)
	}
	now := r.s.clock()

	r.s.wmu.Lock()
	defer r.s.wmu.Unlock()

	tx, err := r.s.db.BeginTx(ctx, nil)
	if err != nil {
		return nil, err
	}
	defer tx.Rollback() //nolint:errcheck // rolled back unless committed

	res, err := tx.ExecContext(ctx,
		`INSERT INTO tags
		   (dev_eui, join_eui, app_key, class, region, sub_band, default_dr,
		    fport, payload_type, payload_config, schedule, enabled, created_at, updated_at)
		 VALUES (?,?,?,?,?,?,?,?,?,?,?,?,?,?)`,
		in.DevEUI, in.JoinEUI, encKey, in.Class, in.Region, in.SubBand, in.DefaultDR,
		in.FPort, in.PayloadType, nullable(in.PayloadConfig), nullable(in.Schedule),
		boolToInt(in.Enabled), now, now)
	if err != nil {
		return nil, fmt.Errorf("inserting tag: %w", err)
	}
	id, err := res.LastInsertId()
	if err != nil {
		return nil, err
	}
	if _, err := tx.ExecContext(ctx,
		`INSERT INTO sessions (tag_id, dev_nonce, joined, updated_at) VALUES (?, 0, 0, ?)`,
		id, now); err != nil {
		return nil, fmt.Errorf("inserting session: %w", err)
	}
	if err := tx.Commit(); err != nil {
		return nil, err
	}
	return r.get(ctx, r.s.db, id)
}

// Get returns the tag by id.
func (r *TagRepo) Get(ctx context.Context, id int64) (*Tag, error) {
	return r.get(ctx, r.s.db, id)
}

type rowQueryer interface {
	QueryRowContext(ctx context.Context, query string, args ...any) *sql.Row
}

func (r *TagRepo) get(ctx context.Context, q rowQueryer, id int64) (*Tag, error) {
	row := q.QueryRowContext(ctx,
		`SELECT id, dev_eui, join_eui, app_key, class, region, sub_band, default_dr,
		        fport, payload_type, payload_config, schedule, enabled, created_at, updated_at
		   FROM tags WHERE id = ?`, id)
	return scanTag(r.s.cipher, row)
}

// GetByDevEUI returns the tag with the given DevEUI.
func (r *TagRepo) GetByDevEUI(ctx context.Context, devEUI string) (*Tag, error) {
	row := r.s.db.QueryRowContext(ctx,
		`SELECT id, dev_eui, join_eui, app_key, class, region, sub_band, default_dr,
		        fport, payload_type, payload_config, schedule, enabled, created_at, updated_at
		   FROM tags WHERE dev_eui = ?`, devEUI)
	return scanTag(r.s.cipher, row)
}

// List returns all tags ordered by id.
func (r *TagRepo) List(ctx context.Context) ([]Tag, error) {
	rows, err := r.s.db.QueryContext(ctx,
		`SELECT id, dev_eui, join_eui, app_key, class, region, sub_band, default_dr,
		        fport, payload_type, payload_config, schedule, enabled, created_at, updated_at
		   FROM tags ORDER BY id`)
	if err != nil {
		return nil, fmt.Errorf("listing tags: %w", err)
	}
	defer rows.Close()

	var out []Tag
	for rows.Next() {
		t, err := scanTag(r.s.cipher, rows)
		if err != nil {
			return nil, err
		}
		out = append(out, *t)
	}
	return out, rows.Err()
}

// Delete removes a tag (cascading to its session).
func (r *TagRepo) Delete(ctx context.Context, id int64) error {
	r.s.wmu.Lock()
	defer r.s.wmu.Unlock()
	res, err := r.s.db.ExecContext(ctx, `DELETE FROM tags WHERE id = ?`, id)
	if err != nil {
		return fmt.Errorf("deleting tag: %w", err)
	}
	if n, _ := res.RowsAffected(); n == 0 {
		return ErrNotFound
	}
	return nil
}

// scanner is satisfied by *sql.Row and *sql.Rows.
type scanner interface {
	Scan(dest ...any) error
}

func scanTag(cipher cipherDecryptor, sc scanner) (*Tag, error) {
	var (
		t                    Tag
		encKey               string
		payloadConfig, sched sql.NullString
		enabled              int
	)
	err := sc.Scan(&t.ID, &t.DevEUI, &t.JoinEUI, &encKey, &t.Class, &t.Region,
		&t.SubBand, &t.DefaultDR, &t.FPort, &t.PayloadType, &payloadConfig, &sched,
		&enabled, &t.CreatedAt, &t.UpdatedAt)
	if errors.Is(err, sql.ErrNoRows) {
		return nil, ErrNotFound
	}
	if err != nil {
		return nil, fmt.Errorf("scanning tag: %w", err)
	}
	appKey, err := cipher.Decrypt(encKey, tagAAD(t.DevEUI, "app_key"))
	if err != nil {
		return nil, fmt.Errorf("decrypting app_key: %w", err)
	}
	t.AppKey = appKey
	t.PayloadConfig = payloadConfig.String
	t.Schedule = sched.String
	t.Enabled = enabled != 0
	return &t, nil
}

// cipherDecryptor is the subset of *secret.Cipher used while scanning.
type cipherDecryptor interface {
	Decrypt(stored string, aad []byte) (string, error)
}

func nullable(s string) any {
	if s == "" {
		return nil
	}
	return s
}

func boolToInt(b bool) int {
	if b {
		return 1
	}
	return 0
}

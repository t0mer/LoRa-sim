// Package store provides thin, hand-written repository wrappers over the
// SQLite database. Writes are serialized through a single mutex so the
// simulator's many goroutines never trip SQLite's single-writer constraint.
package store

import (
	"database/sql"
	"errors"
	"sync"

	"github.com/t0mer/cylon/internal/secret"
)

// ErrNotFound is returned when a requested row does not exist.
var ErrNotFound = errors.New("store: not found")

// Store holds the database handle, serializes writes, and protects sensitive
// columns at rest via the configured cipher.
type Store struct {
	db     *sql.DB
	cipher *secret.Cipher
	wmu    sync.Mutex // serializes writes
	clock  func() string
}

// New returns a Store wrapping the given database handle. The cipher protects
// sensitive columns (AppKey, session keys); pass secret.NewInsecureDisabled for
// dev/CI without encryption.
func New(db *sql.DB, cipher *secret.Cipher) *Store {
	return &Store{db: db, cipher: cipher, clock: nowUTC}
}

// DB exposes the underlying handle for read-only callers.
func (s *Store) DB() *sql.DB { return s.db }

// Gateway returns the gateway repository.
func (s *Store) Gateway() *GatewayRepo { return &GatewayRepo{s: s} }

// Tags returns the tag repository.
func (s *Store) Tags() *TagRepo { return &TagRepo{s: s} }

// Sessions returns the session repository.
func (s *Store) Sessions() *SessionRepo { return &SessionRepo{s: s} }

// Events returns the traffic-event repository.
func (s *Store) Events() *EventRepo { return &EventRepo{s: s} }

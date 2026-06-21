// Package store provides thin, hand-written repository wrappers over the
// SQLite database. Writes are serialized through a single mutex so the
// simulator's many goroutines never trip SQLite's single-writer constraint.
package store

import (
	"database/sql"
	"errors"
	"sync"
)

// ErrNotFound is returned when a requested row does not exist.
var ErrNotFound = errors.New("store: not found")

// Store holds the database handle and serializes writes.
type Store struct {
	db    *sql.DB
	wmu   sync.Mutex // serializes writes
	clock func() string
}

// New returns a Store wrapping the given database handle.
func New(db *sql.DB) *Store {
	return &Store{db: db, clock: nowUTC}
}

// DB exposes the underlying handle for read-only callers.
func (s *Store) DB() *sql.DB { return s.db }

// Gateway returns the gateway repository.
func (s *Store) Gateway() *GatewayRepo { return &GatewayRepo{s: s} }

// Package db opens and migrates cylon's SQLite database.
//
// It uses the pure-Go modernc.org/sqlite driver (CGO-free) so the binary
// cross-compiles cleanly for every release target.
package db

import (
	"database/sql"
	"fmt"
	"os"
	"path/filepath"

	_ "modernc.org/sqlite"
)

// Open opens (creating if needed) the SQLite database at path with WAL journal
// mode, a busy timeout, and foreign keys enabled. The parent directory is
// created if it does not exist. Use ":memory:" for an in-memory database.
func Open(path string) (*sql.DB, error) {
	if path != ":memory:" && path != "" {
		if dir := filepath.Dir(path); dir != "" && dir != "." {
			if err := os.MkdirAll(dir, 0o750); err != nil {
				return nil, fmt.Errorf("creating db dir %q: %w", dir, err)
			}
		}
	}

	dsn := fmt.Sprintf("file:%s?_pragma=journal_mode(WAL)&_pragma=busy_timeout(5000)&_pragma=foreign_keys(ON)",
		path)

	database, err := sql.Open("sqlite", dsn)
	if err != nil {
		return nil, fmt.Errorf("opening sqlite %q: %w", path, err)
	}
	if err := database.Ping(); err != nil {
		database.Close()
		return nil, fmt.Errorf("pinging sqlite %q: %w", path, err)
	}
	return database, nil
}

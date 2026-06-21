package db

import (
	"database/sql"
	"embed"
	"fmt"

	"github.com/pressly/goose/v3"
)

//go:embed migrations/*.sql
var migrationsFS embed.FS

const migrationsDir = "migrations"

// Migrate runs embedded goose migrations against the database. Supported
// commands: "up" (default), "down", "status".
func Migrate(database *sql.DB, command string) error {
	goose.SetBaseFS(migrationsFS)
	if err := goose.SetDialect("sqlite3"); err != nil {
		return fmt.Errorf("setting goose dialect: %w", err)
	}

	switch command {
	case "", "up":
		return goose.Up(database, migrationsDir)
	case "down":
		return goose.Down(database, migrationsDir)
	case "status":
		return goose.Status(database, migrationsDir)
	default:
		return fmt.Errorf("unknown migrate command %q (want up|down|status)", command)
	}
}

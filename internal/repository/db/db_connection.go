package db

import (
	"database/sql"
	"fmt"

	_ "modernc.org/sqlite"
)

// InitDB opens/creates a SQLite DB file and ensures tables exist.
func InitDB(path string) (*sql.DB, error) {
	db, err := sql.Open(sqliteDriverName, path)
	if err != nil {
		return nil, fmt.Errorf("open sqlite at %q: %w", path, err)
	}

	// Conservative pool settings for SQLite
	db.SetMaxOpenConns(1) // SQLite is not great with many writers
	db.SetMaxIdleConns(1)

	// Pragmas to improve reliability
	if _, err := db.Exec("PRAGMA journal_mode = WAL;"); err != nil {
		_ = db.Close()
		return nil, fmt.Errorf("set PRAGMA journal_mode=WAL: %w", err)
	}
	if _, err := db.Exec("PRAGMA foreign_keys = ON;"); err != nil {
		_ = db.Close()
		return nil, fmt.Errorf("set PRAGMA foreign_keys=ON: %w", err)
	}
	if _, err := db.Exec("PRAGMA busy_timeout = 5000;"); err != nil {
		_ = db.Close()
		return nil, fmt.Errorf("set PRAGMA busy_timeout=5000: %w", err)
	}

	if err := ensureSchema(db); err != nil {
		_ = db.Close()
		return nil, err
	}

	// Fail fast if the DB cannot be reached
	if err := db.Ping(); err != nil {
		_ = db.Close()
		return nil, fmt.Errorf("ping sqlite: %w", err)
	}

	return db, nil
}

// ... existing code ...

const sqliteDriverName = "sqlite"

const schemaFurnaceState = `
CREATE TABLE IF NOT EXISTS furnace_state (
    id INTEGER PRIMARY KEY CHECK (id = 1),
    mode TEXT NOT NULL,
    temp_c REAL NOT NULL,
    target_c REAL,
    remaining_s INTEGER,
    errors TEXT,
    running BOOLEAN NOT NULL,
    updated_at TIMESTAMP NOT NULL
);
`

const schemaFurnaceEvents = `
CREATE TABLE IF NOT EXISTS furnace_events (
    id TEXT PRIMARY KEY,
    occurred_at TIMESTAMP NOT NULL,
    type TEXT NOT NULL,
    message TEXT NOT NULL,
    meta TEXT
);
`

const schemaUsers = `
CREATE TABLE IF NOT EXISTS users (
    id INTEGER PRIMARY KEY AUTOINCREMENT,
    username TEXT UNIQUE NOT NULL,
    password_hash TEXT NOT NULL
);
`

func ensureSchema(db *sql.DB) error {
	tx, err := db.Begin()
	if err != nil {
		return fmt.Errorf("begin schema transaction: %w", err)
	}
	defer func() {
		// In case of panic, rollback to avoid leaving an open transaction
		_ = tx.Rollback()
	}()

	for i, stmt := range []string{
		schemaFurnaceState,
		schemaFurnaceEvents,
		schemaUsers,
	} {
		if _, err := tx.Exec(stmt); err != nil {
			return fmt.Errorf("apply schema statement %d: %w", i+1, err)
		}
	}

	if err := tx.Commit(); err != nil {
		return fmt.Errorf("commit schema transaction: %w", err)
	}
	return nil
}

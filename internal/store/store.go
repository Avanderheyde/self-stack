package store

import (
	"database/sql"
	"fmt"

	_ "modernc.org/sqlite"
)

type Store struct {
	db *sql.DB
}

func Open(path string) (*Store, error) {
	db, err := sql.Open("sqlite", path)
	if err != nil {
		return nil, fmt.Errorf("open db: %w", err)
	}
	// Enable WAL mode for concurrent access safety
	if _, err := db.Exec("PRAGMA journal_mode=WAL"); err != nil {
		db.Close()
		return nil, fmt.Errorf("enable WAL: %w", err)
	}
	s := &Store{db: db}
	if err := s.migrate(); err != nil {
		db.Close()
		return nil, fmt.Errorf("migrate: %w", err)
	}
	return s, nil
}

func (s *Store) Close() error { return s.db.Close() }

func (s *Store) migrate() error {
	schema := `
	CREATE TABLE IF NOT EXISTS apps (
		name         TEXT PRIMARY KEY,
		display_name TEXT NOT NULL,
		description  TEXT,
		repo_url     TEXT,
		version      TEXT,
		host_port    INTEGER NOT NULL,
		status       TEXT NOT NULL DEFAULT 'stopped',
		source_type  TEXT NOT NULL DEFAULT 'registry',
		installed_at DATETIME DEFAULT CURRENT_TIMESTAMP
	);
	CREATE TABLE IF NOT EXISTS devices (
		id         TEXT PRIMARY KEY,
		name       TEXT NOT NULL,
		token_hash TEXT NOT NULL,
		trusted    INTEGER NOT NULL DEFAULT 0,
		created_at DATETIME DEFAULT CURRENT_TIMESTAMP,
		last_seen  DATETIME
	);
	CREATE TABLE IF NOT EXISTS app_config (
		app_name TEXT NOT NULL,
		key      TEXT NOT NULL,
		value    TEXT,
		PRIMARY KEY (app_name, key),
		FOREIGN KEY (app_name) REFERENCES apps(name)
	);
	CREATE TABLE IF NOT EXISTS port_allocations (
		port     INTEGER PRIMARY KEY,
		app_name TEXT NOT NULL UNIQUE,
		FOREIGN KEY (app_name) REFERENCES apps(name)
	);`
	if _, err := s.db.Exec(schema); err != nil {
		return err
	}
	return s.ensureColumn("apps", "source_type", "TEXT NOT NULL DEFAULT 'registry'")
}

func (s *Store) ensureColumn(table, column, definition string) error {
	rows, err := s.db.Query("PRAGMA table_info(" + table + ")")
	if err != nil {
		return fmt.Errorf("inspect %s schema: %w", table, err)
	}
	defer rows.Close()

	for rows.Next() {
		var cid int
		var name, typ string
		var notNull int
		var defaultValue any
		var pk int
		if err := rows.Scan(&cid, &name, &typ, &notNull, &defaultValue, &pk); err != nil {
			return fmt.Errorf("scan %s schema: %w", table, err)
		}
		if name == column {
			return nil
		}
	}
	if err := rows.Err(); err != nil {
		return fmt.Errorf("read %s schema: %w", table, err)
	}
	if _, err := s.db.Exec(fmt.Sprintf("ALTER TABLE %s ADD COLUMN %s %s", table, column, definition)); err != nil {
		return fmt.Errorf("add %s.%s: %w", table, column, err)
	}
	return nil
}

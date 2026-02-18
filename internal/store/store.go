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
		repo_url     TEXT NOT NULL,
		version      TEXT,
		host_port    INTEGER NOT NULL,
		status       TEXT NOT NULL DEFAULT 'stopped',
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
	_, err := s.db.Exec(schema)
	return err
}

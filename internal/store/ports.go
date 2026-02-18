package store

import (
	"database/sql"
	"fmt"

	"github.com/selfstack/selfstack/internal/config"
)

func (s *Store) AllocatePort(appName string) (int, error) {
	// Find the lowest available port starting at PortRangeStart.
	// This fills gaps left by released ports.
	var port int
	err := s.db.QueryRow(`
		SELECT MIN(candidate) FROM (
			SELECT ? AS candidate
			UNION ALL
			SELECT port + 1 FROM port_allocations WHERE port >= ?
		) WHERE candidate NOT IN (SELECT port FROM port_allocations)`,
		config.PortRangeStart, config.PortRangeStart,
	).Scan(&port)
	if err != nil {
		return 0, fmt.Errorf("find available port: %w", err)
	}

	_, err = s.db.Exec(
		`INSERT INTO port_allocations (port, app_name) VALUES (?, ?)`,
		port, appName,
	)
	if err != nil {
		return 0, fmt.Errorf("allocate port: %w", err)
	}
	return port, nil
}

func (s *Store) ReleasePort(appName string) error {
	_, err := s.db.Exec(`DELETE FROM port_allocations WHERE app_name = ?`, appName)
	if err != nil {
		return fmt.Errorf("release port: %w", err)
	}
	return nil
}

func (s *Store) GetPort(appName string) (int, error) {
	var port int
	err := s.db.QueryRow(
		`SELECT port FROM port_allocations WHERE app_name = ?`, appName,
	).Scan(&port)
	if err == sql.ErrNoRows {
		return 0, fmt.Errorf("no port allocated for %q", appName)
	}
	if err != nil {
		return 0, fmt.Errorf("get port: %w", err)
	}
	return port, nil
}

package store

import (
	"database/sql"
	"fmt"
)

type App struct {
	Name        string
	DisplayName string
	Description string
	RepoURL     string
	Version     string
	HostPort    int
	Status      string
	SourceType  string // "registry", "deploy", or "local"
}

func (s *Store) InsertApp(a App) error {
	sourceType := a.SourceType
	if sourceType == "" {
		sourceType = "registry"
	}
	_, err := s.db.Exec(
		`INSERT INTO apps (name, display_name, description, repo_url, version, host_port, status, source_type)
		 VALUES (?, ?, ?, ?, ?, ?, ?, ?)`,
		a.Name, a.DisplayName, a.Description, a.RepoURL, a.Version, a.HostPort, a.Status, sourceType,
	)
	if err != nil {
		return fmt.Errorf("insert app: %w", err)
	}
	return nil
}

func (s *Store) GetApp(name string) (App, error) {
	var a App
	var repoURL sql.NullString
	err := s.db.QueryRow(
		`SELECT name, display_name, description, repo_url, version, host_port, status, source_type
		 FROM apps WHERE name = ?`, name,
	).Scan(&a.Name, &a.DisplayName, &a.Description, &repoURL, &a.Version, &a.HostPort, &a.Status, &a.SourceType)
	a.RepoURL = repoURL.String
	if err == sql.ErrNoRows {
		return App{}, fmt.Errorf("app %q not found", name)
	}
	if err != nil {
		return App{}, fmt.Errorf("get app: %w", err)
	}
	return a, nil
}

func (s *Store) ListApps() ([]App, error) {
	rows, err := s.db.Query(
		`SELECT name, display_name, description, repo_url, version, host_port, status, source_type
		 FROM apps ORDER BY name`,
	)
	if err != nil {
		return nil, fmt.Errorf("list apps: %w", err)
	}
	defer rows.Close()

	var apps []App
	for rows.Next() {
		var a App
		var repoURL sql.NullString
		if err := rows.Scan(&a.Name, &a.DisplayName, &a.Description, &repoURL, &a.Version, &a.HostPort, &a.Status, &a.SourceType); err != nil {
			return nil, fmt.Errorf("scan app: %w", err)
		}
		a.RepoURL = repoURL.String
		apps = append(apps, a)
	}
	return apps, rows.Err()
}

func (s *Store) DeleteApp(name string) error {
	_, err := s.db.Exec(`DELETE FROM apps WHERE name = ?`, name)
	if err != nil {
		return fmt.Errorf("delete app: %w", err)
	}
	return nil
}

func (s *Store) UpdateAppMeta(name, displayName, description, version string) error {
	res, err := s.db.Exec(
		`UPDATE apps SET display_name = ?, description = ?, version = ? WHERE name = ?`,
		displayName, description, version, name,
	)
	if err != nil {
		return fmt.Errorf("update app meta: %w", err)
	}
	n, err := res.RowsAffected()
	if err != nil {
		return fmt.Errorf("rows affected: %w", err)
	}
	if n == 0 {
		return fmt.Errorf("app %q not found", name)
	}
	return nil
}

func (s *Store) UpdateAppStatus(name, status string) error {
	res, err := s.db.Exec(`UPDATE apps SET status = ? WHERE name = ?`, status, name)
	if err != nil {
		return fmt.Errorf("update app status: %w", err)
	}
	n, err := res.RowsAffected()
	if err != nil {
		return fmt.Errorf("rows affected: %w", err)
	}
	if n == 0 {
		return fmt.Errorf("app %q not found", name)
	}
	return nil
}

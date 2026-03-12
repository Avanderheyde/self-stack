package app

import (
	"context"
	"fmt"
	"io"
	"os"
	"os/exec"
	"path/filepath"

	"github.com/selfstack/selfstack/internal/config"
	"github.com/selfstack/selfstack/internal/container"
	"github.com/selfstack/selfstack/internal/manifest"
	"github.com/selfstack/selfstack/internal/portless"
	"github.com/selfstack/selfstack/internal/registry"
	"github.com/selfstack/selfstack/internal/store"
)

// ProgressFunc is called with a human-readable step description during long operations.
type ProgressFunc func(step string)

func report(fn ProgressFunc, step string) {
	if fn != nil {
		fn(step)
	}
}

type Service struct {
	store    *store.Store
	registry *registry.Client
	mgr      *container.Manager
}

func NewService(s *store.Store, r *registry.Client) *Service {
	return &Service{store: s, registry: r, mgr: container.NewManager()}
}

func (s *Service) Install(ctx context.Context, appName string, onProgress ProgressFunc) error {
	report(onProgress, "Fetching app info")
	entry, err := s.registry.Lookup(appName)
	if err != nil {
		return err
	}
	appDir := filepath.Join(config.AppsDir(), appName)

	// Clean up orphaned directory from a previous failed install
	if _, err := s.store.GetApp(appName); err != nil {
		os.RemoveAll(appDir)
	}

	if err := os.MkdirAll(filepath.Dir(appDir), 0755); err != nil {
		return err
	}
	report(onProgress, "Cloning repository")
	cmd := exec.CommandContext(ctx, "git", "clone", entry.Repo, appDir)
	if out, err := cmd.CombinedOutput(); err != nil {
		return fmt.Errorf("git clone: %s: %w", string(out), err)
	}

	// On failure after clone, clean up the directory and port
	cleanup := func() {
		os.RemoveAll(appDir)
		s.store.ReleasePort(appName)
	}

	m, err := manifest.ParseFile(filepath.Join(appDir, "selfstack.yml"))
	if err != nil {
		os.RemoveAll(appDir)
		return fmt.Errorf("parse manifest: %w", err)
	}
	port, err := s.store.AllocatePort(appName)
	if err != nil {
		os.RemoveAll(appDir)
		return fmt.Errorf("allocate port: %w", err)
	}
	report(onProgress, "Building container")
	if err := s.mgr.Build(ctx, appDir, m.Runtime.Entry, appName); err != nil {
		cleanup()
		return err
	}
	envVars := make(map[string]string)
	for _, c := range m.Config {
		envVars[c.Key] = c.Default
	}
	report(onProgress, "Starting app")
	if err := s.mgr.Up(ctx, appDir, m.Runtime.Entry, appName, m.Expose.Port, port, envVars); err != nil {
		cleanup()
		return err
	}
	if m.Expose.Health != "" {
		report(onProgress, "Waiting for health check")
		if err := s.mgr.HealthCheck(ctx, port, m.Expose.Health); err != nil {
			cleanup()
			return fmt.Errorf("health check: %w", err)
		}
	}
	if err := s.store.InsertApp(store.App{
		Name: appName, DisplayName: m.DisplayName, Description: m.Description,
		RepoURL: entry.Repo, Version: m.Version, HostPort: port, Status: "running",
	}); err != nil {
		return err
	}
	portless.Alias(appName, port)
	return nil
}

func (s *Service) Start(ctx context.Context, appName string) error {
	app, err := s.store.GetApp(appName)
	if err != nil {
		return err
	}
	appDir := filepath.Join(config.AppsDir(), appName)
	m, err := manifest.ParseFile(filepath.Join(appDir, "selfstack.yml"))
	if err != nil {
		return err
	}
	if err := s.mgr.Start(ctx, appDir, m.Runtime.Entry, appName); err != nil {
		return err
	}
	portless.Alias(appName, app.HostPort)
	return s.store.UpdateAppStatus(app.Name, "running")
}

func (s *Service) Stop(ctx context.Context, appName string) error {
	app, err := s.store.GetApp(appName)
	if err != nil {
		return err
	}
	appDir := filepath.Join(config.AppsDir(), appName)
	m, err := manifest.ParseFile(filepath.Join(appDir, "selfstack.yml"))
	if err != nil {
		return err
	}
	if err := s.mgr.Stop(ctx, appDir, m.Runtime.Entry, appName); err != nil {
		return err
	}
	portless.Unalias(appName)
	return s.store.UpdateAppStatus(app.Name, "stopped")
}

func (s *Service) Remove(ctx context.Context, appName string, onProgress ProgressFunc) error {
	appDir := filepath.Join(config.AppsDir(), appName)
	m, _ := manifest.ParseFile(filepath.Join(appDir, "selfstack.yml"))
	if m != nil {
		report(onProgress, "Stopping container")
		s.mgr.Down(ctx, appDir, m.Runtime.Entry, appName)
	}
	report(onProgress, "Cleaning up")
	portless.Unalias(appName)
	s.store.DeleteApp(appName)
	s.store.ReleasePort(appName)
	os.RemoveAll(appDir)
	return nil
}

func (s *Service) Update(ctx context.Context, appName string, onProgress ProgressFunc) error {
	app, err := s.store.GetApp(appName)
	if err != nil {
		return err
	}
	appDir := filepath.Join(config.AppsDir(), appName)

	m, err := manifest.ParseFile(filepath.Join(appDir, "selfstack.yml"))
	if err != nil {
		return fmt.Errorf("parse manifest: %w", err)
	}

	// Tear down if running (need full rebuild)
	if app.Status == "running" {
		report(onProgress, "Stopping app")
		if err := s.mgr.Down(ctx, appDir, m.Runtime.Entry, appName); err != nil {
			return fmt.Errorf("stop for update: %w", err)
		}
		s.store.UpdateAppStatus(appName, "stopped")
	}

	// Pull latest code
	report(onProgress, "Pulling updates")
	cmd := exec.CommandContext(ctx, "git", "pull")
	cmd.Dir = appDir
	if out, err := cmd.CombinedOutput(); err != nil {
		return fmt.Errorf("git pull: %s: %w", string(out), err)
	}

	// Re-parse manifest (may have changed)
	m, err = manifest.ParseFile(filepath.Join(appDir, "selfstack.yml"))
	if err != nil {
		return fmt.Errorf("parse updated manifest: %w", err)
	}

	// Rebuild and start
	report(onProgress, "Building container")
	if err := s.mgr.Build(ctx, appDir, m.Runtime.Entry, appName); err != nil {
		return fmt.Errorf("build after update: %w", err)
	}
	envVars := make(map[string]string)
	for _, c := range m.Config {
		envVars[c.Key] = c.Default
	}
	report(onProgress, "Starting app")
	if err := s.mgr.Up(ctx, appDir, m.Runtime.Entry, appName, m.Expose.Port, app.HostPort, envVars); err != nil {
		return fmt.Errorf("start after update: %w", err)
	}
	if m.Expose.Health != "" {
		report(onProgress, "Waiting for health check")
		if err := s.mgr.HealthCheck(ctx, app.HostPort, m.Expose.Health); err != nil {
			return fmt.Errorf("health check after update: %w", err)
		}
	}

	// Update metadata from new manifest
	if err := s.store.UpdateAppMeta(appName, m.DisplayName, m.Description, m.Version); err != nil {
		return err
	}
	portless.Alias(appName, app.HostPort)
	return s.store.UpdateAppStatus(appName, "running")
}

func (s *Service) UpdatePort(appName string, port int) error {
	app, err := s.store.GetApp(appName)
	if err != nil {
		return err
	}
	if app.Status != "stopped" {
		return fmt.Errorf("app must be stopped to change port")
	}
	return s.store.UpdatePort(appName, port)
}

func (s *Service) Logs(ctx context.Context, appName string, w io.Writer) error {
	_, err := s.store.GetApp(appName)
	if err != nil {
		return err
	}
	appDir := filepath.Join(config.AppsDir(), appName)
	m, err := manifest.ParseFile(filepath.Join(appDir, "selfstack.yml"))
	if err != nil {
		return err
	}
	return s.mgr.Logs(ctx, appDir, m.Runtime.Entry, appName, w)
}

func (s *Service) List() ([]store.App, error) { return s.store.ListApps() }
func (s *Service) Get(appName string) (store.App, error) { return s.store.GetApp(appName) }

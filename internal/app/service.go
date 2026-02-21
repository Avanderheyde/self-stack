package app

import (
	"context"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"

	"github.com/selfstack/selfstack/internal/config"
	"github.com/selfstack/selfstack/internal/container"
	"github.com/selfstack/selfstack/internal/manifest"
	"github.com/selfstack/selfstack/internal/registry"
	"github.com/selfstack/selfstack/internal/store"
)

type Service struct {
	store    *store.Store
	registry *registry.Client
	mgr      *container.Manager
}

func NewService(s *store.Store, r *registry.Client) *Service {
	return &Service{store: s, registry: r, mgr: container.NewManager()}
}

func (s *Service) Install(ctx context.Context, appName string) error {
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
	if err := s.mgr.Build(ctx, appDir, m.Runtime.Entry); err != nil {
		cleanup()
		return err
	}
	envVars := make(map[string]string)
	for _, c := range m.Config {
		envVars[c.Key] = c.Default
	}
	if err := s.mgr.Up(ctx, appDir, m.Runtime.Entry, appName, m.Expose.Port, port, envVars); err != nil {
		cleanup()
		return err
	}
	if err := s.store.InsertApp(store.App{
		Name: appName, DisplayName: m.DisplayName, Description: m.Description,
		RepoURL: entry.Repo, Version: m.Version, HostPort: port, Status: "running",
	}); err != nil {
		return err
	}
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
	return s.store.UpdateAppStatus(app.Name, "stopped")
}

func (s *Service) Remove(ctx context.Context, appName string) error {
	appDir := filepath.Join(config.AppsDir(), appName)
	m, _ := manifest.ParseFile(filepath.Join(appDir, "selfstack.yml"))
	if m != nil {
		s.mgr.Down(ctx, appDir, m.Runtime.Entry, appName)
	}
	s.store.DeleteApp(appName)
	s.store.ReleasePort(appName)
	os.RemoveAll(appDir)
	return nil
}

func (s *Service) List() ([]store.App, error) { return s.store.ListApps() }
func (s *Service) Get(appName string) (store.App, error) { return s.store.GetApp(appName) }

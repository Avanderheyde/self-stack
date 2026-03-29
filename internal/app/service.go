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
	"github.com/selfstack/selfstack/internal/detect"
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

// Deploy takes a local project directory, auto-detects the project type,
// generates necessary Docker/compose/manifest files, and deploys the app.
// This is the "selfstack deploy" flow for vibe-coded apps.
func (s *Service) Deploy(ctx context.Context, appName, projectDir string, envVars map[string]string, onProgress ProgressFunc) error {
	report(onProgress, "Detecting project type")
	pt := detect.DetectProjectType(projectDir)
	if pt == detect.TypeUnknown {
		return fmt.Errorf("cannot detect project type in %s — add a Dockerfile or docker-compose.yml", projectDir)
	}

	appDir := filepath.Join(config.AppsDir(), appName)

	// Check if this is a redeploy
	existing, err := s.store.GetApp(appName)
	isRedeploy := err == nil && existing.Name != ""

	// Copy project to app directory (or overwrite for redeploy)
	if err := copyDir(projectDir, appDir); err != nil {
		return fmt.Errorf("copy project: %w", err)
	}

	cleanup := func() {
		if !isRedeploy {
			os.RemoveAll(appDir)
			s.store.ReleasePort(appName)
		}
	}

	// Generate Dockerfile if needed
	if pt != detect.TypeDockerCompose && pt != detect.TypeDockerfile {
		report(onProgress, "Generating Dockerfile")
		content, err := detect.GenerateDockerfile(appDir, pt)
		if err != nil {
			cleanup()
			return fmt.Errorf("generate dockerfile: %w", err)
		}
		if err := os.WriteFile(filepath.Join(appDir, "Dockerfile"), []byte(content), 0644); err != nil {
			cleanup()
			return fmt.Errorf("write dockerfile: %w", err)
		}
	}

	containerPort := detect.DefaultPort(pt)

	// Generate docker-compose.yml if needed (when we have a Dockerfile but no compose)
	if pt != detect.TypeDockerCompose {
		composeContent := detect.GenerateComposeFile(appName, containerPort)
		if err := os.WriteFile(filepath.Join(appDir, "docker-compose.yml"), []byte(composeContent), 0644); err != nil {
			cleanup()
			return fmt.Errorf("write docker-compose.yml: %w", err)
		}
	}

	// Generate selfstack.yml manifest
	manifestContent := detect.GenerateManifestYAML(appName, containerPort)
	if err := os.WriteFile(filepath.Join(appDir, "selfstack.yml"), []byte(manifestContent), 0644); err != nil {
		cleanup()
		return fmt.Errorf("write manifest: %w", err)
	}

	m, err := manifest.ParseFile(filepath.Join(appDir, "selfstack.yml"))
	if err != nil {
		cleanup()
		return fmt.Errorf("parse generated manifest: %w", err)
	}

	// For redeploy: stop old container (preserve volumes)
	if isRedeploy {
		report(onProgress, "Stopping previous version")
		s.mgr.Stop(ctx, appDir, m.Runtime.Entry, appName)
	}

	// Allocate port (reuse existing for redeploy)
	hostPort := existing.HostPort
	if !isRedeploy {
		hostPort, err = s.store.AllocatePort(appName)
		if err != nil {
			cleanup()
			return fmt.Errorf("allocate port: %w", err)
		}
	}

	// Merge env vars: manifest defaults + user-provided
	mergedEnv := make(map[string]string)
	for _, c := range m.Config {
		mergedEnv[c.Key] = c.Default
	}
	for k, v := range envVars {
		mergedEnv[k] = v
	}

	report(onProgress, "Building container")
	if err := s.mgr.Build(ctx, appDir, m.Runtime.Entry, appName); err != nil {
		cleanup()
		return fmt.Errorf("build: %w", err)
	}

	report(onProgress, "Starting app")
	if err := s.mgr.Up(ctx, appDir, m.Runtime.Entry, appName, m.Expose.Port, hostPort, mergedEnv); err != nil {
		cleanup()
		return fmt.Errorf("start: %w", err)
	}

	if m.Expose.Health != "" {
		report(onProgress, "Waiting for health check")
		if err := s.mgr.HealthCheck(ctx, hostPort, m.Expose.Health); err != nil {
			// Leave container running for debugging (per plan)
			return fmt.Errorf("health check failed (container still running for debugging): %w", err)
		}
	}

	if isRedeploy {
		if err := s.store.UpdateAppStatus(appName, "running"); err != nil {
			return err
		}
	} else {
		if err := s.store.InsertApp(store.App{
			Name: appName, DisplayName: appName, Description: "Deployed via selfstack deploy",
			HostPort: hostPort, Status: "running", SourceType: "deploy",
		}); err != nil {
			return err
		}
	}

	portless.Alias(appName, hostPort)
	return nil
}

// copyDir copies the contents of src into dst, creating dst if needed.
// Skips common build artifact directories.
func copyDir(src, dst string) error {
	if err := os.MkdirAll(dst, 0755); err != nil {
		return err
	}
	cmd := exec.Command("rsync", "-a",
		"--exclude", "node_modules",
		"--exclude", ".git",
		"--exclude", "__pycache__",
		"--exclude", ".venv",
		"--exclude", "venv",
		"--exclude", ".next",
		"--exclude", "dist",
		"--exclude", "build",
		"--exclude", "target",
		"--exclude", ".DS_Store",
		src+"/", dst+"/",
	)
	out, err := cmd.CombinedOutput()
	if err != nil {
		return fmt.Errorf("rsync: %s: %w", string(out), err)
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

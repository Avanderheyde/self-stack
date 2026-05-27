package app

import (
	"context"
	"fmt"
	"io"
	"os"
	"os/exec"
	"path/filepath"
	"regexp"
	"strings"
	"time"

	"github.com/selfstack/selfstack/internal/config"
	"github.com/selfstack/selfstack/internal/container"
	"github.com/selfstack/selfstack/internal/detect"
	"github.com/selfstack/selfstack/internal/manifest"
	"github.com/selfstack/selfstack/internal/portless"
	"github.com/selfstack/selfstack/internal/registry"
	"github.com/selfstack/selfstack/internal/store"
)

// sourceTypeLocal marks an app as installed from a local source directory.
// Matches the existing "registry" / "deploy" values defined in internal/store
// — the source_type column is the canonical signal for how an app was
// installed, so Update/Remove branches and any future install-type-aware
// logic should switch on it instead of inspecting RepoURL.
const sourceTypeLocal = "local"

// localPathRootsEnv lets the daemon admin override the allowlist of root
// directories that local installs may source from. Colon-separated, same
// shape as $PATH. Empty / unset means "user's home directory only."
const localPathRootsEnv = "SELFSTACK_LOCAL_INSTALL_ROOTS"

// appNamePattern is the strict allowlist for appName values that get
// interpolated into filesystem paths and docker arguments. Mirrors the
// shape of a docker compose project identifier so we never construct a
// path or container reference that breaks tooling downstream.
var appNamePattern = regexp.MustCompile(`^[a-z0-9][a-z0-9_-]{0,30}$`)

// Package-level indirections so tests can simulate POSIX-rare failures like
// filepath.Abs returning an error (which normally requires os.Getwd to fail
// on a deleted cwd — unreliable on macOS).
var (
	absFn      = filepath.Abs
	mkdirAllFn = os.MkdirAll
)

func isLocalApp(a store.App) bool {
	return a.SourceType == sourceTypeLocal
}

// validateAppName rejects appNames that would be unsafe to interpolate
// into filesystem paths or docker arguments. Same rule as docker compose
// project names: lowercase alphanumeric + hyphen/underscore, must start
// with alphanumeric, max 31 chars.
func validateAppName(name string) error {
	if !appNamePattern.MatchString(name) {
		return fmt.Errorf("invalid app name %q: must match %s", name, appNamePattern.String())
	}
	return nil
}

// localInstallRoots returns the directories that local-source installs are
// allowed to source from. Resolves once per call (env vars can change
// between daemon invocations but not within one). All returned paths are
// symlink-resolved absolute paths.
func localInstallRoots() ([]string, error) {
	raw := os.Getenv(localPathRootsEnv)
	if raw == "" {
		home, err := os.UserHomeDir()
		if err != nil {
			return nil, fmt.Errorf("resolve $HOME for default local-install root: %w", err)
		}
		raw = home
	}
	var roots []string
	for _, p := range filepath.SplitList(raw) {
		if p == "" {
			continue
		}
		resolved, err := filepath.EvalSymlinks(p)
		if err != nil {
			// Skip roots that don't exist; an admin's env var pointing
			// at a not-yet-mounted path shouldn't lock out everything else.
			continue
		}
		roots = append(roots, resolved)
	}
	if len(roots) == 0 {
		return nil, fmt.Errorf("no usable local-install roots (set %s or have a valid $HOME)", localPathRootsEnv)
	}
	return roots, nil
}

// validateLocalPath rejects paths that escape the configured allowlist of
// root directories. Resolves symlinks before the prefix check so an
// attacker can't bypass the gate by creating a symlink inside an allowed
// root that points elsewhere. Returns the resolved absolute path on
// success — callers should use this resolved form going forward.
func validateLocalPath(label, path string) (string, error) {
	if path == "" {
		return "", fmt.Errorf("%s is empty", label)
	}
	resolved, err := filepath.EvalSymlinks(path)
	if err != nil {
		return "", fmt.Errorf("resolve %s %q: %w", label, path, err)
	}
	roots, err := localInstallRoots()
	if err != nil {
		return "", err
	}
	for _, root := range roots {
		// filepath.Rel handles cross-volume edge cases on Windows.
		// On POSIX a HasPrefix check would also work, but Rel + Clean
		// is robust against trailing slashes and `.` segments.
		rel, err := filepath.Rel(root, resolved)
		if err != nil {
			continue
		}
		if rel == "." || (!strings.HasPrefix(rel, "..") && !filepath.IsAbs(rel)) {
			return resolved, nil
		}
	}
	return "", fmt.Errorf("%s %q is outside allowed roots %v (configure via %s)", label, path, roots, localPathRootsEnv)
}

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

// InstallLocal installs an app from a local source directory. The directory
// must contain a selfstack.yml. Instead of cloning, we symlink the source into
// AppsDir so edits to the source flow through to the next rebuild. If
// seedDataPath is non-empty, its contents are copied into the named docker
// volume declared by the compose file before the container starts, used for
// migrating an existing on-disk database into a fresh selfstack-managed volume.
func (s *Service) InstallLocal(ctx context.Context, appName, localPath, seedDataPath string, onProgress ProgressFunc) error {
	report(onProgress, "Validating local path")
	// appName is interpolated into filesystem paths (appDir, volume name)
	// and docker arguments. A traversal-shaped or malformed value would
	// make the daemon symlink, delete, or mount outside the apps directory.
	if err := validateAppName(appName); err != nil {
		return err
	}
	// Refuse to reinstall over an existing row. Without this, a successful
	// Build/Up against a stale appDir + a stale port_allocations row would
	// reach InsertApp, fail with UNIQUE, then trigger cleanup() — which
	// releases the EXISTING app's port allocation and leaves an orphaned
	// app row with no port. The caller should `selfstack remove` first.
	if _, err := s.store.GetApp(appName); err == nil {
		return fmt.Errorf("app %q is already installed; run `selfstack remove %s` first", appName, appName)
	}
	absSrc, err := absFn(localPath)
	if err != nil {
		return fmt.Errorf("resolve local path: %w", err)
	}
	// localPath sandbox: must resolve to a path under the configured
	// allowlist roots ($HOME by default, override via SELFSTACK_LOCAL_INSTALL_ROOTS).
	// Without this, an unauthenticated API caller could symlink any host
	// directory into the apps tree and trigger a docker build against it.
	absSrc, err = validateLocalPath("local path", absSrc)
	if err != nil {
		return err
	}
	if seedDataPath != "" {
		// seedDataPath is bind-mounted read-only into a one-shot alpine
		// container that copies its contents into a docker volume the new
		// app will mount. Without sandboxing the caller could exfiltrate
		// any file the daemon user can read.
		resolvedSeed, err := validateLocalPath("seed-data path", seedDataPath)
		if err != nil {
			return err
		}
		seedDataPath = resolvedSeed
	}
	info, err := os.Stat(absSrc)
	if err != nil {
		return fmt.Errorf("stat local path: %w", err)
	}
	if !info.IsDir() {
		return fmt.Errorf("local path is not a directory: %s", absSrc)
	}
	manifestPath := filepath.Join(absSrc, "selfstack.yml")
	m, err := manifest.ParseFile(manifestPath)
	if err != nil {
		return fmt.Errorf("parse manifest at %s: %w", manifestPath, err)
	}
	if m.Name != appName {
		return fmt.Errorf("manifest name %q does not match app name %q", m.Name, appName)
	}
	// Runtime.Entry is a compose-file path passed to `docker compose -f`.
	// The manifest is user-controlled, so an attacker could set it to
	// "../../etc/passwd" to make docker read an arbitrary host file as a
	// compose definition. filepath.IsLocal rejects absolute paths and any
	// path that escapes its base via parent (`..`) components.
	if m.Runtime.Entry != "" && !filepath.IsLocal(m.Runtime.Entry) {
		return fmt.Errorf("manifest runtime.entry %q escapes the source directory; only relative paths inside selfstack.yml's directory are allowed", m.Runtime.Entry)
	}

	appDir := filepath.Join(config.AppsDir(), appName)

	// Clean up orphaned symlink from a previous failed install. We already
	// confirmed above that no DB row exists for this app, so removing a
	// leftover symlink at appDir is safe — its target is whatever the
	// previous failed install pointed at, not a live app.
	os.RemoveAll(appDir)
	if err := mkdirAllFn(filepath.Dir(appDir), 0755); err != nil {
		return err
	}

	report(onProgress, "Linking source directory")
	if err := os.Symlink(absSrc, appDir); err != nil {
		return fmt.Errorf("symlink %s -> %s: %w", absSrc, appDir, err)
	}

	// cleanup is the canonical rollback for every failure after symlink
	// creation. ReleasePort is safe to call even before AllocatePort
	// succeeds (it's a no-op when no row exists). The mgr.Down call is
	// bounded by a 30s deadline so a hung dockerd can't stall the
	// daemon's HTTP handler indefinitely. The seeded flag controls volume
	// cleanup — once seedDataVolume has run we must docker volume rm to
	// avoid leaking attacker-supplied data into future install attempts.
	seeded := false
	cleanup := func() {
		downCtx, downCancel := context.WithTimeout(context.Background(), 30*time.Second)
		defer downCancel()
		s.mgr.Down(downCtx, appDir, m.Runtime.Entry, appName)
		if seeded {
			volume := "selfstack-" + appName + "_data"
			rmCtx, rmCancel := context.WithTimeout(context.Background(), 10*time.Second)
			defer rmCancel()
			exec.CommandContext(rmCtx, "docker", "volume", "rm", "-f", volume).Run()
		}
		os.Remove(appDir)
		s.store.ReleasePort(appName)
	}

	port, err := s.store.AllocatePort(appName)
	if err != nil {
		cleanup()
		return fmt.Errorf("allocate port: %w", err)
	}

	if seedDataPath != "" {
		report(onProgress, "Seeding data volume")
		if err := s.seedDataVolume(ctx, appName, seedDataPath); err != nil {
			cleanup()
			return fmt.Errorf("seed data volume: %w", err)
		}
		seeded = true
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
		RepoURL: absSrc, Version: m.Version, HostPort: port, Status: "running",
		SourceType: sourceTypeLocal,
	}); err != nil {
		cleanup()
		return fmt.Errorf("insert app: %w", err)
	}
	portless.Alias(appName, port)
	return nil
}

// seedDataVolume copies the contents of srcDir into the docker named volume
// that compose creates for the app's `data` volume. The volume name follows
// compose's project-prefixed convention: <project>_<volume> where project is
// "selfstack-<appName>". We pre-create the volume and copy via a one-shot
// alpine container, which works on any docker host without needing the source
// to be reachable from inside compose.
//
// Callers must pass an already-validated absolute srcDir (see validateLocalPath
// in InstallLocal). The --mount key=value syntax is used instead of -v so a
// colon in a path can't be reinterpreted by docker as extra mount options.
//
// To narrow the TOCTOU window between path validation and bind-mount,
// we Lstat srcDir immediately before docker run and reject if it has
// become a symlink (which would let an attacker who controls the
// directory tree redirect the bind-mount after validation succeeded).
// This does not eliminate the race entirely — the kernel could still
// swap the inode between Lstat and the docker bind syscall — but it
// closes the trivially-exploitable window.
func (s *Service) seedDataVolume(ctx context.Context, appName, srcDir string) error {
	absSrc, err := absFn(srcDir)
	if err != nil {
		return fmt.Errorf("resolve seed path: %w", err)
	}
	fi, err := os.Lstat(absSrc)
	if err != nil {
		return fmt.Errorf("stat seed path: %w", err)
	}
	if fi.Mode()&os.ModeSymlink != 0 {
		return fmt.Errorf("seed path %q is a symlink; would bind-mount target outside the validated sandbox", absSrc)
	}
	if !fi.IsDir() {
		return fmt.Errorf("seed path %q is not a directory", absSrc)
	}
	volume := "selfstack-" + appName + "_data"
	createCmd := exec.CommandContext(ctx, "docker", "volume", "create", volume)
	if out, err := createCmd.CombinedOutput(); err != nil {
		return fmt.Errorf("docker volume create %s: %s: %w", volume, string(out), err)
	}
	copyCmd := exec.CommandContext(ctx, "docker", "run", "--rm",
		"--mount", fmt.Sprintf("type=volume,source=%s,target=/dest", volume),
		"--mount", fmt.Sprintf("type=bind,source=%s,target=/src,readonly", absSrc),
		"alpine", "sh", "-c", "cp -a /src/. /dest/")
	if out, err := copyCmd.CombinedOutput(); err != nil {
		return fmt.Errorf("seed copy: %s: %w", string(out), err)
	}
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

	// Record the app in the store BEFORE the health check so that, if the
	// check fails and we leave the container running for debugging, `selfstack
	// logs <app>` can still resolve the app to its compose file. Without this
	// a first-deploy with a slow startup looks like a ghost: container alive,
	// no DB row, no logs.
	if isRedeploy {
		if err := s.store.UpdateAppStatus(appName, "starting"); err != nil {
			return err
		}
	} else {
		if err := s.store.InsertApp(store.App{
			Name: appName, DisplayName: appName, Description: "Deployed via selfstack deploy",
			HostPort: hostPort, Status: "starting", SourceType: "deploy",
		}); err != nil {
			cleanup()
			return fmt.Errorf("insert app: %w", err)
		}
	}

	if m.Expose.Health != "" {
		report(onProgress, "Waiting for health check")
		if err := s.mgr.HealthCheck(ctx, hostPort, m.Expose.Health); err != nil {
			// Per design: don't tear down, leave the container so the user
			// can `selfstack logs <app>`. Mark unhealthy so dashboards and
			// future commands can distinguish from a healthy app.
			s.store.UpdateAppStatus(appName, "unhealthy")
			return fmt.Errorf("health check failed (container still running, inspect via `selfstack logs %s`): %w", appName, err)
		}
	}

	if err := s.store.UpdateAppStatus(appName, "running"); err != nil {
		return err
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
	// For local installs the appDir is a symlink to a user-owned source
	// directory. Only remove the symlink itself, not the target tree.
	if fi, err := os.Lstat(appDir); err == nil && fi.Mode()&os.ModeSymlink != 0 {
		os.Remove(appDir)
	} else {
		os.RemoveAll(appDir)
	}
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

	// Pull latest code (registry installs only; local installs are
	// updated in-place by editing the source directory).
	if !isLocalApp(app) {
		report(onProgress, "Pulling updates")
		cmd := exec.CommandContext(ctx, "git", "pull")
		cmd.Dir = appDir
		if out, err := cmd.CombinedOutput(); err != nil {
			return fmt.Errorf("git pull: %s: %w", string(out), err)
		}
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

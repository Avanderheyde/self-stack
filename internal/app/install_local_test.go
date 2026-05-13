package app

import (
	"context"
	"errors"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/selfstack/selfstack/internal/registry"
	"github.com/selfstack/selfstack/internal/store"
)

// TestIsLocalApp covers the isLocalApp helper that drives the
// Update/Remove branching logic. Locality is signaled by the
// canonical SourceType column, not by RepoURL string-matching.
func TestIsLocalApp(t *testing.T) {
	tests := []struct {
		name string
		app  store.App
		want bool
	}{
		{"local source type", store.App{SourceType: "local", RepoURL: "/Users/x/proj"}, true},
		{"registry source type", store.App{SourceType: "registry", RepoURL: "https://github.com/foo/bar"}, false},
		{"deploy source type", store.App{SourceType: "deploy", RepoURL: ""}, false},
		{"empty source type", store.App{SourceType: "", RepoURL: ""}, false},
		{"local source type even with weird url", store.App{SourceType: "local", RepoURL: "https://example.com"}, true},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if got := isLocalApp(tt.app); got != tt.want {
				t.Errorf("isLocalApp(%+v) = %v, want %v", tt.app, got, tt.want)
			}
		})
	}
}

// newTestService builds a Service backed by a temp store. Tests that touch
// AppsDir should also set SELFSTACK_HOME to t.TempDir() via t.Setenv.
//
// Sets SELFSTACK_LOCAL_INSTALL_ROOTS to the OS temp root so test fixtures
// created under t.TempDir() (which lives under /var/folders on macOS, /tmp
// on Linux) are accepted by validateLocalPath. Tests that need to assert
// the rejection path should override this with their own t.Setenv.
func newTestService(t *testing.T) *Service {
	t.Helper()
	tmpRoot, err := filepath.EvalSymlinks(os.TempDir())
	if err != nil {
		t.Fatal(err)
	}
	t.Setenv(localPathRootsEnv, tmpRoot)
	dir := t.TempDir()
	s, err := store.Open(filepath.Join(dir, "test.db"))
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { s.Close() })
	r := registry.NewClient("http://localhost")
	return NewService(s, r)
}

// TestInstallLocal_PathDoesNotExist exercises the rejection branch for a
// localPath that doesn't resolve to an existing directory. validateLocalPath
// runs EvalSymlinks first and surfaces a "resolve local path" error before
// reaching the later os.Stat check — either error shape proves the missing
// path was rejected.
func TestInstallLocal_PathDoesNotExist(t *testing.T) {
	t.Setenv("SELFSTACK_HOME", t.TempDir())
	svc := newTestService(t)
	err := svc.InstallLocal(context.Background(), "myapp", filepath.Join(t.TempDir(), "nope"), "", nil)
	if err == nil {
		t.Fatal("expected error for missing local path, got nil")
	}
	if !strings.Contains(err.Error(), "stat local path") && !strings.Contains(err.Error(), "resolve local path") {
		t.Fatalf("expected stat/resolve error, got %v", err)
	}
}

// TestInstallLocal_PathIsFile exercises the "not a directory" branch.
func TestInstallLocal_PathIsFile(t *testing.T) {
	t.Setenv("SELFSTACK_HOME", t.TempDir())
	svc := newTestService(t)
	dir := t.TempDir()
	f := filepath.Join(dir, "file.txt")
	if err := os.WriteFile(f, []byte("x"), 0644); err != nil {
		t.Fatal(err)
	}
	err := svc.InstallLocal(context.Background(), "myapp", f, "", nil)
	if err == nil || !strings.Contains(err.Error(), "not a directory") {
		t.Fatalf("expected not-a-directory error, got %v", err)
	}
}

// TestInstallLocal_MissingManifest exercises the manifest-parse-fails branch.
func TestInstallLocal_MissingManifest(t *testing.T) {
	t.Setenv("SELFSTACK_HOME", t.TempDir())
	svc := newTestService(t)
	src := t.TempDir()
	err := svc.InstallLocal(context.Background(), "myapp", src, "", nil)
	if err == nil || !strings.Contains(err.Error(), "parse manifest") {
		t.Fatalf("expected parse manifest error, got %v", err)
	}
}

// writeManifest writes a minimal valid selfstack.yml at src.
func writeManifest(t *testing.T, src, name string) {
	t.Helper()
	content := "name: " + name + "\n" +
		"display_name: Test\n" +
		"description: test app\n" +
		"version: 0.1.0\n" +
		"expose:\n" +
		"  port: 3000\n"
	if err := os.WriteFile(filepath.Join(src, "selfstack.yml"), []byte(content), 0644); err != nil {
		t.Fatal(err)
	}
}

// TestInstallLocal_NameMismatch exercises the manifest-name-mismatch branch.
func TestInstallLocal_NameMismatch(t *testing.T) {
	t.Setenv("SELFSTACK_HOME", t.TempDir())
	svc := newTestService(t)
	src := t.TempDir()
	writeManifest(t, src, "otherapp")
	err := svc.InstallLocal(context.Background(), "myapp", src, "", nil)
	if err == nil || !strings.Contains(err.Error(), "does not match") {
		t.Fatalf("expected name mismatch error, got %v", err)
	}
}

// TestInstallLocal_ProgressReportsValidating verifies the first progress step
// fires before any error short-circuits the function (covers the initial
// report() call path).
func TestInstallLocal_ProgressReportsValidating(t *testing.T) {
	t.Setenv("SELFSTACK_HOME", t.TempDir())
	svc := newTestService(t)
	var steps []string
	_ = svc.InstallLocal(context.Background(), "myapp", "/does/not/exist", "", func(s string) {
		steps = append(steps, s)
	})
	if len(steps) == 0 || steps[0] != "Validating local path" {
		t.Fatalf("expected first step to be 'Validating local path', got %v", steps)
	}
}

// TestInstallLocal_RefusesWhenAlreadyInstalled covers the duplicate-row
// guard. The previous shape of this test triggered os.Symlink EEXIST by
// leaving the orphan-cleanup branch unreachable when a row existed; that
// path is now impossible because the duplicate guard refuses to proceed
// before touching the filesystem. Refusing early avoids the port-race
// bug where cleanup() would release the EXISTING app's port.
func TestInstallLocal_RefusesWhenAlreadyInstalled(t *testing.T) {
	home := t.TempDir()
	t.Setenv("SELFSTACK_HOME", home)
	svc := newTestService(t)
	src := t.TempDir()
	writeManifest(t, src, "myapp")

	// Pretend the store already has an entry — could be from a prior
	// successful install of the same name. InstallLocal must refuse.
	if err := svc.store.InsertApp(store.App{Name: "myapp", DisplayName: "x", HostPort: 10001, Status: "running"}); err != nil {
		t.Fatal(err)
	}
	err := svc.InstallLocal(context.Background(), "myapp", src, "", nil)
	if err == nil {
		t.Fatal("expected duplicate-install error, got nil")
	}
	if !strings.Contains(err.Error(), "already installed") {
		t.Fatalf("expected 'already installed' error, got %v", err)
	}
	// Confirm we didn't touch the filesystem on the way out: no symlink
	// at appDir means cleanup() never ran, so the existing app's port
	// allocation is intact.
	appDir := filepath.Join(home, "apps", "myapp")
	if _, err := os.Lstat(appDir); !os.IsNotExist(err) {
		t.Errorf("expected appDir to NOT exist after refusal, got err=%v", err)
	}
}

// TestInstallLocal_BuildFails_DockerMissing exercises the Build error +
// cleanup path. We force docker to be missing by clearing PATH; the
// symlink + port allocation succeed, then Build fails. After the error,
// the cleanup func should have removed the symlink and released the port.
//
// Skips if docker IS on PATH (we want a deterministic missing-docker env).
func TestInstallLocal_BuildFails_CleanupRunsOnMissingDocker(t *testing.T) {
	home := t.TempDir()
	t.Setenv("SELFSTACK_HOME", home)
	t.Setenv("PATH", "") // ensure docker can't be found
	svc := newTestService(t)
	src := t.TempDir()
	writeManifest(t, src, "myapp")

	err := svc.InstallLocal(context.Background(), "myapp", src, "", nil)
	if err == nil {
		t.Fatal("expected build failure without docker on PATH")
	}
	// Symlink should have been removed by cleanup.
	if _, statErr := os.Lstat(filepath.Join(home, "apps", "myapp")); !os.IsNotExist(statErr) {
		t.Fatalf("expected symlink to be cleaned up, got %v", statErr)
	}
	// The source directory must be untouched (this is the whole point of
	// the local-install symlink design).
	if _, err := os.Stat(filepath.Join(src, "selfstack.yml")); err != nil {
		t.Fatalf("source directory was damaged by cleanup: %v", err)
	}
}

// TestSeedDataVolume_StatFails exercises the early stat-fails branch.
func TestSeedDataVolume_StatFails(t *testing.T) {
	svc := newTestService(t)
	err := svc.seedDataVolume(context.Background(), "myapp", "/does/not/exist/either")
	if err == nil || !strings.Contains(err.Error(), "stat seed path") {
		t.Fatalf("expected stat seed path error, got %v", err)
	}
}

// TestSeedDataVolume_DockerMissing exercises the docker volume create
// failure path.
func TestSeedDataVolume_DockerMissing(t *testing.T) {
	t.Setenv("PATH", "")
	svc := newTestService(t)
	src := t.TempDir()
	err := svc.seedDataVolume(context.Background(), "myapp", src)
	if err == nil || !strings.Contains(err.Error(), "docker volume create") {
		t.Fatalf("expected docker volume create error, got %v", err)
	}
}

// TestRemove_SymlinkPreservesTarget covers the new branch in Remove() that
// unlinks a symlink instead of recursively deleting it. Without this branch,
// a `selfstack remove` on a --local install would wipe the user's source tree.
func TestRemove_SymlinkPreservesTarget(t *testing.T) {
	home := t.TempDir()
	t.Setenv("SELFSTACK_HOME", home)
	svc := newTestService(t)

	// Real source directory we expect to survive the Remove.
	src := t.TempDir()
	marker := filepath.Join(src, "marker.txt")
	if err := os.WriteFile(marker, []byte("keep me"), 0644); err != nil {
		t.Fatal(err)
	}

	appDir := filepath.Join(home, "apps", "myapp")
	if err := os.MkdirAll(filepath.Dir(appDir), 0755); err != nil {
		t.Fatal(err)
	}
	if err := os.Symlink(src, appDir); err != nil {
		t.Fatal(err)
	}

	// No manifest at the symlink target -> Remove skips Down() and goes
	// straight to the cleanup branch we care about.
	if err := svc.Remove(context.Background(), "myapp", nil); err != nil {
		t.Fatalf("Remove returned error: %v", err)
	}
	if _, err := os.Lstat(appDir); !os.IsNotExist(err) {
		t.Fatalf("expected symlink to be removed, got %v", err)
	}
	if _, err := os.Stat(marker); err != nil {
		t.Fatalf("expected source marker file to survive, got %v", err)
	}
}

// TestRemove_RegularDirIsDeletedRecursively covers the other side of the
// Remove() branch — the original behavior must still work for registry
// installs.
func TestRemove_RegularDirIsDeletedRecursively(t *testing.T) {
	home := t.TempDir()
	t.Setenv("SELFSTACK_HOME", home)
	svc := newTestService(t)
	appDir := filepath.Join(home, "apps", "myapp")
	if err := os.MkdirAll(filepath.Join(appDir, "sub"), 0755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(appDir, "sub", "f"), []byte("x"), 0644); err != nil {
		t.Fatal(err)
	}
	if err := svc.Remove(context.Background(), "myapp", nil); err != nil {
		t.Fatalf("Remove returned error: %v", err)
	}
	if _, err := os.Stat(appDir); !os.IsNotExist(err) {
		t.Fatalf("expected appDir removed, got %v", err)
	}
}

// TestRemove_AppDirMissing exercises the third Remove() case: neither
// symlink nor directory exists. RemoveAll on a missing path is a no-op,
// so the function should succeed silently.
func TestRemove_AppDirMissing(t *testing.T) {
	home := t.TempDir()
	t.Setenv("SELFSTACK_HOME", home)
	svc := newTestService(t)
	if err := svc.Remove(context.Background(), "ghost", nil); err != nil {
		t.Fatalf("Remove returned error: %v", err)
	}
}

// TestUpdate_LocalRepoSkipsGitPull verifies that for a local install,
// Update() does NOT run `git pull`. We assert this by constructing a fake
// app dir that is NOT a git repo — if Update tried `git pull`, the
// command would fail with a "not a git repository" error. With the new
// branch, Update should bypass git entirely and fail later (at Build).
func TestUpdate_LocalRepoSkipsGitPull(t *testing.T) {
	home := t.TempDir()
	t.Setenv("SELFSTACK_HOME", home)
	// Force docker missing so Build fails predictably AFTER the pull skip.
	t.Setenv("PATH", "")
	svc := newTestService(t)

	appDir := filepath.Join(home, "apps", "myapp")
	if err := os.MkdirAll(appDir, 0755); err != nil {
		t.Fatal(err)
	}
	writeManifest(t, appDir, "myapp")
	if err := svc.store.InsertApp(store.App{
		Name: "myapp", DisplayName: "x", RepoURL: "/wherever",
		Version: "0.1.0", HostPort: 10001, Status: "stopped",
		SourceType: "local",
	}); err != nil {
		t.Fatal(err)
	}

	err := svc.Update(context.Background(), "myapp", nil)
	if err == nil {
		t.Fatal("expected update to fail at build step")
	}
	// The failure should be at build/docker, NOT at git pull. If the
	// branch were missing, we'd see "git pull" in the error.
	if strings.Contains(err.Error(), "git pull") {
		t.Fatalf("Update tried git pull for a local repo: %v", err)
	}
}

// TestInstallLocal_AbsFails covers P3: filepath.Abs failure inside InstallLocal.
// Normally this branch is unreachable on POSIX (only fires when os.Getwd
// fails on a deleted cwd, which behaves inconsistently across platforms).
// We stub the package-level absFn to force it.
func TestInstallLocal_AbsFails(t *testing.T) {
	t.Setenv("SELFSTACK_HOME", t.TempDir())
	svc := newTestService(t)
	prev := absFn
	t.Cleanup(func() { absFn = prev })
	absFn = func(string) (string, error) { return "", errors.New("synthetic abs failure") }
	err := svc.InstallLocal(context.Background(), "myapp", "/tmp/whatever", "", nil)
	if err == nil || !strings.Contains(err.Error(), "resolve local path") {
		t.Fatalf("expected resolve local path error, got %v", err)
	}
}

// TestInstallLocal_MkdirAllFails covers P9: os.MkdirAll for the apps-dir
// parent fails. Hard to reach without root-owned dirs on the test runner;
// stub mkdirAllFn instead.
func TestInstallLocal_MkdirAllFails(t *testing.T) {
	t.Setenv("SELFSTACK_HOME", t.TempDir())
	svc := newTestService(t)
	src := t.TempDir()
	writeManifest(t, src, "myapp")
	prev := mkdirAllFn
	t.Cleanup(func() { mkdirAllFn = prev })
	mkdirAllFn = func(string, os.FileMode) error { return errors.New("synthetic mkdir failure") }
	err := svc.InstallLocal(context.Background(), "myapp", src, "", nil)
	if err == nil || !strings.Contains(err.Error(), "synthetic mkdir failure") {
		t.Fatalf("expected synthetic mkdir error, got %v", err)
	}
}

// TestSeedDataVolume_AbsFails covers P20: filepath.Abs failure inside
// seedDataVolume.
func TestSeedDataVolume_AbsFails(t *testing.T) {
	svc := newTestService(t)
	prev := absFn
	t.Cleanup(func() { absFn = prev })
	absFn = func(string) (string, error) { return "", errors.New("synthetic seed abs failure") }
	err := svc.seedDataVolume(context.Background(), "myapp", "/tmp/whatever")
	if err == nil || !strings.Contains(err.Error(), "resolve seed path") {
		t.Fatalf("expected resolve seed path error, got %v", err)
	}
}

// TestUpdate_NonLocalRepoAttemptsGitPull is the dual of the above: for a
// non-local repo, Update SHOULD attempt git pull. We don't have a git repo
// in the appDir, so the pull will fail with a git error — that error is
// our evidence the branch was taken.
func TestUpdate_NonLocalRepoAttemptsGitPull(t *testing.T) {
	home := t.TempDir()
	t.Setenv("SELFSTACK_HOME", home)
	svc := newTestService(t)

	appDir := filepath.Join(home, "apps", "myapp")
	if err := os.MkdirAll(appDir, 0755); err != nil {
		t.Fatal(err)
	}
	writeManifest(t, appDir, "myapp")
	if err := svc.store.InsertApp(store.App{
		Name: "myapp", DisplayName: "x", RepoURL: "https://example.com/repo.git",
		Version: "0.1.0", HostPort: 10001, Status: "stopped",
	}); err != nil {
		t.Fatal(err)
	}

	err := svc.Update(context.Background(), "myapp", nil)
	if err == nil || !strings.Contains(err.Error(), "git pull") {
		t.Fatalf("expected git pull error for non-local repo, got %v", err)
	}
}

// TestValidateAppName covers the allowlist regex that gates appName values
// before they're interpolated into filesystem paths or docker arguments.
// Bad values are: empty, leading hyphen, traversal (../), uppercase, slash,
// colon, too long.
func TestValidateAppName(t *testing.T) {
	good := []string{"agentboard", "vibe-costs", "a", "a1", "a_b", "app-1_v2"}
	bad := []string{
		"",
		"-bad",                 // leading hyphen
		"../etc",               // traversal
		"a/b",                  // slash
		"a:b",                  // colon (docker -v parser)
		"UPPERCASE",            // mixed case
		strings.Repeat("a", 32), // too long
	}
	for _, name := range good {
		if err := validateAppName(name); err != nil {
			t.Errorf("validateAppName(%q) rejected valid name: %v", name, err)
		}
	}
	for _, name := range bad {
		if err := validateAppName(name); err == nil {
			t.Errorf("validateAppName(%q) accepted invalid name", name)
		}
	}
}

// TestInstallLocal_RejectsBadAppName proves the API boundary refuses to
// run InstallLocal at all when appName fails the allowlist. Without this
// check, an attacker controlling appName via the HTTP body could plant
// symlinks or trigger RemoveAll outside the apps directory.
func TestInstallLocal_RejectsBadAppName(t *testing.T) {
	svc := newTestService(t)
	t.Setenv("SELFSTACK_HOME", t.TempDir())
	// Use a real src dir so we'd otherwise get further into the flow.
	src := t.TempDir()
	writeManifest(t, src, "../../etc/evil")
	err := svc.InstallLocal(context.Background(), "../../etc/evil", src, "", nil)
	if err == nil || !strings.Contains(err.Error(), "invalid app name") {
		t.Fatalf("expected invalid app name error, got %v", err)
	}
}

// TestInstallLocal_RejectsOutsideAllowlist covers the localPath sandbox.
// A path that resolves outside the configured roots (default $HOME, here
// the OS temp root) is rejected before any filesystem mutation runs.
func TestInstallLocal_RejectsOutsideAllowlist(t *testing.T) {
	svc := newTestService(t)
	t.Setenv("SELFSTACK_HOME", t.TempDir())
	// Override newTestService's permissive default — restrict to a path
	// that does NOT contain /tmp, so the source under t.TempDir() is rejected.
	t.Setenv(localPathRootsEnv, "/nonexistent-only-root")
	src := t.TempDir()
	writeManifest(t, src, "myapp")
	err := svc.InstallLocal(context.Background(), "myapp", src, "", nil)
	if err == nil {
		t.Fatal("expected rejection for path outside allowlist")
	}
	if !strings.Contains(err.Error(), "outside allowed roots") && !strings.Contains(err.Error(), "no usable local-install roots") {
		t.Fatalf("expected allowlist rejection, got %v", err)
	}
}

// TestInstallLocal_RejectsBadSeedDataPath checks that seed-data is
// independently sandboxed from local_path. Pointing at /etc would be
// the canonical attack: legitimate local_path + malicious seed_data to
// copy /etc into a docker volume the attacker's compose then mounts.
func TestInstallLocal_RejectsBadSeedDataPath(t *testing.T) {
	svc := newTestService(t)
	t.Setenv("SELFSTACK_HOME", t.TempDir())
	src := t.TempDir() // allowed (under newTestService's tmp root)
	writeManifest(t, src, "myapp")
	// seed dir lives outside the allowlist root.
	err := svc.InstallLocal(context.Background(), "myapp", src, "/etc", nil)
	if err == nil {
		t.Fatal("expected rejection for seed-data outside allowlist")
	}
	if !strings.Contains(err.Error(), "outside allowed roots") && !strings.Contains(err.Error(), "seed-data path") {
		t.Fatalf("expected seed-data allowlist rejection, got %v", err)
	}
}

// TestValidateLocalPath_FollowsSymlinks proves the sandbox can't be
// bypassed by creating a symlink inside an allowed root that points to
// an out-of-bounds target. The symlink target is what gets validated.
func TestValidateLocalPath_FollowsSymlinks(t *testing.T) {
	allowed, err := filepath.EvalSymlinks(t.TempDir())
	if err != nil {
		t.Fatal(err)
	}
	t.Setenv(localPathRootsEnv, allowed)

	outside := t.TempDir() // different tmpdir, NOT under `allowed`
	if strings.HasPrefix(outside, allowed) {
		t.Skipf("test temp dirs share a parent on this platform; skipping symlink-bypass check")
	}

	link := filepath.Join(allowed, "decoy")
	if err := os.Symlink(outside, link); err != nil {
		t.Fatal(err)
	}
	if _, err := validateLocalPath("test", link); err == nil {
		t.Fatal("symlink inside allowed root pointing outside should be rejected after EvalSymlinks")
	}
}

// TestInstallLocal_RejectsManifestRuntimeEntryTraversal covers the
// runtime.entry path-injection finding from adversarial review. A
// malicious selfstack.yml could set runtime.entry to "../../etc/passwd"
// to make `docker compose -f` read an arbitrary host file as a compose
// definition. filepath.IsLocal rejects any escape.
func TestInstallLocal_RejectsManifestRuntimeEntryTraversal(t *testing.T) {
	home := t.TempDir()
	t.Setenv("SELFSTACK_HOME", home)
	svc := newTestService(t)
	src := t.TempDir()

	manifestPath := filepath.Join(src, "selfstack.yml")
	content := "name: myapp\nruntime:\n  type: docker-compose\n  entry: ../../../etc/passwd\nexpose:\n  port: 3000\n"
	if err := os.WriteFile(manifestPath, []byte(content), 0644); err != nil {
		t.Fatal(err)
	}

	err := svc.InstallLocal(context.Background(), "myapp", src, "", nil)
	if err == nil {
		t.Fatal("expected rejection for traversal in manifest runtime.entry")
	}
	if !strings.Contains(err.Error(), "runtime.entry") || !strings.Contains(err.Error(), "escapes") {
		t.Fatalf("expected runtime.entry escape error, got %v", err)
	}
}

// TestInstallLocal_RejectsAbsoluteRuntimeEntry verifies that absolute
// paths in runtime.entry are also rejected (filepath.IsLocal returns
// false for absolute paths).
func TestInstallLocal_RejectsAbsoluteRuntimeEntry(t *testing.T) {
	home := t.TempDir()
	t.Setenv("SELFSTACK_HOME", home)
	svc := newTestService(t)
	src := t.TempDir()

	manifestPath := filepath.Join(src, "selfstack.yml")
	content := "name: myapp\nruntime:\n  type: docker-compose\n  entry: /etc/passwd\nexpose:\n  port: 3000\n"
	if err := os.WriteFile(manifestPath, []byte(content), 0644); err != nil {
		t.Fatal(err)
	}

	err := svc.InstallLocal(context.Background(), "myapp", src, "", nil)
	if err == nil {
		t.Fatal("expected rejection for absolute path in manifest runtime.entry")
	}
	if !strings.Contains(err.Error(), "runtime.entry") {
		t.Fatalf("expected runtime.entry rejection, got %v", err)
	}
}

// TestSeedDataVolume_RejectsSymlinkAtSrc covers the TOCTOU mitigation:
// even if a path passed validateLocalPath at the top of InstallLocal,
// if it has since become a symlink (attacker swapped a directory for
// a link to /etc), seedDataVolume's Lstat check refuses to bind-mount it.
func TestSeedDataVolume_RejectsSymlinkAtSrc(t *testing.T) {
	tmp := t.TempDir()
	link := filepath.Join(tmp, "decoy")
	target := t.TempDir()
	if err := os.Symlink(target, link); err != nil {
		t.Fatal(err)
	}
	svc := newTestService(t)
	err := svc.seedDataVolume(context.Background(), "myapp", link)
	if err == nil {
		t.Fatal("expected symlink rejection")
	}
	if !strings.Contains(err.Error(), "symlink") {
		t.Fatalf("expected symlink-shaped error, got %v", err)
	}
}

// TestSeedDataVolume_RejectsNonDirSrc proves the bind-mount won't be
// attempted against a regular file (a precondition for the cp -a
// command). Without this check we'd create the docker volume then
// fail at cp time, leaking the volume.
func TestSeedDataVolume_RejectsNonDirSrc(t *testing.T) {
	tmp := t.TempDir()
	f := filepath.Join(tmp, "afile")
	if err := os.WriteFile(f, []byte("x"), 0644); err != nil {
		t.Fatal(err)
	}
	svc := newTestService(t)
	err := svc.seedDataVolume(context.Background(), "myapp", f)
	if err == nil {
		t.Fatal("expected non-directory rejection")
	}
	if !strings.Contains(err.Error(), "not a directory") {
		t.Fatalf("expected 'not a directory' error, got %v", err)
	}
}

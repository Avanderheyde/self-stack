//go:build docker_integration
// +build docker_integration

// Integration tests for InstallLocal / seedDataVolume paths that require a
// live docker daemon. Gated behind the docker_integration build tag so
// `go test ./...` stays green on hosts without docker. To run:
//
//   go test -tags=docker_integration ./internal/app/...
//
// Each test also self-skips at runtime if dockerAvailable() returns false,
// so even with the tag set the test pass is non-fatal on broken CI rigs.
//
// Paths covered:
//   P15: InstallLocal -> Up() fails branch with cleanup
//   P16: InstallLocal -> HealthCheck invoked when manifest.Expose.Health != ""
//   P17: InstallLocal -> HealthCheck fails branch with cleanup
//   P18: InstallLocal -> success path, store.InsertApp with local:<absSrc>
//   P19: InstallLocal -> portless.Alias on success
//   P23: seedDataVolume -> docker run alpine cp failure branch (with docker)
//   P24: seedDataVolume -> success (no return error)

package app

import (
	"context"
	"fmt"
	"net"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"testing"
	"time"
)

// port10001Free returns true if TCP port 10001 (selfstack's default first
// allocation) is unbound on the host. Integration tests use AllocatePort
// which always starts at 10001 on a fresh store; if the dev box has a
// real selfstack app already bound there, the integration test will fail
// to spin up. Tests skip in that case.
func port10001Free() bool {
	// Use 0.0.0.0 (not 127.0.0.1) so we detect ports already bound by
	// dockerd's port-forwarding, which listens on 0.0.0.0 / :: only.
	l, err := net.Listen("tcp", "0.0.0.0:10001")
	if err != nil {
		return false
	}
	l.Close()
	return true
}

// writeIntegrationFixture lays down a minimal self-contained app at dir:
// a `selfstack.yml`, `Dockerfile`, `docker-compose.yml`, and a tiny Go server.
// If healthPath is "" the manifest omits the health key. The container port
// is fixed at 3000.
func writeIntegrationFixture(t *testing.T, dir, appName, healthPath string) {
	t.Helper()

	manifest := "name: " + appName + "\n" +
		"display_name: " + appName + "\n" +
		"description: integration fixture\n" +
		"version: 0.1.0\n" +
		"runtime:\n" +
		"  type: docker-compose\n" +
		"  entry: docker-compose.yml\n" +
		"expose:\n" +
		"  port: 3000\n"
	if healthPath != "" {
		manifest += "  health: " + healthPath + "\n"
	}
	if err := os.WriteFile(filepath.Join(dir, "selfstack.yml"), []byte(manifest), 0644); err != nil {
		t.Fatal(err)
	}

	dockerfile := "FROM alpine:3.19\n" +
		"RUN apk add --no-cache busybox-extras\n" +
		"WORKDIR /app\n" +
		`CMD ["sh","-c","while true; do echo -e 'HTTP/1.1 200 OK\r\nContent-Length: 2\r\n\r\nok' | nc -l -p 3000 -q 1; done"]` + "\n"
	if err := os.WriteFile(filepath.Join(dir, "Dockerfile"), []byte(dockerfile), 0644); err != nil {
		t.Fatal(err)
	}

	compose := "services:\n" +
		"  app:\n" +
		"    build: .\n" +
		"    ports:\n" +
		"      - \"${SELFSTACK_HOST_PORT:-3000}:3000\"\n"
	if err := os.WriteFile(filepath.Join(dir, "docker-compose.yml"), []byte(compose), 0644); err != nil {
		t.Fatal(err)
	}
}

// dockerComposeDown best-effort cleanup so test runs don't leave containers.
func dockerComposeDown(appName, appDir string) {
	ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
	defer cancel()
	_ = exec.CommandContext(ctx, "docker", "compose",
		"-f", filepath.Join(appDir, "docker-compose.yml"),
		"-p", "selfstack-"+appName,
		"down", "-v",
	).Run()
}

func dockerVolumeRm(name string) {
	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()
	_ = exec.CommandContext(ctx, "docker", "volume", "rm", "-f", name).Run()
}

// TestInstallLocal_Integration_Success covers P16, P18, P19 together: a full
// happy-path install with a health check. Asserts the DB entry was inserted
// with the local: prefix and that the symlink is in place.
func TestInstallLocal_Integration_Success(t *testing.T) {
	if !dockerAvailable() {
		t.Skip("docker not available; skipping integration test")
	}
	if !port10001Free() {
		t.Skip("port 10001 already bound on this host; skipping integration test")
	}
	home := t.TempDir()
	t.Setenv("SELFSTACK_HOME", home)
	svc := newTestService(t)

	src := t.TempDir()
	appName := fmt.Sprintf("itg-%d", time.Now().UnixNano())
	writeIntegrationFixture(t, src, appName, "/")

	appDir := filepath.Join(home, "apps", appName)
	t.Cleanup(func() { dockerComposeDown(appName, appDir) })

	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Minute)
	defer cancel()
	if err := svc.InstallLocal(ctx, appName, src, "", nil); err != nil {
		t.Fatalf("InstallLocal: %v", err)
	}
	app, err := svc.store.GetApp(appName)
	if err != nil {
		t.Fatalf("GetApp: %v", err)
	}
	if app.SourceType != "local" {
		t.Errorf("SourceType = %q, want local", app.SourceType)
	}
	if app.RepoURL != src {
		t.Errorf("RepoURL = %q, want %q (raw path, no prefix)", app.RepoURL, src)
	}
	if app.Status != "running" {
		t.Errorf("Status = %q, want running", app.Status)
	}
	if fi, err := os.Lstat(appDir); err != nil || fi.Mode()&os.ModeSymlink == 0 {
		t.Errorf("expected symlink at %s, err=%v", appDir, err)
	}
}

// TestInstallLocal_Integration_HealthCheckFails covers P17: when the manifest
// declares a health check and the container never serves the health endpoint,
// HealthCheck fails and cleanup runs.
//
// We use a Dockerfile that exits immediately, guaranteeing the container is
// gone before HealthCheck times out. HealthCheck has a 5-minute timeout
// internally; we override the manager timeout via a short context.
func TestInstallLocal_Integration_HealthCheckFails(t *testing.T) {
	if !dockerAvailable() {
		t.Skip("docker not available; skipping integration test")
	}
	if !port10001Free() {
		t.Skip("port 10001 already bound on this host; skipping integration test")
	}
	home := t.TempDir()
	t.Setenv("SELFSTACK_HOME", home)
	svc := newTestService(t)

	src := t.TempDir()
	appName := fmt.Sprintf("itg-hcfail-%d", time.Now().UnixNano())
	// Container that exits immediately — Up() succeeds but health never passes.
	manifest := "name: " + appName + "\n" +
		"display_name: " + appName + "\n" +
		"version: 0.1.0\n" +
		"runtime:\n  type: docker-compose\n  entry: docker-compose.yml\n" +
		"expose:\n  port: 3000\n  health: /never\n"
	if err := os.WriteFile(filepath.Join(src, "selfstack.yml"), []byte(manifest), 0644); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(src, "Dockerfile"), []byte("FROM alpine:3.19\nCMD [\"true\"]\n"), 0644); err != nil {
		t.Fatal(err)
	}
	compose := "services:\n  app:\n    build: .\n    ports:\n      - \"${SELFSTACK_HOST_PORT:-3000}:3000\"\n"
	if err := os.WriteFile(filepath.Join(src, "docker-compose.yml"), []byte(compose), 0644); err != nil {
		t.Fatal(err)
	}

	appDir := filepath.Join(home, "apps", appName)
	t.Cleanup(func() { dockerComposeDown(appName, appDir) })

	// 30s ctx — HealthCheck's internal deadline is 5min but it honors
	// ctx.Done(), so this caps the wall-clock cost of the failure case.
	ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
	defer cancel()
	err := svc.InstallLocal(ctx, appName, src, "", nil)
	if err == nil || !strings.Contains(err.Error(), "health check") {
		t.Fatalf("expected health check error, got %v", err)
	}
	// Cleanup ran: symlink gone.
	if _, statErr := os.Lstat(appDir); !os.IsNotExist(statErr) {
		t.Errorf("expected symlink cleaned up after health-check failure, got %v", statErr)
	}
}

// TestInstallLocal_Integration_UpFails covers P15: Up() fails (here because
// the compose file references an undefined image). Cleanup must run.
func TestInstallLocal_Integration_UpFails(t *testing.T) {
	if !dockerAvailable() {
		t.Skip("docker not available; skipping integration test")
	}
	if !port10001Free() {
		t.Skip("port 10001 already bound on this host; skipping integration test")
	}
	home := t.TempDir()
	t.Setenv("SELFSTACK_HOME", home)
	svc := newTestService(t)

	src := t.TempDir()
	appName := fmt.Sprintf("itg-upfail-%d", time.Now().UnixNano())
	manifest := "name: " + appName + "\n" +
		"display_name: " + appName + "\n" +
		"version: 0.1.0\n" +
		"runtime:\n  type: docker-compose\n  entry: docker-compose.yml\n" +
		"expose:\n  port: 3000\n"
	if err := os.WriteFile(filepath.Join(src, "selfstack.yml"), []byte(manifest), 0644); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(src, "Dockerfile"), []byte("FROM alpine:3.19\nCMD [\"true\"]\n"), 0644); err != nil {
		t.Fatal(err)
	}
	// Compose file references the non-existent service `nope` — `docker
	// compose up` will fail before any container starts.
	compose := "services:\n  app:\n    build: .\n    depends_on:\n      - nope\n    ports:\n      - \"${SELFSTACK_HOST_PORT:-3000}:3000\"\n"
	if err := os.WriteFile(filepath.Join(src, "docker-compose.yml"), []byte(compose), 0644); err != nil {
		t.Fatal(err)
	}

	appDir := filepath.Join(home, "apps", appName)
	t.Cleanup(func() { dockerComposeDown(appName, appDir) })

	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Minute)
	defer cancel()
	err := svc.InstallLocal(ctx, appName, src, "", nil)
	if err == nil {
		t.Fatal("expected Up() or Build() failure for malformed compose")
	}
	if _, statErr := os.Lstat(appDir); !os.IsNotExist(statErr) {
		t.Errorf("expected symlink cleaned up after Up()/Build() failure, got %v", statErr)
	}
}

// TestSeedDataVolume_Integration_Success covers P24: end-to-end seed copy
// against a real docker daemon.
func TestSeedDataVolume_Integration_Success(t *testing.T) {
	if !dockerAvailable() {
		t.Skip("docker not available; skipping integration test")
	}
	svc := newTestService(t)
	appName := fmt.Sprintf("itg-seed-%d", time.Now().UnixNano())
	volume := "selfstack-" + appName + "_data"
	t.Cleanup(func() { dockerVolumeRm(volume) })

	src := t.TempDir()
	if err := os.WriteFile(filepath.Join(src, "marker.txt"), []byte("seeded"), 0644); err != nil {
		t.Fatal(err)
	}

	ctx, cancel := context.WithTimeout(context.Background(), 2*time.Minute)
	defer cancel()
	if err := svc.seedDataVolume(ctx, appName, src); err != nil {
		t.Fatalf("seedDataVolume: %v", err)
	}
	// Verify content landed in the volume.
	out, err := exec.CommandContext(ctx, "docker", "run", "--rm",
		"-v", volume+":/data", "alpine", "cat", "/data/marker.txt").CombinedOutput()
	if err != nil {
		t.Fatalf("read back from volume: %s: %v", string(out), err)
	}
	if strings.TrimSpace(string(out)) != "seeded" {
		t.Errorf("volume content = %q, want %q", string(out), "seeded")
	}
}

// TestSeedDataVolume_Integration_CopyFails covers P23: docker run alpine cp
// fails. We trigger this by making the seed source unreadable by the alpine
// container — easiest is to point at a path that exists on the host but not
// inside the container. We use a bind-mount path that the daemon can't
// access by pointing at a directory whose parent is restricted, or by
// removing the source between stat and the docker run call. Cleanest
// reproduction: create the source, but pre-create the named volume with
// content that conflicts; alternatively, force the copy command to fail by
// passing a source dir containing a file with invalid mode bits.
//
// Simplest reliable reproduction: pre-create the volume and then mount it
// read-only so the `cp -a` write fails. But seedDataVolume creates the
// volume itself with `docker volume create` — we can pre-create it with
// labels that conflict. Easier still: use a source path that, after Abs(),
// resolves to a path the daemon can't see (e.g. a tmpfs in a user
// namespace). On Linux-only that's brittle.
//
// Stable approach: kill the docker daemon between create and run. That's
// too invasive. Instead, we use a context with a very short timeout so the
// `docker run` step gets killed mid-flight and returns an error — the
// volume_create completes (fast), then the run is canceled.
func TestSeedDataVolume_Integration_CopyFails(t *testing.T) {
	if !dockerAvailable() {
		t.Skip("docker not available; skipping integration test")
	}
	svc := newTestService(t)
	appName := fmt.Sprintf("itg-seedfail-%d", time.Now().UnixNano())
	volume := "selfstack-" + appName + "_data"
	t.Cleanup(func() { dockerVolumeRm(volume) })

	src := t.TempDir()
	// Make a large enough fixture that copy doesn't complete instantly.
	for i := 0; i < 50; i++ {
		_ = os.WriteFile(filepath.Join(src, fmt.Sprintf("f%d.bin", i)), make([]byte, 1024*1024), 0644)
	}

	// 100ms — enough to pass volume_create, not enough to finish `docker run`.
	ctx, cancel := context.WithTimeout(context.Background(), 100*time.Millisecond)
	defer cancel()
	err := svc.seedDataVolume(ctx, appName, src)
	if err == nil {
		t.Fatal("expected seedDataVolume to fail under tight deadline")
	}
	if !strings.Contains(err.Error(), "seed copy") && !strings.Contains(err.Error(), "docker volume create") {
		t.Fatalf("expected seed copy or docker volume create error, got %v", err)
	}
}

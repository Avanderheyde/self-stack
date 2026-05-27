package container

import (
	"context"
	"fmt"
	"io"
	"net/http"
	"os/exec"
	"path/filepath"
	"time"
)

var execLookPath = exec.LookPath

type Manager struct {
	timeout time.Duration
}

func NewManager() *Manager {
	return &Manager{timeout: 5 * time.Minute}
}

func (m *Manager) Build(ctx context.Context, appDir string, composePath string, appName string) error {
	// Allow Docker's layer cache so redeploys can hit the <60s warm-cache
	// target from the design doc. A clean rebuild is still possible via
	// `docker builder prune` or a fresh app name.
	cmd := composeCommand(ctx, "-f", filepath.Join(appDir, composePath), "-p", "selfstack-"+appName, "build")
	cmd.Dir = appDir
	out, err := cmd.CombinedOutput()
	if err != nil {
		return fmt.Errorf("docker compose build: %s: %w", string(out), err)
	}
	return nil
}

func (m *Manager) Up(ctx context.Context, appDir, composePath string, appName string, containerPort, hostPort int, envVars map[string]string) error {
	cmd := composeCommand(ctx, "-f", filepath.Join(appDir, composePath), "-p", "selfstack-"+appName, "up", "-d", "--force-recreate")
	cmd.Dir = appDir
	env := cmd.Environ()
	env = append(env, fmt.Sprintf("SELFSTACK_HOST_PORT=%d", hostPort))
	env = append(env, fmt.Sprintf("SELFSTACK_CONTAINER_PORT=%d", containerPort))
	for k, v := range envVars {
		env = append(env, fmt.Sprintf("%s=%s", k, v))
	}
	cmd.Env = env
	out, err := cmd.CombinedOutput()
	if err != nil {
		return fmt.Errorf("docker compose up: %s: %w", string(out), err)
	}
	return nil
}

func (m *Manager) Down(ctx context.Context, appDir, composePath, appName string) error {
	cmd := composeCommand(ctx, "-f", filepath.Join(appDir, composePath), "-p", "selfstack-"+appName, "down")
	cmd.Dir = appDir
	out, err := cmd.CombinedOutput()
	if err != nil {
		return fmt.Errorf("docker compose down: %s: %w", string(out), err)
	}
	return nil
}

func (m *Manager) Stop(ctx context.Context, appDir, composePath, appName string) error {
	cmd := composeCommand(ctx, "-f", filepath.Join(appDir, composePath), "-p", "selfstack-"+appName, "stop")
	cmd.Dir = appDir
	out, err := cmd.CombinedOutput()
	if err != nil {
		return fmt.Errorf("docker compose stop: %s: %w", string(out), err)
	}
	return nil
}

func (m *Manager) Start(ctx context.Context, appDir, composePath, appName string) error {
	cmd := composeCommand(ctx, "-f", filepath.Join(appDir, composePath), "-p", "selfstack-"+appName, "start")
	cmd.Dir = appDir
	out, err := cmd.CombinedOutput()
	if err != nil {
		return fmt.Errorf("docker compose start: %s: %w", string(out), err)
	}
	return nil
}

func (m *Manager) Logs(ctx context.Context, appDir, composePath, appName string, w io.Writer) error {
	cmd := composeCommand(ctx, "-f", filepath.Join(appDir, composePath), "-p", "selfstack-"+appName, "logs", "-f", "--tail=100")
	cmd.Dir = appDir
	cmd.Stdout = w
	cmd.Stderr = w
	return cmd.Run()
}

func composeCommand(ctx context.Context, args ...string) *exec.Cmd {
	name, prefix := composeCommandParts()
	return exec.CommandContext(ctx, name, append(prefix, args...)...)
}

func composeCommandParts() (string, []string) {
	if _, err := execLookPath("docker-compose"); err == nil {
		return "docker-compose", nil
	}
	return "docker", []string{"compose"}
}

func (m *Manager) HealthCheck(ctx context.Context, hostPort int, healthPath string) error {
	url := fmt.Sprintf("http://localhost:%d%s", hostPort, healthPath)
	deadline := time.Now().Add(m.timeout)
	for time.Now().Before(deadline) {
		resp, err := http.Get(url)
		if err == nil && resp.StatusCode >= 200 && resp.StatusCode < 400 {
			resp.Body.Close()
			return nil
		}
		if resp != nil {
			resp.Body.Close()
		}
		select {
		case <-ctx.Done():
			return ctx.Err()
		case <-time.After(2 * time.Second):
		}
	}
	return fmt.Errorf("health check timeout for %s", url)
}

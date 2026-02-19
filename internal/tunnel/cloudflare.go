package tunnel

import (
	"context"
	"fmt"
	"os/exec"
)

type CloudflareTunnel struct {
	token   string
	running bool
	cmd     *exec.Cmd
}

func NewCloudflare(token string) *CloudflareTunnel {
	return &CloudflareTunnel{token: token}
}

func (t *CloudflareTunnel) Start(ctx context.Context) error {
	if t.token == "" {
		return fmt.Errorf("cloudflare tunnel token not configured — run 'selfstack tunnel setup'")
	}
	t.cmd = exec.CommandContext(ctx, "cloudflared", "tunnel", "run", "--token", t.token)
	if err := t.cmd.Start(); err != nil {
		return fmt.Errorf("start cloudflared: %w", err)
	}
	t.running = true
	return nil
}

func (t *CloudflareTunnel) Stop() error {
	if t.cmd != nil && t.cmd.Process != nil {
		t.running = false
		return t.cmd.Process.Kill()
	}
	return nil
}

func (t *CloudflareTunnel) IsRunning() bool { return t.running }

func IsInstalled() bool {
	_, err := exec.LookPath("cloudflared")
	return err == nil
}

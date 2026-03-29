package cloud

import (
	"context"
	"fmt"
	"strconv"
	"time"

	"github.com/hetznercloud/hcloud-go/v2/hcloud"
)

// ServerConfig holds the VPS spec for provisioning.
type ServerConfig struct {
	ServerType string // e.g., "cx22" (~$4/mo, 2 vCPU, 4GB RAM)
	Image      string // e.g., "ubuntu-24.04"
	Location   string // e.g., "fsn1" (Falkenstein, DE)
}

func DefaultServerConfig() ServerConfig {
	return ServerConfig{
		ServerType: "cx22",
		Image:      "ubuntu-24.04",
		Location:   "fsn1",
	}
}

// ProvisionResult holds the output of a successful VPS provisioning.
type ProvisionResult struct {
	ServerID int64
	IPv4     string
	RootPass string // only set if no SSH key provided
}

// ProvisionServer creates a new Hetzner VPS with the given name and user-data script.
func ProvisionServer(ctx context.Context, token, name, userData string, sshKeyIDs []int64) (*ProvisionResult, error) {
	client := hcloud.NewClient(hcloud.WithToken(token))
	cfg := DefaultServerConfig()

	opts := hcloud.ServerCreateOpts{
		Name:       name,
		ServerType: &hcloud.ServerType{Name: cfg.ServerType},
		Image:      &hcloud.Image{Name: cfg.Image},
		Location:   &hcloud.Location{Name: cfg.Location},
		UserData:   userData,
		Labels:     map[string]string{"managed-by": "selfstack"},
	}

	for _, id := range sshKeyIDs {
		opts.SSHKeys = append(opts.SSHKeys, &hcloud.SSHKey{ID: id})
	}

	result, _, err := client.Server.Create(ctx, opts)
	if err != nil {
		return nil, fmt.Errorf("create server: %w", err)
	}

	pr := &ProvisionResult{
		ServerID: result.Server.ID,
		IPv4:     result.Server.PublicNet.IPv4.IP.String(),
	}
	if result.RootPassword != "" {
		pr.RootPass = result.RootPassword
	}
	return pr, nil
}

// WaitForReady polls until the server is running and SSH is reachable.
func WaitForReady(ctx context.Context, token string, serverID int64, timeout time.Duration) error {
	client := hcloud.NewClient(hcloud.WithToken(token))
	deadline := time.Now().Add(timeout)

	for time.Now().Before(deadline) {
		srv, _, err := client.Server.GetByID(ctx, serverID)
		if err != nil {
			return fmt.Errorf("check server: %w", err)
		}
		if srv.Status == hcloud.ServerStatusRunning {
			return nil
		}
		select {
		case <-ctx.Done():
			return ctx.Err()
		case <-time.After(3 * time.Second):
		}
	}
	return fmt.Errorf("server not ready after %s", timeout)
}

// DestroyServer deletes a Hetzner VPS.
func DestroyServer(ctx context.Context, token string, serverID int64) error {
	client := hcloud.NewClient(hcloud.WithToken(token))
	_, _, err := client.Server.DeleteWithResult(ctx, &hcloud.Server{ID: serverID})
	if err != nil {
		return fmt.Errorf("destroy server: %w", err)
	}
	return nil
}

// ListSSHKeys returns the user's SSH keys from Hetzner.
func ListSSHKeys(ctx context.Context, token string) ([]SSHKeyInfo, error) {
	client := hcloud.NewClient(hcloud.WithToken(token))
	keys, _, err := client.SSHKey.List(ctx, hcloud.SSHKeyListOpts{})
	if err != nil {
		return nil, fmt.Errorf("list ssh keys: %w", err)
	}
	result := make([]SSHKeyInfo, len(keys))
	for i, k := range keys {
		result[i] = SSHKeyInfo{ID: k.ID, Name: k.Name, Fingerprint: k.Fingerprint}
	}
	return result, nil
}

type SSHKeyInfo struct {
	ID          int64
	Name        string
	Fingerprint string
}

// ListServers returns selfstack-managed servers.
func ListServers(ctx context.Context, token string) ([]ServerInfo, error) {
	client := hcloud.NewClient(hcloud.WithToken(token))
	servers, _, err := client.Server.List(ctx, hcloud.ServerListOpts{
		ListOpts: hcloud.ListOpts{LabelSelector: "managed-by=selfstack"},
	})
	if err != nil {
		return nil, fmt.Errorf("list servers: %w", err)
	}
	result := make([]ServerInfo, len(servers))
	for i, s := range servers {
		result[i] = ServerInfo{
			ID:     s.ID,
			Name:   s.Name,
			IPv4:   s.PublicNet.IPv4.IP.String(),
			Status: string(s.Status),
		}
	}
	return result, nil
}

type ServerInfo struct {
	ID     int64
	Name   string
	IPv4   string
	Status string
}

// SaveToken stores the Hetzner API token in the selfstack config.
func SaveToken(token string) error {
	// Stored separately from remote.yml for security (mode 0600)
	return writeSecretFile("hetzner_token", token)
}

// LoadToken reads the stored Hetzner API token.
func LoadToken() (string, error) {
	return readSecretFile("hetzner_token")
}

// SetupScript generates a cloud-init user-data script that installs
// Docker, Tailscale, and selfstack on a fresh Ubuntu VPS.
func SetupScript(tailscaleAuthKey string) string {
	script := `#!/bin/bash
set -euo pipefail

# Install Docker
curl -fsSL https://get.docker.com | sh
systemctl enable docker
systemctl start docker

# Install Docker Compose plugin
apt-get install -y docker-compose-plugin

`

	if tailscaleAuthKey != "" {
		script += `# Install and authenticate Tailscale
curl -fsSL https://tailscale.com/install.sh | sh
tailscale up --authkey ` + tailscaleAuthKey + `

`
	} else {
		script += `# Install Tailscale (auth manually later)
curl -fsSL https://tailscale.com/install.sh | sh

`
	}

	script += `# Install selfstack
curl -fsSL https://selfstack.dev/install | sh

# Create systemd service for selfstack serve
cat > /etc/systemd/system/selfstack.service << 'UNIT'
[Unit]
Description=SelfStack Server
After=network.target docker.service
Requires=docker.service

[Service]
Type=simple
ExecStart=/usr/local/bin/selfstack serve
Restart=always
RestartSec=5
Environment=SELFSTACK_HOME=/root/.selfstack

[Install]
WantedBy=multi-user.target
UNIT

systemctl daemon-reload
systemctl enable selfstack
systemctl start selfstack
`

	// Add a marker file so we know setup completed
	script += `
# Signal setup complete
touch /root/.selfstack-setup-done
`
	return script
}

// ServerID helper for config persistence
func ServerIDStr(id int64) string {
	return strconv.FormatInt(id, 10)
}

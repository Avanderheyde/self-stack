package tailscale

import (
	"encoding/json"
	"fmt"
	"os/exec"
	"strings"
)

// Status holds the result of `tailscale status --json`.
type Status struct {
	Running  bool
	Hostname string
	DNSName  string
	IP       string
}

type statusJSON struct {
	Self struct {
		HostName     string   `json:"HostName"`
		DNSName      string   `json:"DNSName"`
		Online       bool     `json:"Online"`
		TailscaleIPs []string `json:"TailscaleIPs"`
	} `json:"Self"`
}

func parseStatus(data []byte) (Status, error) {
	var raw statusJSON
	if err := json.Unmarshal(data, &raw); err != nil {
		return Status{}, fmt.Errorf("parse tailscale status: %w", err)
	}
	ip := ""
	if len(raw.Self.TailscaleIPs) > 0 {
		ip = raw.Self.TailscaleIPs[0]
	}
	return Status{
		Running:  true,
		Hostname: raw.Self.HostName,
		DNSName:  raw.Self.DNSName,
		IP:       ip,
	}, nil
}

// serveURL returns the HTTPS URL for an app served via Tailscale Serve.
func serveURL(dnsName string, port int) string {
	name := strings.TrimSuffix(dnsName, ".")
	return fmt.Sprintf("https://%s:%d", name, port)
}

// GetStatus checks if Tailscale is running and returns node info.
func GetStatus() (Status, error) {
	out, err := exec.Command("tailscale", "status", "--json").CombinedOutput()
	if err != nil {
		return Status{}, fmt.Errorf("tailscale not running or not installed: %w", err)
	}
	return parseStatus(out)
}

// Serve configures Tailscale Serve to expose a port via HTTPS on the tailnet.
// Returns the Tailscale URL for the app.
func Serve(port int) (string, error) {
	status, err := GetStatus()
	if err != nil {
		return "", err
	}

	// tailscale serve --bg <port>
	cmd := exec.Command("tailscale", "serve", "--bg", fmt.Sprintf("%d", port))
	if out, err := cmd.CombinedOutput(); err != nil {
		return "", fmt.Errorf("tailscale serve: %s: %w", strings.TrimSpace(string(out)), err)
	}

	return serveURL(status.DNSName, port), nil
}

// ServeRemote configures Tailscale Serve on a remote host via SSH.
func ServeRemote(sshTarget string, port int) (string, error) {
	// Get remote tailscale status
	statusCmd := exec.Command("ssh", sshTarget, "tailscale", "status", "--json")
	out, err := statusCmd.CombinedOutput()
	if err != nil {
		return "", fmt.Errorf("remote tailscale not available: %w", err)
	}
	status, err := parseStatus(out)
	if err != nil {
		return "", err
	}

	// Configure serve on remote
	serveCmd := exec.Command("ssh", sshTarget, "tailscale", "serve", "--bg", fmt.Sprintf("%d", port))
	if out, err := serveCmd.CombinedOutput(); err != nil {
		return "", fmt.Errorf("remote tailscale serve: %s: %w", strings.TrimSpace(string(out)), err)
	}

	return serveURL(status.DNSName, port), nil
}

// Reset removes the Tailscale Serve configuration for a port.
func Reset(port int) error {
	cmd := exec.Command("tailscale", "serve", "off", fmt.Sprintf("%d", port))
	if out, err := cmd.CombinedOutput(); err != nil {
		return fmt.Errorf("tailscale serve off: %s: %w", strings.TrimSpace(string(out)), err)
	}
	return nil
}

// ResetRemote removes Tailscale Serve on a remote host.
func ResetRemote(sshTarget string, port int) error {
	cmd := exec.Command("ssh", sshTarget, "tailscale", "serve", "off", fmt.Sprintf("%d", port))
	if out, err := cmd.CombinedOutput(); err != nil {
		return fmt.Errorf("remote tailscale serve off: %s: %w", strings.TrimSpace(string(out)), err)
	}
	return nil
}

// Available returns true if the tailscale CLI is installed.
func Available() bool {
	_, err := exec.LookPath("tailscale")
	return err == nil
}

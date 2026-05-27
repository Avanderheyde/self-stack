package tailscale

import (
	"encoding/json"
	"fmt"
	"os/exec"
	"strconv"
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

// TrimDNS strips the trailing dot tailscale status returns on DNS names.
func TrimDNS(dnsName string) string {
	return strings.TrimSuffix(dnsName, ".")
}

// parseServedPorts pulls the TCP port keys out of `tailscale serve status --json`.
// Tailscale's serve status JSON looks like `{"TCP": {"10001": {...}, "10002": {...}}}`.
// Returns nil for unparseable input — callers should treat "unknown" the same
// as "no serves configured" rather than guessing.
func parseServedPorts(data []byte) []int {
	var raw struct {
		TCP map[string]any `json:"TCP"`
	}
	if err := json.Unmarshal(data, &raw); err != nil {
		return nil
	}
	ports := make([]int, 0, len(raw.TCP))
	for k := range raw.TCP {
		if p, err := strconv.Atoi(k); err == nil {
			ports = append(ports, p)
		}
	}
	return ports
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
	return fmt.Sprintf("https://%s:%d", TrimDNS(dnsName), port)
}

// GetStatus checks if Tailscale is running and returns node info.
func GetStatus() (Status, error) {
	out, err := exec.Command("tailscale", "status", "--json").CombinedOutput()
	if err != nil {
		return Status{}, fmt.Errorf("tailscale not running or not installed: %w", err)
	}
	return parseStatus(out)
}

// serveArgs returns the Tailscale Serve flags that expose a single
// localhost port on a dedicated HTTPS port of the tailnet machine. We use
// the explicit --https=<port> form so multiple SelfStack apps can coexist —
// the shorthand `tailscale serve <port>` reuses the default 443 listener,
// which only ever maps one app.
func serveArgs(port int) []string {
	portStr := fmt.Sprintf("%d", port)
	return []string{"serve", "--bg", "--https=" + portStr, "http://localhost:" + portStr}
}

// resetArgs turns off the per-app HTTPS listener installed by serveArgs.
func resetArgs(port int) []string {
	return []string{"serve", "--https=" + fmt.Sprintf("%d", port), "off"}
}

// Serve configures Tailscale Serve to expose a port via HTTPS on the tailnet.
// Returns the Tailscale URL for the app.
func Serve(port int) (string, error) {
	status, err := GetStatus()
	if err != nil {
		return "", err
	}
	cmd := exec.Command("tailscale", serveArgs(port)...)
	if out, err := cmd.CombinedOutput(); err != nil {
		return "", fmt.Errorf("tailscale serve: %s: %w", strings.TrimSpace(string(out)), err)
	}
	return serveURL(status.DNSName, port), nil
}

// ServeRemote configures Tailscale Serve on a remote host via SSH.
func ServeRemote(sshTarget string, port int) (string, error) {
	statusCmd := exec.Command("ssh", sshTarget, "tailscale", "status", "--json")
	out, err := statusCmd.CombinedOutput()
	if err != nil {
		return "", fmt.Errorf("remote tailscale not available: %w", err)
	}
	status, err := parseStatus(out)
	if err != nil {
		return "", err
	}
	args := append([]string{sshTarget, "tailscale"}, serveArgs(port)...)
	serveCmd := exec.Command("ssh", args...)
	if out, err := serveCmd.CombinedOutput(); err != nil {
		return "", fmt.Errorf("remote tailscale serve: %s: %w", strings.TrimSpace(string(out)), err)
	}
	return serveURL(status.DNSName, port), nil
}

// Reset removes the Tailscale Serve configuration for a port.
func Reset(port int) error {
	cmd := exec.Command("tailscale", resetArgs(port)...)
	if out, err := cmd.CombinedOutput(); err != nil {
		return fmt.Errorf("tailscale serve off: %s: %w", strings.TrimSpace(string(out)), err)
	}
	return nil
}

// ResetRemote removes Tailscale Serve on a remote host.
func ResetRemote(sshTarget string, port int) error {
	args := append([]string{sshTarget, "tailscale"}, resetArgs(port)...)
	cmd := exec.Command("ssh", args...)
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

// ServedPorts returns the host ports currently exposed via `tailscale serve`.
// Returns nil when Tailscale is unavailable, not running, or has no serves
// configured — all three cases mean "no remote URL to show."
func ServedPorts() []int {
	out, err := exec.Command("tailscale", "serve", "status", "--json").Output()
	if err != nil {
		return nil
	}
	return parseServedPorts(out)
}

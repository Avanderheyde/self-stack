package cmd

import (
	"context"
	"fmt"
	"net/http"
	"os/exec"
	"strconv"
	"strings"
	"time"

	"github.com/selfstack/selfstack/internal/cloud"
	"github.com/selfstack/selfstack/internal/config"
	"github.com/spf13/cobra"
)

var cloudCmd = &cobra.Command{
	Use:   "cloud",
	Short: "Manage your SelfStack cloud VPS",
}

var cloudInstallURL string

var cloudSetupCmd = &cobra.Command{
	Use:   "setup",
	Short: "Provision a new VPS and configure it for selfstack deploy",
	Long:  "Creates a Hetzner VPS (~$4/mo), installs Docker + Tailscale + selfstack, and configures everything automatically. Zero SSH knowledge required.",
	RunE: func(cmd *cobra.Command, args []string) error {
		ctx := context.Background()

		// Check for existing config
		existing, _ := config.LoadRemoteConfig()
		if existing.IsConfigured() {
			fmt.Printf("Already configured: %s\n", existing.APIURL())
			fmt.Print("Set up a new VPS anyway? [y/N] ")
			var answer string
			fmt.Scanln(&answer)
			if strings.ToLower(strings.TrimSpace(answer)) != "y" {
				return nil
			}
		}

		// Get or prompt for Hetzner API token
		token, err := cloud.LoadToken()
		if err != nil || token == "" {
			fmt.Println("SelfStack Cloud uses Hetzner for hosting (~$4/mo for a 2 vCPU, 4GB RAM VPS).")
			fmt.Println("Get your API token at: https://console.hetzner.cloud/projects → API Tokens")
			fmt.Print("\nHetzner API token: ")
			fmt.Scanln(&token)
			token = strings.TrimSpace(token)
			if token == "" {
				return fmt.Errorf("Hetzner API token is required")
			}
			if err := cloud.SaveToken(token); err != nil {
				return fmt.Errorf("save token: %w", err)
			}
			fmt.Println("  ✓ Token saved")
		} else {
			fmt.Println("Using saved Hetzner API token")
		}

		// Ensure SSH keys are on Hetzner (auto-uploads local key if needed)
		fmt.Println("\nChecking SSH keys...")
		sshKeyIDs, err := cloud.EnsureSSHKey(ctx, token)
		if err != nil {
			fmt.Printf("  ⚠ SSH key issue: %v\n", err)
			fmt.Println("  Continuing without SSH key — root password will be shown")
		} else if len(sshKeyIDs) > 0 {
			fmt.Printf("  ✓ SSH key configured (%d key(s))\n", len(sshKeyIDs))
		} else {
			fmt.Println("  No SSH key found locally (~/.ssh/id_*.pub) or on Hetzner")
			fmt.Println("  Server will be created with a root password (shown once)")
		}

		// Prompt for optional Tailscale auth key
		fmt.Println("\nTailscale enables secure access to your apps from any device.")
		fmt.Println("Get an auth key at: https://login.tailscale.com/admin/settings/keys")
		fmt.Print("Tailscale auth key (press Enter to skip): ")
		var tsKey string
		fmt.Scanln(&tsKey)
		tsKey = strings.TrimSpace(tsKey)

		// Check for existing selfstack servers
		servers, _ := cloud.ListServers(ctx, token)
		serverName := "selfstack"
		if len(servers) > 0 {
			serverName = fmt.Sprintf("selfstack-%d", len(servers)+1)
			fmt.Printf("\nFound %d existing selfstack server(s). New server: %s\n", len(servers), serverName)
		}

		// Provision
		cfg := cloud.DefaultServerConfig()
		fmt.Printf("\nProvisioning %s (%s, %s)...\n", serverName, cfg.ServerType, cfg.Location)

		if cloudInstallURL != "" {
			fmt.Printf("Using custom install script: %s\n", cloudInstallURL)
		}
		userData := cloud.SetupScriptWithInstallURL(tsKey, cloudInstallURL)
		result, err := cloud.ProvisionServer(ctx, token, serverName, userData, sshKeyIDs)
		if err != nil {
			return fmt.Errorf("provisioning failed: %w", err)
		}
		fmt.Printf("  ✓ Server created: %s (ID: %d)\n", result.IPv4, result.ServerID)

		if result.RootPass != "" {
			fmt.Printf("  Root password: %s\n", result.RootPass)
			fmt.Println("  (Save this — it won't be shown again)")
		}

		// Wait for server to be running
		fmt.Println("  Waiting for server to start...")
		if err := cloud.WaitForReady(ctx, token, result.ServerID, 2*time.Minute); err != nil {
			return fmt.Errorf("server did not start: %w", err)
		}
		fmt.Println("  ✓ Server running")

		// Persist the remote config as soon as the VPS exists. If the
		// cloud-init script later times out, `selfstack cloud status` and
		// `selfstack cloud destroy` still work without a recovery dance.
		sshTarget := "root@" + result.IPv4
		rc := config.RemoteConfig{
			Host:     sshTarget,
			APIPort:  config.DefaultPort,
			ServerID: cloud.ServerIDStr(result.ServerID),
		}
		if err := config.SaveRemoteConfig(rc); err != nil {
			return fmt.Errorf("save config: %w", err)
		}

		// Wait for cloud-init to complete (setup script)
		fmt.Println("  Installing Docker, Tailscale, selfstack (1-2 min)...")
		setupDone := false
		deadline := time.Now().Add(5 * time.Minute)
		for time.Now().Before(deadline) {
			check := exec.Command("ssh",
				"-o", "ConnectTimeout=5",
				"-o", "StrictHostKeyChecking=no",
				"-o", "BatchMode=yes",
				sshTarget, "test -f /root/.selfstack-setup-done && echo done",
			)
			if out, err := check.CombinedOutput(); err == nil && strings.TrimSpace(string(out)) == "done" {
				setupDone = true
				break
			}
			time.Sleep(10 * time.Second)
		}
		if !setupDone {
			fmt.Println("  ⚠ Setup didn't finish within 5 minutes.")
			fmt.Println("    The server is provisioned and your config is saved — you can recover with:")
			fmt.Printf("      ssh %s tail -f /var/log/cloud-init-output.log   # watch install\n", sshTarget)
			fmt.Println("      selfstack cloud status                              # check health")
			fmt.Println("    Re-run 'selfstack cloud setup' only if you want a fresh VPS.")
		} else {
			fmt.Println("  ✓ Setup complete")
		}

		// Verify selfstack serve is running
		fmt.Println("  Checking selfstack serve...")
		serveCheck := exec.Command("ssh",
			"-o", "ConnectTimeout=5",
			"-o", "StrictHostKeyChecking=no",
			sshTarget,
			fmt.Sprintf("curl -sf http://localhost:%d/api/status", config.DefaultPort),
		)
		if _, err := serveCheck.CombinedOutput(); err != nil {
			fmt.Println("  ⚠ selfstack serve not responding yet — may need a minute")
		} else {
			fmt.Println("  ✓ selfstack serve running")
		}

		fmt.Printf("\n✓ SelfStack Cloud ready!\n")
		fmt.Printf("  VPS:    %s (%s)\n", result.IPv4, serverName)
		fmt.Printf("  API:    %s\n", rc.APIURL())
		fmt.Printf("  Config: %s/remote.yml\n", config.HomeDir())
		fmt.Println("\nRun 'selfstack deploy' in any project directory to deploy.")
		return nil
	},
}

var cloudDestroyCmd = &cobra.Command{
	Use:   "destroy",
	Short: "Destroy your SelfStack cloud VPS",
	RunE: func(cmd *cobra.Command, args []string) error {
		ctx := context.Background()

		rc, _ := config.LoadRemoteConfig()
		if !rc.IsConfigured() || rc.ServerID == "" {
			return fmt.Errorf("no cloud VPS configured — run 'selfstack cloud setup' first")
		}

		token, err := cloud.LoadToken()
		if err != nil || token == "" {
			return fmt.Errorf("Hetzner API token not found — run 'selfstack cloud setup'")
		}

		fmt.Printf("This will permanently destroy your VPS at %s and all data on it.\n", rc.Hostname())
		fmt.Print("Type 'destroy' to confirm: ")
		var confirm string
		fmt.Scanln(&confirm)
		if strings.TrimSpace(confirm) != "destroy" {
			fmt.Println("Cancelled.")
			return nil
		}

		serverID, err := parseServerID(rc.ServerID)
		if err != nil {
			return err
		}

		fmt.Println("Destroying VPS...")
		if err := cloud.DestroyServer(ctx, token, serverID); err != nil {
			return fmt.Errorf("destroy failed: %w", err)
		}

		// Clear config
		config.SaveRemoteConfig(config.RemoteConfig{})
		fmt.Println("✓ VPS destroyed and config cleared")
		return nil
	},
}

var cloudStatusCmd = &cobra.Command{
	Use:   "status",
	Short: "Show your SelfStack cloud VPS status",
	RunE: func(cmd *cobra.Command, args []string) error {
		ctx := context.Background()

		rc, _ := config.LoadRemoteConfig()
		activeID := ""
		if rc.IsConfigured() {
			activeID = rc.ServerID
			fmt.Printf("Active remote: %s\n", rc.SSHTarget())
			fmt.Printf("  API:    %s\n", rc.APIURL())
			fmt.Printf("  Config: %s/remote.yml\n", config.HomeDir())
			fmt.Printf("  Health: %s\n", probeRemoteHealth(rc.APIURL()))
		} else {
			fmt.Println("No remote configured. Run 'selfstack cloud setup' to provision a VPS.")
		}

		token, err := cloud.LoadToken()
		if err != nil || token == "" {
			if rc.IsConfigured() {
				// Have a remote but no Hetzner token — still useful info.
				fmt.Println("\n(Hetzner API token not saved — can't list inventory. Run 'selfstack cloud setup' to save one.)")
				return nil
			}
			return fmt.Errorf("Hetzner API token not found — run 'selfstack cloud setup'")
		}

		servers, err := cloud.ListServers(ctx, token)
		if err != nil {
			return fmt.Errorf("list servers: %w", err)
		}

		if len(servers) == 0 {
			fmt.Println("\nNo selfstack-managed servers found on Hetzner.")
			fmt.Println("Run 'selfstack cloud setup' to provision one.")
			return nil
		}

		fmt.Printf("\nSelfStack servers on Hetzner (%d):\n", len(servers))
		for _, s := range servers {
			marker := "  "
			if activeID != "" && strconv.FormatInt(s.ID, 10) == activeID {
				marker = "* "
			}
			fmt.Printf("%s%s — %s (%s) [ID: %d]\n", marker, s.Name, s.IPv4, s.Status, s.ID)
		}
		if activeID != "" {
			fmt.Println("\n* = active remote (saved in remote.yml)")
		}
		return nil
	},
}

// probeRemoteHealth does a fast HEAD/GET to the API and returns a one-word
// status the user can read at a glance. Times out quickly so 'cloud status'
// stays snappy even when the VPS is unreachable.
func probeRemoteHealth(apiURL string) string {
	client := http.Client{Timeout: 3 * time.Second}
	resp, err := client.Get(apiURL + "/api/status")
	if err != nil {
		return "unreachable (" + shortErr(err) + ")"
	}
	defer resp.Body.Close()
	if resp.StatusCode != 200 {
		return fmt.Sprintf("error (HTTP %d)", resp.StatusCode)
	}
	return "ok"
}

func shortErr(err error) string {
	msg := err.Error()
	// Strip the verbose `Get "url":` prefix net/http adds.
	if i := strings.LastIndex(msg, ": "); i >= 0 && i < len(msg)-2 {
		return msg[i+2:]
	}
	return msg
}

func init() {
	cloudSetupCmd.Flags().StringVar(&cloudInstallURL, "install-url", "", "Install script URL for cloud-init (defaults to SELFSTACK_INSTALL_URL or release installer)")
	cloudCmd.AddCommand(cloudSetupCmd)
	cloudCmd.AddCommand(cloudDestroyCmd)
	cloudCmd.AddCommand(cloudStatusCmd)
	rootCmd.AddCommand(cloudCmd)
}

func parseServerID(s string) (int64, error) {
	id, err := strconv.ParseInt(strings.TrimSpace(s), 10, 64)
	if err != nil {
		return 0, fmt.Errorf("invalid server ID %q: %w", s, err)
	}
	return id, nil
}

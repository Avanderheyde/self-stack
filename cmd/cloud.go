package cmd

import (
	"context"
	"fmt"
	"os/exec"
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

		userData := cloud.SetupScript(tsKey)
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

		// Wait for cloud-init to complete (setup script)
		fmt.Println("  Installing Docker, Tailscale, selfstack (1-2 min)...")
		sshTarget := "root@" + result.IPv4
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
			fmt.Println("  ⚠ Setup may still be running. Check with:")
			fmt.Printf("    ssh %s tail -f /var/log/cloud-init-output.log\n", sshTarget)
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

		// Save remote config
		rc := config.RemoteConfig{
			Host:     sshTarget,
			APIPort:  config.DefaultPort,
			ServerID: cloud.ServerIDStr(result.ServerID),
		}
		if err := config.SaveRemoteConfig(rc); err != nil {
			return fmt.Errorf("save config: %w", err)
		}

		fmt.Printf("\n✓ SelfStack Cloud ready!\n")
		fmt.Printf("  VPS: %s (%s)\n", result.IPv4, serverName)
		fmt.Printf("  API: %s\n", rc.APIURL())
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

		token, err := cloud.LoadToken()
		if err != nil || token == "" {
			return fmt.Errorf("Hetzner API token not found — run 'selfstack cloud setup'")
		}

		servers, err := cloud.ListServers(ctx, token)
		if err != nil {
			return fmt.Errorf("list servers: %w", err)
		}

		if len(servers) == 0 {
			fmt.Println("No selfstack servers found.")
			fmt.Println("Run 'selfstack cloud setup' to provision one.")
			return nil
		}

		fmt.Printf("SelfStack servers (%d):\n", len(servers))
		for _, s := range servers {
			fmt.Printf("  %s — %s (%s) [ID: %d]\n", s.Name, s.IPv4, s.Status, s.ID)
		}
		return nil
	},
}

func init() {
	cloudCmd.AddCommand(cloudSetupCmd)
	cloudCmd.AddCommand(cloudDestroyCmd)
	cloudCmd.AddCommand(cloudStatusCmd)
	rootCmd.AddCommand(cloudCmd)
}

func parseServerID(s string) (int64, error) {
	id, err := fmt.Sscanf(s, "%d", new(int64))
	if err != nil || id == 0 {
		return 0, fmt.Errorf("invalid server ID: %s", s)
	}
	var result int64
	fmt.Sscanf(s, "%d", &result)
	return result, nil
}

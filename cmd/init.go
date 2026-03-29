package cmd

import (
	"fmt"
	"os"
	"os/exec"
	"strings"

	"github.com/selfstack/selfstack/internal/config"
	"github.com/spf13/cobra"
)

var initCmd = &cobra.Command{
	Use:   "init",
	Short: "Set up a VPS for selfstack deploy",
	Long:  "Connects to a VPS via SSH, verifies Docker and Tailscale are running, and stores the connection for future deploys.",
	RunE: func(cmd *cobra.Command, args []string) error {
		// Check if already configured
		existing, _ := config.LoadRemoteConfig()
		if existing.IsConfigured() {
			fmt.Printf("Already configured: %s (API: %s)\n", existing.SSHTarget(), existing.APIURL())
			fmt.Print("Reconfigure? [y/N] ")
			var answer string
			fmt.Scanln(&answer)
			if strings.ToLower(strings.TrimSpace(answer)) != "y" {
				return nil
			}
		}

		// Prompt for VPS connection
		fmt.Print("VPS SSH connection (e.g., root@1.2.3.4): ")
		var host string
		fmt.Scanln(&host)
		host = strings.TrimSpace(host)
		if host == "" {
			return fmt.Errorf("SSH connection is required")
		}

		rc := config.RemoteConfig{
			Host:    host,
			APIPort: config.DefaultPort,
		}

		// Test SSH connection
		fmt.Printf("Testing SSH connection to %s...\n", rc.SSHTarget())
		sshTest := exec.Command("ssh", "-o", "ConnectTimeout=10", "-o", "BatchMode=yes", rc.SSHTarget(), "echo ok")
		if out, err := sshTest.CombinedOutput(); err != nil {
			return fmt.Errorf("SSH connection failed: %s\nMake sure you can SSH to %s with key-based auth", strings.TrimSpace(string(out)), rc.SSHTarget())
		}
		fmt.Println("  ✓ SSH connection works")

		// Check Docker
		fmt.Println("Checking Docker...")
		dockerCheck := exec.Command("ssh", rc.SSHTarget(), "docker info --format '{{.ServerVersion}}'")
		if out, err := dockerCheck.CombinedOutput(); err != nil {
			return fmt.Errorf("Docker not found on VPS: %s\nInstall Docker: https://docs.docker.com/engine/install/", strings.TrimSpace(string(out)))
		} else {
			fmt.Printf("  ✓ Docker %s\n", strings.TrimSpace(string(out)))
		}

		// Check Docker Compose plugin
		composeCheck := exec.Command("ssh", rc.SSHTarget(), "docker compose version --short")
		if out, err := composeCheck.CombinedOutput(); err != nil {
			return fmt.Errorf("Docker Compose plugin not found: %s\nInstall: apt install docker-compose-plugin", strings.TrimSpace(string(out)))
		} else {
			fmt.Printf("  ✓ Docker Compose %s\n", strings.TrimSpace(string(out)))
		}

		// Check Tailscale (optional)
		fmt.Println("Checking Tailscale...")
		tsCheck := exec.Command("ssh", rc.SSHTarget(), "tailscale status --json 2>/dev/null | head -1")
		if _, err := tsCheck.CombinedOutput(); err != nil {
			fmt.Println("  ⚠ Tailscale not found — apps will be accessible via LAN/VPN only")
			fmt.Println("    Install: https://tailscale.com/download/linux")
		} else {
			fmt.Println("  ✓ Tailscale running")
		}

		// Check if selfstack is installed on VPS
		fmt.Println("Checking selfstack on VPS...")
		ssCheck := exec.Command("ssh", rc.SSHTarget(), "selfstack status 2>/dev/null")
		if _, err := ssCheck.CombinedOutput(); err != nil {
			fmt.Println("  selfstack not found on VPS")
			fmt.Print("  Install selfstack on VPS? [Y/n] ")
			var answer string
			fmt.Scanln(&answer)
			if strings.ToLower(strings.TrimSpace(answer)) != "n" {
				fmt.Println("  Installing selfstack on VPS...")
				installCmd := exec.Command("ssh", rc.SSHTarget(), "curl -fsSL https://selfstack.dev/install | sh")
				installCmd.Stdout = os.Stdout
				installCmd.Stderr = os.Stderr
				if err := installCmd.Run(); err != nil {
					return fmt.Errorf("selfstack installation failed: %w\nInstall manually: curl -fsSL https://selfstack.dev/install | sh", err)
				}
				fmt.Println("  ✓ selfstack installed")
			}
		} else {
			fmt.Println("  ✓ selfstack found")
		}

		// Check if selfstack serve is running
		fmt.Println("Checking selfstack serve...")
		serveCheck := exec.Command("ssh", rc.SSHTarget(), fmt.Sprintf("curl -sf http://localhost:%d/api/status", rc.APIPort))
		if _, err := serveCheck.CombinedOutput(); err != nil {
			fmt.Println("  selfstack serve is not running")
			fmt.Println("  Start it with: ssh", rc.SSHTarget(), "selfstack serve &")
			fmt.Println("  Or set up as a systemd service for persistence")
		} else {
			fmt.Println("  ✓ selfstack serve is running")
		}

		// Save config
		if err := config.SaveRemoteConfig(rc); err != nil {
			return fmt.Errorf("save config: %w", err)
		}
		fmt.Printf("\n✓ VPS configured: %s\n", rc.SSHTarget())
		fmt.Printf("  API: %s\n", rc.APIURL())
		fmt.Println("\nRun 'selfstack deploy' in any project directory to deploy.")
		return nil
	},
}

func init() { rootCmd.AddCommand(initCmd) }

package cmd

import (
	"encoding/json"
	"fmt"
	"io"
	"net/http"

	"github.com/selfstack/selfstack/internal/config"
	"github.com/selfstack/selfstack/internal/tailscale"
	"github.com/spf13/cobra"
)

var undeployCmd = &cobra.Command{
	Use:   "undeploy [app-name]",
	Short: "Remove a deployed app and clean up Tailscale Serve",
	Args:  cobra.ExactArgs(1),
	RunE: func(cmd *cobra.Command, args []string) error {
		appName := args[0]

		rc, _ := config.LoadRemoteConfig()
		targetAPI := apiURL
		if rc.IsConfigured() {
			targetAPI = rc.APIURL()
		}

		// Get app info to know the port for Tailscale cleanup
		resp, err := http.Get(targetAPI + "/api/apps/" + appName)
		if err != nil {
			return fmt.Errorf("failed to reach SelfStack API: %w", err)
		}
		defer resp.Body.Close()
		if resp.StatusCode == 404 {
			return fmt.Errorf("app %q not found", appName)
		}

		body, _ := io.ReadAll(resp.Body)
		var appInfo struct {
			HostPort int `json:"HostPort"`
		}
		json.Unmarshal(body, &appInfo)

		fmt.Printf("Removing %s...\n", appName)

		// Remove via API (reuses existing remove handler)
		req, _ := http.NewRequest("DELETE", targetAPI+"/api/apps/"+appName, nil)
		delResp, err := http.DefaultClient.Do(req)
		if err != nil {
			return fmt.Errorf("remove failed: %w", err)
		}
		delResp.Body.Close()

		// Clean up Tailscale Serve
		if appInfo.HostPort > 0 {
			if rc.IsConfigured() {
				if err := tailscale.ResetRemote(rc.SSHTarget(), appInfo.HostPort); err != nil {
					fmt.Printf("  ⚠ Could not remove Tailscale Serve for port %d: %v\n", appInfo.HostPort, err)
				} else {
					fmt.Printf("  ✓ Tailscale Serve removed for port %d\n", appInfo.HostPort)
				}
			} else if tailscale.Available() {
				if err := tailscale.Reset(appInfo.HostPort); err != nil {
					fmt.Printf("  ⚠ Could not remove Tailscale Serve for port %d: %v\n", appInfo.HostPort, err)
				} else {
					fmt.Printf("  ✓ Tailscale Serve removed for port %d\n", appInfo.HostPort)
				}
			}
		}

		fmt.Printf("✓ %s removed\n", appName)
		return nil
	},
}

func init() { rootCmd.AddCommand(undeployCmd) }

package cmd

import (
	"bufio"
	"bytes"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"time"

	"github.com/selfstack/selfstack/internal/config"
	"github.com/selfstack/selfstack/internal/detect"
	"github.com/selfstack/selfstack/internal/tailscale"
	"github.com/spf13/cobra"
)

var deployName string
var deployEnvFile string

var deployCmd = &cobra.Command{
	Use:   "deploy",
	Short: "Deploy the current directory as a SelfStack app",
	Long:  "Auto-detects the project type, generates Docker configuration if needed, and deploys to your SelfStack server.",
	RunE: func(cmd *cobra.Command, args []string) error {
		dir, err := os.Getwd()
		if err != nil {
			return fmt.Errorf("get working directory: %w", err)
		}

		appName := deployName
		if appName == "" {
			appName = detect.SlugifyDirName(filepath.Base(dir))
		}
		if err := detect.ValidateAppName(appName); err != nil {
			return fmt.Errorf("invalid app name: %w\nUse --name to specify a valid name", err)
		}

		envVars := make(map[string]string)
		envFile := deployEnvFile
		if envFile == "" {
			envFile = filepath.Join(dir, ".env")
		}
		if _, err := os.Stat(envFile); err == nil {
			vars, err := parseEnvFile(envFile)
			if err != nil {
				return fmt.Errorf("parse .env file: %w", err)
			}
			envVars = vars
			fmt.Printf("Loaded %d environment variables from %s\n", len(envVars), envFile)
		}

		pt := detect.DetectProjectType(dir)
		if pt == detect.TypeUnknown {
			return fmt.Errorf("cannot detect project type in %s\nAdd a Dockerfile, docker-compose.yml, package.json, go.mod, or requirements.txt", dir)
		}
		fmt.Printf("Detected %s project: %s\n", pt, appName)

		// Determine target: remote VPS or local
		rc, _ := config.LoadRemoteConfig()
		targetAPI := apiURL
		deployAppDir := dir
		if rc.IsConfigured() {
			targetAPI = rc.APIURL()
		}

		// Verify the API is reachable before doing expensive work like rsync.
		if err := pingAPI(targetAPI); err != nil {
			if rc.IsConfigured() {
				return fmt.Errorf("cannot reach SelfStack API at %s: %w\nIs `selfstack serve` running on %s? (try: ssh %s 'selfstack serve &')", targetAPI, err, rc.Hostname(), rc.SSHTarget())
			}
			return fmt.Errorf("cannot reach SelfStack API at %s: %w\nStart it with: selfstack serve", targetAPI, err)
		}

		if rc.IsConfigured() {
			// Resolve the remote user's home so we can send the API a real
			// absolute path it can stat. The previous version sent the local
			// CWD, which fails os.Stat on the VPS.
			remoteHome, err := resolveRemoteHome(rc.SSHTarget())
			if err != nil {
				return fmt.Errorf("resolve remote home: %w", err)
			}
			deployAppDir = filepath.Join(remoteHome, ".selfstack", "apps", appName)

			fmt.Printf("Syncing to %s:%s\n", rc.SSHTarget(), deployAppDir)
			mkdirCmd := exec.Command("ssh", rc.SSHTarget(), "mkdir", "-p", deployAppDir)
			if out, err := mkdirCmd.CombinedOutput(); err != nil {
				return fmt.Errorf("create remote dir: %s: %w", string(out), err)
			}

			rsyncCmd := exec.Command("rsync", "-az", "--delete",
				"--exclude", "node_modules",
				"--exclude", ".git",
				"--exclude", "__pycache__",
				"--exclude", ".venv",
				"--exclude", "venv",
				"--exclude", ".next",
				"--exclude", "dist",
				"--exclude", "build",
				"--exclude", "target",
				"--exclude", ".DS_Store",
				dir+"/", rc.SSHTarget()+":"+deployAppDir+"/",
			)
			rsyncCmd.Stdout = os.Stdout
			rsyncCmd.Stderr = os.Stderr
			if err := rsyncCmd.Run(); err != nil {
				return fmt.Errorf("rsync to VPS failed: %w", err)
			}
			fmt.Println("  ✓ Files synced")
		}

		body, _ := json.Marshal(map[string]any{
			"name":     appName,
			"app_dir":  deployAppDir,
			"env_vars": envVars,
		})
		resp, err := http.Post(targetAPI+"/api/apps/deploy", "application/json", bytes.NewReader(body))
		if err != nil {
			return fmt.Errorf("failed to reach SelfStack API at %s: %w", targetAPI, err)
		}
		defer resp.Body.Close()

		if resp.StatusCode != 200 {
			out, _ := io.ReadAll(resp.Body)
			return fmt.Errorf("deploy request failed (%d): %s", resp.StatusCode, strings.TrimSpace(string(out)))
		}

		port, err := streamDeployProgress(resp.Body, appName)
		if err != nil {
			return err
		}

		// Print URLs based on deploy target.
		if rc.IsConfigured() {
			fmt.Printf("\n✓ %s deployed to %s on port %d\n", appName, rc.Hostname(), port)
			fmt.Printf("  Direct: http://%s:%d\n", rc.Hostname(), port)
			tsURL, err := tailscale.ServeRemote(rc.SSHTarget(), port)
			if err != nil {
				fmt.Printf("  ⚠ Tailscale Serve not configured: %v\n", err)
				fmt.Println("    App still accessible via the Direct URL above.")
			} else {
				fmt.Printf("  Tailscale: %s\n", tsURL)
			}
		} else {
			fmt.Printf("\n✓ %s deployed locally on port %d\n", appName, port)
			fmt.Printf("  Local: http://%s.selfstack.local\n", appName)
			fmt.Printf("  Direct: http://localhost:%d\n", port)
			if tailscale.Available() {
				tsURL, err := tailscale.Serve(port)
				if err != nil {
					fmt.Printf("  ⚠ Tailscale Serve not configured: %v\n", err)
				} else {
					fmt.Printf("  Tailscale: %s\n", tsURL)
				}
			}
		}
		return nil
	},
}

func init() {
	deployCmd.Flags().StringVar(&deployName, "name", "", "App name (defaults to directory name)")
	deployCmd.Flags().StringVar(&deployEnvFile, "env-file", "", "Path to .env file (defaults to .env in project root)")
	rootCmd.AddCommand(deployCmd)
}

// streamDeployProgress consumes the SSE response, prints each step, and returns
// the host port the app was bound to once a `done: true` event arrives.
// Returns an error if the stream ends before a done event (silent truncation
// would otherwise look like success and leak the wrong URL to the user).
func streamDeployProgress(body io.Reader, appName string) (int, error) {
	scanner := bufio.NewScanner(body)
	// Default 64KB line limit isn't enough for `docker compose build` output
	// when an error event embeds the full build log. Match install.go.
	scanner.Buffer(make([]byte, 64*1024), 1024*1024)

	for scanner.Scan() {
		line := scanner.Text()
		if !strings.HasPrefix(line, "data: ") {
			continue
		}
		var event map[string]any
		if err := json.Unmarshal([]byte(line[6:]), &event); err != nil {
			continue
		}
		if errMsg, ok := event["error"].(string); ok {
			return 0, fmt.Errorf("deploy failed: %s", errMsg)
		}
		if step, ok := event["step"].(string); ok {
			fmt.Printf("  → %s\n", step)
		}
		done, _ := event["done"].(bool)
		if !done {
			continue
		}
		app, ok := event["app"].(map[string]any)
		if !ok {
			return 0, fmt.Errorf("deploy succeeded but server omitted app details")
		}
		hostPort, ok := app["HostPort"].(float64)
		if !ok {
			return 0, fmt.Errorf("deploy succeeded but server omitted HostPort")
		}
		return int(hostPort), nil
	}
	if err := scanner.Err(); err != nil {
		return 0, fmt.Errorf("read deploy stream: %w", err)
	}
	return 0, fmt.Errorf("deploy stream ended without completion event (check `selfstack logs %s`)", appName)
}

// pingAPI performs a fast HEAD-equivalent request to /api/status so we can
// fail fast with a clear message before doing expensive work like rsync.
func pingAPI(baseURL string) error {
	client := &http.Client{Timeout: 5 * time.Second}
	resp, err := client.Get(baseURL + "/api/status")
	if err != nil {
		return err
	}
	defer resp.Body.Close()
	if resp.StatusCode != 200 {
		return fmt.Errorf("unexpected status %d", resp.StatusCode)
	}
	return nil
}

// resolveRemoteHome SSHes to the target and prints $HOME so deploys can
// build an absolute path the remote API can stat. Without this the CLI
// sends the local CWD and the remote os.Stat fails.
func resolveRemoteHome(sshTarget string) (string, error) {
	cmd := exec.Command("ssh", "-o", "BatchMode=yes", "-o", "ConnectTimeout=10", sshTarget, "printf %s \"$HOME\"")
	out, err := cmd.Output()
	if err != nil {
		return "", fmt.Errorf("ssh resolve $HOME: %w", err)
	}
	home := strings.TrimSpace(string(out))
	if home == "" {
		return "", fmt.Errorf("remote $HOME is empty")
	}
	return home, nil
}

// parseEnvFile reads a .env file and returns key-value pairs.
// Supports comments (#), empty lines, and quoted values.
func parseEnvFile(path string) (map[string]string, error) {
	f, err := os.Open(path)
	if err != nil {
		return nil, err
	}
	defer f.Close()

	vars := make(map[string]string)
	scanner := bufio.NewScanner(f)
	for scanner.Scan() {
		line := strings.TrimSpace(scanner.Text())
		if line == "" || strings.HasPrefix(line, "#") {
			continue
		}
		idx := strings.IndexByte(line, '=')
		if idx < 0 {
			continue
		}
		key := strings.TrimSpace(line[:idx])
		val := strings.TrimSpace(line[idx+1:])
		if len(val) >= 2 && ((val[0] == '"' && val[len(val)-1] == '"') || (val[0] == '\'' && val[len(val)-1] == '\'')) {
			val = val[1 : len(val)-1]
		}
		vars[key] = val
	}
	return vars, scanner.Err()
}

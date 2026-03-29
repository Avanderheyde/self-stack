package cmd

import (
	"bufio"
	"bytes"
	"encoding/json"
	"fmt"
	"net/http"
	"os"
	"os/exec"
	"path/filepath"
	"strings"

	"github.com/selfstack/selfstack/internal/config"
	"github.com/selfstack/selfstack/internal/detect"
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

		// Derive app name from directory if not specified
		appName := deployName
		if appName == "" {
			appName = detect.SlugifyDirName(filepath.Base(dir))
		}
		if err := detect.ValidateAppName(appName); err != nil {
			return fmt.Errorf("invalid app name: %w\nUse --name to specify a valid name", err)
		}

		// Load .env file if present
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

		// Check project type locally first for fast feedback
		pt := detect.DetectProjectType(dir)
		if pt == detect.TypeUnknown {
			return fmt.Errorf("cannot detect project type in %s\nAdd a Dockerfile, docker-compose.yml, package.json, go.mod, or requirements.txt", dir)
		}
		fmt.Printf("Detected %s project: %s\n", pt, appName)

		// Determine target: remote VPS or local
		rc, _ := config.LoadRemoteConfig()
		targetAPI := apiURL
		if rc.IsConfigured() {
			targetAPI = rc.APIURL()
			// rsync files to VPS
			remoteDir := fmt.Sprintf("~/.selfstack/apps/%s/", appName)
			fmt.Printf("Syncing to %s:%s\n", rc.SSHTarget(), remoteDir)

			mkdirCmd := exec.Command("ssh", rc.SSHTarget(), "mkdir", "-p", remoteDir)
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
				dir+"/", rc.SSHTarget()+":"+remoteDir,
			)
			rsyncCmd.Stdout = os.Stdout
			rsyncCmd.Stderr = os.Stderr
			if err := rsyncCmd.Run(); err != nil {
				return fmt.Errorf("rsync to VPS failed: %w", err)
			}
			fmt.Println("  ✓ Files synced")
		}

		// POST to the deploy API (local or remote)
		body, _ := json.Marshal(map[string]any{
			"name":     appName,
			"app_dir":  dir,
			"env_vars": envVars,
		})
		resp, err := http.Post(targetAPI+"/api/apps/deploy", "application/json", bytes.NewReader(body))
		if err != nil {
			return fmt.Errorf("failed to reach SelfStack API at %s: %w\nIs 'selfstack serve' running?", targetAPI, err)
		}
		defer resp.Body.Close()

		// Stream SSE progress
		scanner := bufio.NewScanner(resp.Body)
		for scanner.Scan() {
			line := scanner.Text()
			if !strings.HasPrefix(line, "data: ") {
				continue
			}
			data := line[6:]
			var event map[string]any
			if err := json.Unmarshal([]byte(data), &event); err != nil {
				continue
			}
			if step, ok := event["step"].(string); ok {
				fmt.Printf("  → %s\n", step)
			}
			if errMsg, ok := event["error"].(string); ok {
				return fmt.Errorf("deploy failed: %s", errMsg)
			}
			if done, ok := event["done"].(bool); ok && done {
				if app, ok := event["app"].(map[string]any); ok {
					port := int(app["HostPort"].(float64))
					fmt.Printf("\n✓ %s deployed successfully on port %d\n", appName, port)
					fmt.Printf("  Local: http://%s.selfstack.local\n", appName)
					fmt.Printf("  Direct: http://localhost:%d\n", port)
				}
				return nil
			}
		}
		return scanner.Err()
	},
}

func init() {
	deployCmd.Flags().StringVar(&deployName, "name", "", "App name (defaults to directory name)")
	deployCmd.Flags().StringVar(&deployEnvFile, "env-file", "", "Path to .env file (defaults to .env in project root)")
	rootCmd.AddCommand(deployCmd)
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
		// Strip surrounding quotes
		if len(val) >= 2 && ((val[0] == '"' && val[len(val)-1] == '"') || (val[0] == '\'' && val[len(val)-1] == '\'')) {
			val = val[1 : len(val)-1]
		}
		vars[key] = val
	}
	return vars, scanner.Err()
}

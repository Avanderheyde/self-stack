package cmd

import (
	"bufio"
	"bytes"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"path/filepath"
	"strings"

	"github.com/spf13/cobra"
)

var (
	installLocal    string
	installSeedData string
)

// Indirection so tests can simulate filepath.Abs failure — normally only
// reachable when os.Getwd fails on a deleted cwd, unreliable on macOS/Linux.
var installAbsFn = filepath.Abs

var installCmd = &cobra.Command{
	Use:   "install [app-name]",
	Short: "Install an app from the registry (or from a local path with --local)",
	Args:  cobra.ExactArgs(1),
	RunE: func(cmd *cobra.Command, args []string) error {
		// --seed-data only applies to local installs. Without --local the
		// API drops seed_data on the registry-clone path and the user sees
		// a successful install with no data migrated — silent footgun.
		if installSeedData != "" && installLocal == "" {
			return fmt.Errorf("--seed-data requires --local: seeding only applies to local-source installs")
		}
		body := map[string]string{"name": args[0]}
		if installLocal != "" {
			abs, err := installAbsFn(installLocal)
			if err != nil {
				return fmt.Errorf("resolve --local path: %w", err)
			}
			body["local_path"] = abs
		}
		if installSeedData != "" {
			abs, err := installAbsFn(installSeedData)
			if err != nil {
				return fmt.Errorf("resolve --seed-data path: %w", err)
			}
			body["seed_data"] = abs
		}
		buf, _ := json.Marshal(body)
		resp, err := http.Post(apiURL+"/api/apps/install", "application/json", bytes.NewReader(buf))
		if err != nil {
			return fmt.Errorf("failed to reach SelfStack API: %w", err)
		}
		defer resp.Body.Close()

		if resp.StatusCode != 200 {
			out, _ := io.ReadAll(resp.Body)
			return fmt.Errorf("install failed: %s", string(out))
		}

		// API streams Server-Sent Events: one `data: {...}\n\n` per step.
		// Show each step to the user; bail out on `error`; success on `done`.
		scanner := bufio.NewScanner(resp.Body)
		scanner.Buffer(make([]byte, 64*1024), 1024*1024)
		for scanner.Scan() {
			line := scanner.Text()
			if !strings.HasPrefix(line, "data: ") {
				continue
			}
			payload := strings.TrimPrefix(line, "data: ")
			var ev map[string]any
			if err := json.Unmarshal([]byte(payload), &ev); err != nil {
				continue
			}
			if errMsg, ok := ev["error"].(string); ok {
				return fmt.Errorf("install failed: %s", errMsg)
			}
			if step, ok := ev["step"].(string); ok {
				fmt.Printf("  %s\n", step)
			}
			if done, _ := ev["done"].(bool); done {
				fmt.Printf("Installed %s successfully\n", args[0])
				return nil
			}
		}
		if err := scanner.Err(); err != nil {
			return fmt.Errorf("stream read error: %w", err)
		}
		return fmt.Errorf("install stream ended without completion")
	},
}

func init() {
	installCmd.Flags().StringVar(&installLocal, "local", "", "install from a local source directory instead of the registry")
	installCmd.Flags().StringVar(&installSeedData, "seed-data", "", "copy contents of this directory into the app's docker 'data' volume before first start")
	rootCmd.AddCommand(installCmd)
}

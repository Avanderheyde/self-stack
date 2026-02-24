package cmd

import (
	"bytes"
	"encoding/json"
	"fmt"
	"io"
	"net/http"

	"github.com/spf13/cobra"
)

var installCmd = &cobra.Command{
	Use:   "install [app-name]",
	Short: "Install an app from the registry",
	Args:  cobra.ExactArgs(1),
	RunE: func(cmd *cobra.Command, args []string) error {
		body, _ := json.Marshal(map[string]string{"name": args[0]})
		resp, err := http.Post(apiURL+"/api/apps/install", "application/json", bytes.NewReader(body))
		if err != nil {
			return fmt.Errorf("failed to reach SelfStack API: %w", err)
		}
		defer resp.Body.Close()
		out, _ := io.ReadAll(resp.Body)
		if resp.StatusCode != 200 {
			return fmt.Errorf("install failed: %s", string(out))
		}
		if bytes.Contains(out, []byte(`"error"`)) {
			return fmt.Errorf("install failed: %s", string(out))
		}
		fmt.Printf("Installed %s successfully\n", args[0])
		return nil
	},
}

func init() { rootCmd.AddCommand(installCmd) }

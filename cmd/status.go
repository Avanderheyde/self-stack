package cmd

import (
	"encoding/json"
	"fmt"
	"io"
	"net/http"

	"github.com/spf13/cobra"
)

var statusCmd = &cobra.Command{
	Use:   "status",
	Short: "Show SelfStack server status",
	RunE: func(cmd *cobra.Command, args []string) error {
		resp, err := http.Get(apiURL + "/api/status")
		if err != nil {
			return fmt.Errorf("failed to reach SelfStack API: %w", err)
		}
		defer resp.Body.Close()
		out, _ := io.ReadAll(resp.Body)
		if resp.StatusCode != 200 {
			return fmt.Errorf("status failed: %s", string(out))
		}
		var status map[string]any
		json.Unmarshal(out, &status)
		fmt.Printf("SelfStack v%s\n", status["version"])
		fmt.Printf("Status: %s\n", status["status"])
		fmt.Printf("Apps: %.0f\n", status["apps"])
		return nil
	},
}

func init() { rootCmd.AddCommand(statusCmd) }

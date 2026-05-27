package cmd

import (
	"encoding/json"
	"fmt"
	"io"
	"net/http"

	"github.com/spf13/cobra"
)

var listCmd = &cobra.Command{
	Use:   "list",
	Short: "List installed apps",
	RunE: func(cmd *cobra.Command, args []string) error {
		resp, err := http.Get(apiURL + "/api/apps")
		if err != nil {
			return fmt.Errorf("failed to reach SelfStack API: %w", err)
		}
		defer resp.Body.Close()
		out, _ := io.ReadAll(resp.Body)
		if resp.StatusCode != 200 {
			return fmt.Errorf("list failed: %s", string(out))
		}
		var apps []struct {
			Name        string `json:"Name"`
			DisplayName string `json:"DisplayName"`
			Status      string `json:"Status"`
			HostPort    int    `json:"HostPort"`
		}
		json.Unmarshal(out, &apps)
		if len(apps) == 0 {
			fmt.Println("No apps installed")
			return nil
		}
		for _, a := range apps {
			fmt.Printf("  %-20s %-15s %-10s port:%d\n", a.Name, a.DisplayName, a.Status, a.HostPort)
		}
		return nil
	},
}

func init() { rootCmd.AddCommand(listCmd) }

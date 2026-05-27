package cmd

import (
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"net/url"

	"github.com/spf13/cobra"
)

var searchCmd = &cobra.Command{
	Use:   "search [query]",
	Short: "Search the app registry",
	Args:  cobra.ExactArgs(1),
	RunE: func(cmd *cobra.Command, args []string) error {
		resp, err := http.Get(apiURL + "/api/registry/search?q=" + url.QueryEscape(args[0]))
		if err != nil {
			return fmt.Errorf("failed to reach SelfStack API: %w", err)
		}
		defer resp.Body.Close()
		out, _ := io.ReadAll(resp.Body)
		if resp.StatusCode != 200 {
			return fmt.Errorf("search failed: %s", string(out))
		}
		var results []struct {
			Name        string `json:"name"`
			DisplayName string `json:"display_name"`
			Description string `json:"description"`
			Category    string `json:"category"`
		}
		json.Unmarshal(out, &results)
		if len(results) == 0 {
			fmt.Println("No apps found")
			return nil
		}
		for _, r := range results {
			fmt.Printf("  %-20s %-15s %s\n", r.Name, r.Category, r.Description)
		}
		return nil
	},
}

func init() { rootCmd.AddCommand(searchCmd) }

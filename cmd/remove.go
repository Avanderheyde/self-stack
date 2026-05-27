package cmd

import (
	"fmt"
	"io"
	"net/http"

	"github.com/spf13/cobra"
)

var removeCmd = &cobra.Command{
	Use:   "remove [app-name]",
	Short: "Remove an installed app",
	Args:  cobra.ExactArgs(1),
	RunE: func(cmd *cobra.Command, args []string) error {
		req, _ := http.NewRequest("DELETE", apiURL+"/api/apps/"+args[0], nil)
		resp, err := http.DefaultClient.Do(req)
		if err != nil {
			return fmt.Errorf("failed to reach SelfStack API: %w", err)
		}
		defer resp.Body.Close()
		if resp.StatusCode != 200 {
			out, _ := io.ReadAll(resp.Body)
			return fmt.Errorf("remove failed: %s", string(out))
		}
		fmt.Printf("Removed %s\n", args[0])
		return nil
	},
}

func init() { rootCmd.AddCommand(removeCmd) }

package cmd

import (
	"fmt"
	"io"
	"net/http"

	"github.com/spf13/cobra"
)

var stopCmd = &cobra.Command{
	Use:   "stop [app-name]",
	Short: "Stop a running app",
	Args:  cobra.ExactArgs(1),
	RunE: func(cmd *cobra.Command, args []string) error {
		resp, err := http.Post(apiURL+"/api/apps/"+args[0]+"/stop", "", nil)
		if err != nil {
			return fmt.Errorf("failed to reach SelfStack API: %w", err)
		}
		defer resp.Body.Close()
		if resp.StatusCode != 200 {
			out, _ := io.ReadAll(resp.Body)
			return fmt.Errorf("stop failed: %s", string(out))
		}
		fmt.Printf("Stopped %s\n", args[0])
		return nil
	},
}

func init() { rootCmd.AddCommand(stopCmd) }

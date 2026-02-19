package cmd

import (
	"fmt"
	"os"

	"github.com/spf13/cobra"
)

var apiURL string

var rootCmd = &cobra.Command{
	Use:   "selfstack",
	Short: "Personal app server — install, run, and access self-hosted web apps",
}

func Execute() {
	if err := rootCmd.Execute(); err != nil {
		fmt.Fprintln(os.Stderr, err)
		os.Exit(1)
	}
}

func init() {
	rootCmd.PersistentFlags().StringVar(&apiURL, "api", "http://localhost:8080", "SelfStack API URL")
}

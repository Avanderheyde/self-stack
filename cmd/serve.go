package cmd

import (
	"fmt"
	"log"
	"net/http"
	"os"

	"github.com/selfstack/selfstack/internal/api"
	"github.com/selfstack/selfstack/internal/app"
	"github.com/selfstack/selfstack/internal/config"
	"github.com/selfstack/selfstack/internal/registry"
	"github.com/selfstack/selfstack/internal/store"
	"github.com/spf13/cobra"
)

const defaultRegistryURL = "https://raw.githubusercontent.com/selfstack/registry/main/registry.json"

var serveCmd = &cobra.Command{
	Use:   "serve",
	Short: "Start the SelfStack server",
	RunE: func(cmd *cobra.Command, args []string) error {
		for _, dir := range []string{config.AppsDir(), config.DataDir(), config.KeysPath()} {
			os.MkdirAll(dir, 0755)
		}

		s, err := store.Open(config.DBPath())
		if err != nil {
			return fmt.Errorf("open store: %w", err)
		}
		defer s.Close()

		reg := registry.NewClient(defaultRegistryURL)
		appSvc := app.NewService(s, reg)
		srv := api.NewServer(appSvc, reg)

		addr := fmt.Sprintf(":%d", config.DefaultPort)
		log.Printf("SelfStack server starting on %s", addr)
		return http.ListenAndServe(addr, srv)
	},
}

func init() { rootCmd.AddCommand(serveCmd) }

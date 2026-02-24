package cmd

import (
	"context"
	"encoding/json"
	"fmt"
	"log"
	"net/http"
	"os"
	"strings"
	"time"

	"github.com/selfstack/selfstack/internal/api"
	"github.com/selfstack/selfstack/internal/app"
	"github.com/selfstack/selfstack/internal/auth"
	"github.com/selfstack/selfstack/internal/config"
	"github.com/selfstack/selfstack/internal/proxy"
	"github.com/selfstack/selfstack/internal/registry"
	"github.com/selfstack/selfstack/internal/store"
	"github.com/spf13/cobra"
)

const defaultRegistryURL = "https://raw.githubusercontent.com/Avanderheyde/selfstack-registry/main/registry.json"

var servePort int

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

		// Initialize auth
		a, err := auth.New(config.KeysPath())
		if err != nil {
			return fmt.Errorf("init auth: %w", err)
		}
		_ = a // Auth middleware can be wired in when ready

		// Create reverse proxy and register running apps
		p := proxy.New()
		apps, _ := appSvc.List()
		for _, app := range apps {
			if app.Status == "running" {
				p.Register(app.Name, app.HostPort)
			}
		}

		// Start reverse proxy on port 8081 (dev) — use port 80 in production
		go func() {
			proxyAddr := ":8081"
			log.Printf("Reverse proxy on %s", proxyAddr)
			http.ListenAndServe(proxyAddr, p)
		}()

		// Start API + dashboard on main port
		srv := api.NewServer(appSvc, reg, Version)
		port := servePort
		if port == 0 {
			port = config.DefaultPort
		}
		addr := fmt.Sprintf(":%d", port)

		// Non-blocking update check
		go func() {
			ctx, cancel := context.WithTimeout(context.Background(), 3*time.Second)
			defer cancel()
			req, err := http.NewRequestWithContext(ctx, "GET",
				fmt.Sprintf("https://api.github.com/repos/%s/releases/latest", githubRepo),
				nil)
			if err != nil {
				return
			}
			resp, err := http.DefaultClient.Do(req)
			if err != nil || resp.StatusCode != 200 {
				return
			}
			defer resp.Body.Close()
			var rel struct{ TagName string `json:"tag_name"` }
			if json.NewDecoder(resp.Body).Decode(&rel) != nil {
				return
			}
			latest := strings.TrimPrefix(rel.TagName, "v")
			if latest != Version && Version != "dev" {
				log.Printf("A new version of SelfStack is available (%s). Run \"selfstack update\" to upgrade.", rel.TagName)
			}
		}()

		log.Printf("SelfStack dashboard on %s", addr)
		return http.ListenAndServe(addr, srv)
	},
}

func init() {
	serveCmd.Flags().IntVarP(&servePort, "port", "p", 0, "Port to listen on (default 8080)")
	rootCmd.AddCommand(serveCmd)
}

package api

import (
	"encoding/json"
	"fmt"
	"log"
	"net/http"
	"strings"
	"sync"

	"github.com/go-chi/chi/v5"
	"github.com/go-chi/chi/v5/middleware"
	"github.com/selfstack/selfstack/internal/app"
	"github.com/selfstack/selfstack/internal/dashboard"
	"github.com/selfstack/selfstack/internal/portless"
	"github.com/selfstack/selfstack/internal/registry"
	"github.com/selfstack/selfstack/internal/tailscale"
)

type activeOp struct {
	Type string `json:"type"`
	Step string `json:"step"`
	Done bool   `json:"done"`
	Err  string `json:"error,omitempty"`
}

type Server struct {
	appSvc   *app.Service
	registry *registry.Client
	version  string
	router   chi.Router
	ops      map[string]*activeOp
	opsMu    sync.Mutex
}

func NewServer(appSvc *app.Service, reg *registry.Client, version string) *Server {
	s := &Server{appSvc: appSvc, registry: reg, version: version, ops: make(map[string]*activeOp)}
	r := chi.NewRouter()
	r.Use(middleware.Logger)
	r.Use(middleware.Recoverer)

	r.Route("/api", func(r chi.Router) {
		r.Get("/status", s.handleStatus)
		r.Get("/apps", s.handleListApps)
		r.Post("/apps/install", s.handleInstallApp)
		r.Post("/apps/deploy", s.handleDeployApp)
		r.Post("/apps/{name}/start", s.handleStartApp)
		r.Post("/apps/{name}/stop", s.handleStopApp)
		r.Get("/apps/{name}", s.handleGetApp)
		r.Patch("/apps/{name}/port", s.handleUpdatePort)
		r.Post("/apps/{name}/update", s.handleUpdateApp)
		r.Get("/apps/{name}/operation", s.handleGetOperation)
		r.Delete("/apps/{name}", s.handleRemoveApp)
		r.Get("/apps/{name}/logs", s.handleLogs)
		r.Get("/registry/search", s.handleRegistrySearch)
		r.Get("/update/check", s.handleUpdateCheck)
		r.Post("/update/apply", s.handleUpdateApply)
		r.Get("/portless", handlePortlessStatus)
		r.Get("/tailscale", handleTailscaleStatus)
	})

	r.Handle("/*", dashboard.Handler())

	s.router = r
	return s
}

func (s *Server) ServeHTTP(w http.ResponseWriter, r *http.Request) {
	s.router.ServeHTTP(w, r)
}

func jsonResponse(w http.ResponseWriter, status int, data any) {
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(status)
	if err := json.NewEncoder(w).Encode(data); err != nil {
		log.Printf("jsonResponse: encode: %v", err)
	}
}

func jsonError(w http.ResponseWriter, status int, msg string) {
	jsonResponse(w, status, map[string]string{"error": msg})
}

func sseEvent(w http.ResponseWriter, flusher http.Flusher, data any) {
	buf, _ := json.Marshal(data)
	fmt.Fprintf(w, "data: %s\n\n", buf)
	flusher.Flush()
}

func handlePortlessStatus(w http.ResponseWriter, r *http.Request) {
	available := portless.Available()
	port := 0
	if available {
		port = portless.ProxyPort()
	}
	jsonResponse(w, 200, map[string]any{
		"available": available,
		"port":      port,
	})
}

func handleTailscaleStatus(w http.ResponseWriter, r *http.Request) {
	if !tailscale.Available() {
		jsonResponse(w, 200, map[string]any{"available": false})
		return
	}
	status, err := tailscale.GetStatus()
	if err != nil {
		// CLI present but node isn't running — still useful to tell the UI.
		jsonResponse(w, 200, map[string]any{"available": true, "running": false})
		return
	}
	jsonResponse(w, 200, map[string]any{
		"available":    true,
		"running":      status.Running,
		"hostname":     status.Hostname,
		"dns_name":     tailscale.TrimDNS(status.DNSName),
		"ip":           status.IP,
		"served_ports": tailscale.ServedPorts(),
	})
}

func errStatus(err error) int {
	if strings.Contains(err.Error(), "not found") {
		return 404
	}
	return 500
}

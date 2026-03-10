package api

import (
	"encoding/json"
	"fmt"
	"log"
	"net/http"
	"strings"

	"github.com/go-chi/chi/v5"
	"github.com/go-chi/chi/v5/middleware"
	"github.com/selfstack/selfstack/internal/app"
	"github.com/selfstack/selfstack/internal/dashboard"
	"github.com/selfstack/selfstack/internal/registry"
)

type Server struct {
	appSvc   *app.Service
	registry *registry.Client
	version  string
	router   chi.Router
}

func NewServer(appSvc *app.Service, reg *registry.Client, version string) *Server {
	s := &Server{appSvc: appSvc, registry: reg, version: version}
	r := chi.NewRouter()
	r.Use(middleware.Logger)
	r.Use(middleware.Recoverer)

	r.Route("/api", func(r chi.Router) {
		r.Get("/status", s.handleStatus)
		r.Get("/apps", s.handleListApps)
		r.Post("/apps/install", s.handleInstallApp)
		r.Post("/apps/{name}/start", s.handleStartApp)
		r.Post("/apps/{name}/stop", s.handleStopApp)
		r.Get("/apps/{name}", s.handleGetApp)
		r.Patch("/apps/{name}/port", s.handleUpdatePort)
		r.Post("/apps/{name}/update", s.handleUpdateApp)
		r.Delete("/apps/{name}", s.handleRemoveApp)
		r.Get("/registry/search", s.handleRegistrySearch)
		r.Get("/update/check", s.handleUpdateCheck)
		r.Post("/update/apply", s.handleUpdateApply)
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

func errStatus(err error) int {
	if strings.Contains(err.Error(), "not found") {
		return 404
	}
	return 500
}

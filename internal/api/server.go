package api

import (
	"encoding/json"
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
	router   chi.Router
}

func NewServer(appSvc *app.Service, reg *registry.Client) *Server {
	s := &Server{appSvc: appSvc, registry: reg}
	r := chi.NewRouter()
	r.Use(middleware.Logger)
	r.Use(middleware.Recoverer)

	r.Route("/api", func(r chi.Router) {
		r.Get("/status", s.handleStatus)
		r.Get("/apps", s.handleListApps)
		r.Post("/apps/install", s.handleInstallApp)
		r.Post("/apps/{name}/start", s.handleStartApp)
		r.Post("/apps/{name}/stop", s.handleStopApp)
		r.Delete("/apps/{name}", s.handleRemoveApp)
		r.Get("/registry/search", s.handleRegistrySearch)
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

func errStatus(err error) int {
	if strings.Contains(err.Error(), "not found") {
		return 404
	}
	return 500
}

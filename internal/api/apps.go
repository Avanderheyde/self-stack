package api

import (
	"encoding/json"
	"net/http"

	"github.com/go-chi/chi/v5"
	"github.com/selfstack/selfstack/internal/registry"
	"github.com/selfstack/selfstack/internal/store"
)

func (s *Server) handleStatus(w http.ResponseWriter, r *http.Request) {
	apps, err := s.appSvc.List()
	if err != nil {
		jsonError(w, 500, err.Error())
		return
	}
	jsonResponse(w, 200, map[string]any{
		"version": "0.1.0",
		"apps":    len(apps),
		"status":  "ok",
	})
}

func (s *Server) handleListApps(w http.ResponseWriter, r *http.Request) {
	apps, err := s.appSvc.List()
	if err != nil {
		jsonError(w, 500, err.Error())
		return
	}
	if apps == nil {
		apps = []store.App{}
	}
	jsonResponse(w, 200, apps)
}

func (s *Server) handleGetApp(w http.ResponseWriter, r *http.Request) {
	name := chi.URLParam(r, "name")
	a, err := s.appSvc.Get(name)
	if err != nil {
		jsonError(w, errStatus(err), err.Error())
		return
	}
	jsonResponse(w, 200, a)
}

func (s *Server) handleInstallApp(w http.ResponseWriter, r *http.Request) {
	var req struct {
		Name string `json:"name"`
	}
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		jsonError(w, 400, "invalid request body")
		return
	}
	if req.Name == "" {
		jsonError(w, 400, "name is required")
		return
	}
	if err := s.appSvc.Install(r.Context(), req.Name); err != nil {
		jsonError(w, 500, err.Error())
		return
	}
	app, err := s.appSvc.Get(req.Name)
	if err != nil {
		jsonError(w, 500, err.Error())
		return
	}
	jsonResponse(w, 201, app)
}

func (s *Server) handleStartApp(w http.ResponseWriter, r *http.Request) {
	name := chi.URLParam(r, "name")
	if err := s.appSvc.Start(r.Context(), name); err != nil {
		jsonError(w, errStatus(err), err.Error())
		return
	}
	jsonResponse(w, 200, map[string]string{"status": "started"})
}

func (s *Server) handleStopApp(w http.ResponseWriter, r *http.Request) {
	name := chi.URLParam(r, "name")
	if err := s.appSvc.Stop(r.Context(), name); err != nil {
		jsonError(w, errStatus(err), err.Error())
		return
	}
	jsonResponse(w, 200, map[string]string{"status": "stopped"})
}

func (s *Server) handleRemoveApp(w http.ResponseWriter, r *http.Request) {
	name := chi.URLParam(r, "name")
	if err := s.appSvc.Remove(r.Context(), name); err != nil {
		jsonError(w, errStatus(err), err.Error())
		return
	}
	jsonResponse(w, 200, map[string]string{"status": "removed"})
}

func (s *Server) handleRegistrySearch(w http.ResponseWriter, r *http.Request) {
	q := r.URL.Query().Get("q")
	cat := s.registry.Cached()
	if cat == nil {
		var err error
		cat, err = s.registry.Fetch()
		if err != nil {
			jsonError(w, 500, err.Error())
			return
		}
	}
	results := registry.Search(cat, q)
	if results == nil {
		results = []registry.AppEntry{}
	}
	jsonResponse(w, 200, results)
}

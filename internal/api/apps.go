package api

import (
	"encoding/json"
	"net/http"

	"github.com/go-chi/chi/v5"
	"github.com/selfstack/selfstack/internal/app"
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
		"version": s.version,
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

	flusher, ok := w.(http.Flusher)
	if !ok {
		jsonError(w, 500, "streaming not supported")
		return
	}
	w.Header().Set("Content-Type", "text/event-stream")
	w.Header().Set("Cache-Control", "no-cache")
	w.Header().Set("Connection", "keep-alive")

	onProgress := app.ProgressFunc(func(step string) {
		sseEvent(w, flusher, map[string]string{"step": step})
	})

	if err := s.appSvc.Install(r.Context(), req.Name, onProgress); err != nil {
		sseEvent(w, flusher, map[string]string{"error": err.Error()})
		return
	}
	installed, err := s.appSvc.Get(req.Name)
	if err != nil {
		sseEvent(w, flusher, map[string]string{"error": err.Error()})
		return
	}
	sseEvent(w, flusher, map[string]any{"done": true, "app": installed})
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

func (s *Server) handleUpdateApp(w http.ResponseWriter, r *http.Request) {
	name := chi.URLParam(r, "name")

	flusher, ok := w.(http.Flusher)
	if !ok {
		jsonError(w, 500, "streaming not supported")
		return
	}
	w.Header().Set("Content-Type", "text/event-stream")
	w.Header().Set("Cache-Control", "no-cache")
	w.Header().Set("Connection", "keep-alive")

	onProgress := app.ProgressFunc(func(step string) {
		sseEvent(w, flusher, map[string]string{"step": step})
	})

	if err := s.appSvc.Update(r.Context(), name, onProgress); err != nil {
		sseEvent(w, flusher, map[string]string{"error": err.Error()})
		return
	}
	updated, err := s.appSvc.Get(name)
	if err != nil {
		sseEvent(w, flusher, map[string]string{"error": err.Error()})
		return
	}
	sseEvent(w, flusher, map[string]any{"done": true, "app": updated})
}

func (s *Server) handleUpdatePort(w http.ResponseWriter, r *http.Request) {
	name := chi.URLParam(r, "name")
	var req struct {
		Port int `json:"port"`
	}
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		jsonError(w, 400, "invalid request body")
		return
	}
	if err := s.appSvc.UpdatePort(name, req.Port); err != nil {
		jsonError(w, errStatus(err), err.Error())
		return
	}
	jsonResponse(w, 200, map[string]string{"status": "updated"})
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

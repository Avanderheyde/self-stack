package api

import (
	"bufio"
	"context"
	"encoding/json"
	"io"
	"net/http"
	"time"

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

	if err := s.appSvc.Remove(r.Context(), name, onProgress); err != nil {
		sseEvent(w, flusher, map[string]string{"error": err.Error()})
		return
	}
	sseEvent(w, flusher, map[string]any{"done": true})
}

func (s *Server) handleUpdateApp(w http.ResponseWriter, r *http.Request) {
	name := chi.URLParam(r, "name")

	s.opsMu.Lock()
	if existing, ok := s.ops[name]; ok && !existing.Done {
		s.opsMu.Unlock()
		jsonResponse(w, 200, map[string]any{"active": true, "step": existing.Step})
		return
	}
	op := &activeOp{Type: "update"}
	s.ops[name] = op
	s.opsMu.Unlock()

	// Run update in background so it survives client disconnect
	go func() {
		onProgress := app.ProgressFunc(func(step string) {
			s.opsMu.Lock()
			op.Step = step
			s.opsMu.Unlock()
		})
		if err := s.appSvc.Update(context.Background(), name, onProgress); err != nil {
			s.opsMu.Lock()
			op.Err = err.Error()
			op.Done = true
			s.opsMu.Unlock()
			return
		}
		s.opsMu.Lock()
		op.Done = true
		s.opsMu.Unlock()
	}()

	// Stream SSE to this caller by polling the operation state
	flusher, ok := w.(http.Flusher)
	if !ok {
		jsonError(w, 500, "streaming not supported")
		return
	}
	w.Header().Set("Content-Type", "text/event-stream")
	w.Header().Set("Cache-Control", "no-cache")
	w.Header().Set("Connection", "keep-alive")

	ticker := time.NewTicker(500 * time.Millisecond)
	defer ticker.Stop()
	lastStep := ""
	for {
		select {
		case <-r.Context().Done():
			return
		case <-ticker.C:
			s.opsMu.Lock()
			step := op.Step
			done := op.Done
			errMsg := op.Err
			s.opsMu.Unlock()

			if step != lastStep {
				sseEvent(w, flusher, map[string]string{"step": step})
				lastStep = step
			}
			if done {
				if errMsg != "" {
					sseEvent(w, flusher, map[string]string{"error": errMsg})
				} else {
					updated, err := s.appSvc.Get(name)
					if err != nil {
						sseEvent(w, flusher, map[string]string{"error": err.Error()})
					} else {
						sseEvent(w, flusher, map[string]any{"done": true, "app": updated})
					}
				}
				return
			}
		}
	}
}

func (s *Server) handleGetOperation(w http.ResponseWriter, r *http.Request) {
	name := chi.URLParam(r, "name")
	s.opsMu.Lock()
	op := s.ops[name]
	if op == nil {
		s.opsMu.Unlock()
		jsonResponse(w, 200, map[string]any{"active": false})
		return
	}
	resp := map[string]any{
		"active": !op.Done,
		"type":   op.Type,
		"step":   op.Step,
		"done":   op.Done,
	}
	if op.Err != "" {
		resp["error"] = op.Err
	}
	done := op.Done
	s.opsMu.Unlock()
	// Clean up completed operations after returning
	if done {
		s.opsMu.Lock()
		delete(s.ops, name)
		s.opsMu.Unlock()
	}
	jsonResponse(w, 200, resp)
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

func (s *Server) handleLogs(w http.ResponseWriter, r *http.Request) {
	name := chi.URLParam(r, "name")

	flusher, ok := w.(http.Flusher)
	if !ok {
		jsonError(w, 500, "streaming not supported")
		return
	}
	w.Header().Set("Content-Type", "text/event-stream")
	w.Header().Set("Cache-Control", "no-cache")
	w.Header().Set("Connection", "keep-alive")

	pr, pw := io.Pipe()
	go func() {
		defer pw.Close()
		s.appSvc.Logs(r.Context(), name, pw)
	}()

	scanner := bufio.NewScanner(pr)
	for scanner.Scan() {
		sseEvent(w, flusher, map[string]string{"line": scanner.Text()})
	}
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

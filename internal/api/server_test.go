package api

import (
	"encoding/json"
	"net/http/httptest"
	"path/filepath"
	"strings"
	"testing"

	"github.com/selfstack/selfstack/internal/app"
	"github.com/selfstack/selfstack/internal/registry"
	"github.com/selfstack/selfstack/internal/store"
)

func testServer(t *testing.T) *Server {
	t.Helper()
	dir := t.TempDir()
	s, err := store.Open(filepath.Join(dir, "test.db"))
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { s.Close() })
	r := registry.NewClient("http://localhost")
	svc := app.NewService(s, r)
	return NewServer(svc, r, "0.1.0-test")
}

func TestHandleStatus(t *testing.T) {
	srv := testServer(t)
	req := httptest.NewRequest("GET", "/api/status", nil)
	w := httptest.NewRecorder()
	srv.ServeHTTP(w, req)
	if w.Code != 200 {
		t.Fatalf("expected 200, got %d", w.Code)
	}
	var body map[string]any
	json.NewDecoder(w.Body).Decode(&body)
	if body["status"] != "ok" {
		t.Fatalf("expected status ok, got %v", body["status"])
	}
	if body["version"] != "0.1.0-test" {
		t.Fatalf("expected version 0.1.0-test, got %v", body["version"])
	}
}

func TestHandleListApps_Empty(t *testing.T) {
	srv := testServer(t)
	req := httptest.NewRequest("GET", "/api/apps", nil)
	w := httptest.NewRecorder()
	srv.ServeHTTP(w, req)
	if w.Code != 200 {
		t.Fatalf("expected 200, got %d", w.Code)
	}
}

func TestHandleInstallApp_BadBody(t *testing.T) {
	srv := testServer(t)
	req := httptest.NewRequest("POST", "/api/apps/install", strings.NewReader("not json"))
	w := httptest.NewRecorder()
	srv.ServeHTTP(w, req)
	if w.Code != 400 {
		t.Fatalf("expected 400, got %d", w.Code)
	}
}

func TestHandleInstallApp_MissingName(t *testing.T) {
	srv := testServer(t)
	req := httptest.NewRequest("POST", "/api/apps/install", strings.NewReader(`{}`))
	w := httptest.NewRecorder()
	srv.ServeHTTP(w, req)
	if w.Code != 400 {
		t.Fatalf("expected 400, got %d", w.Code)
	}
}

func TestHandleStartApp_NotFound(t *testing.T) {
	srv := testServer(t)
	req := httptest.NewRequest("POST", "/api/apps/nonexistent/start", nil)
	w := httptest.NewRecorder()
	srv.ServeHTTP(w, req)
	if w.Code != 404 {
		t.Fatalf("expected 404, got %d", w.Code)
	}
}

func TestHandleStopApp_NotFound(t *testing.T) {
	srv := testServer(t)
	req := httptest.NewRequest("POST", "/api/apps/nonexistent/stop", nil)
	w := httptest.NewRecorder()
	srv.ServeHTTP(w, req)
	if w.Code != 404 {
		t.Fatalf("expected 404, got %d", w.Code)
	}
}

func TestHandleRemoveApp_NotFound(t *testing.T) {
	srv := testServer(t)
	req := httptest.NewRequest("DELETE", "/api/apps/nonexistent", nil)
	w := httptest.NewRecorder()
	srv.ServeHTTP(w, req)
	// Remove doesn't error on not-found (it's idempotent)
	if w.Code != 200 {
		t.Fatalf("expected 200, got %d", w.Code)
	}
}

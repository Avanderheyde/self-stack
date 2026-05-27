package api

import (
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/selfstack/selfstack/internal/auth"
)

func TestAuthMiddleware_AllowsStatus(t *testing.T) {
	dir := t.TempDir()
	a, _ := auth.New(dir)
	handler := AuthMiddleware(a)(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(200)
	}))
	req := httptest.NewRequest("GET", "/api/status", nil)
	w := httptest.NewRecorder()
	handler.ServeHTTP(w, req)
	if w.Code != 200 {
		t.Fatalf("expected 200 for status, got %d", w.Code)
	}
}

func TestAuthMiddleware_BlocksWithoutToken(t *testing.T) {
	dir := t.TempDir()
	a, _ := auth.New(dir)
	handler := AuthMiddleware(a)(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(200)
	}))
	req := httptest.NewRequest("GET", "/api/apps", nil)
	w := httptest.NewRecorder()
	handler.ServeHTTP(w, req)
	if w.Code != 401 {
		t.Fatalf("expected 401, got %d", w.Code)
	}
}

func TestAuthMiddleware_AllowsWithValidToken(t *testing.T) {
	dir := t.TempDir()
	a, _ := auth.New(dir)
	token, _ := a.IssueToken("test-device")
	handler := AuthMiddleware(a)(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(200)
	}))
	req := httptest.NewRequest("GET", "/api/apps", nil)
	req.Header.Set("Authorization", token)
	w := httptest.NewRecorder()
	handler.ServeHTTP(w, req)
	if w.Code != 200 {
		t.Fatalf("expected 200 with valid token, got %d", w.Code)
	}
}

func TestAuthMiddleware_AllowsWithCookie(t *testing.T) {
	dir := t.TempDir()
	a, _ := auth.New(dir)
	token, _ := a.IssueToken("test-device")
	handler := AuthMiddleware(a)(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(200)
	}))
	req := httptest.NewRequest("GET", "/api/apps", nil)
	req.AddCookie(&http.Cookie{Name: "selfstack_token", Value: token})
	w := httptest.NewRecorder()
	handler.ServeHTTP(w, req)
	if w.Code != 200 {
		t.Fatalf("expected 200 with cookie token, got %d", w.Code)
	}
}

func TestAuthMiddleware_RejectsInvalidToken(t *testing.T) {
	dir := t.TempDir()
	a, _ := auth.New(dir)
	handler := AuthMiddleware(a)(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(200)
	}))
	req := httptest.NewRequest("GET", "/api/apps", nil)
	req.Header.Set("Authorization", "invalid.token")
	w := httptest.NewRecorder()
	handler.ServeHTTP(w, req)
	if w.Code != 401 {
		t.Fatalf("expected 401 for invalid token, got %d", w.Code)
	}
}

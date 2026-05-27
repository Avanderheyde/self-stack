package proxy

import (
	"fmt"
	"net/http"
	"net/http/httptest"
	"testing"
)

func TestRegisterAndRoute(t *testing.T) {
	backend := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Write([]byte("hello from app"))
	}))
	defer backend.Close()

	p := New()
	var port int
	fmt.Sscanf(backend.URL, "http://127.0.0.1:%d", &port)
	p.Register("myapp", port)

	req := httptest.NewRequest("GET", "/", nil)
	req.Host = "myapp.selfstack.local"
	w := httptest.NewRecorder()
	p.ServeHTTP(w, req)

	if w.Code != 200 {
		t.Fatalf("expected 200, got %d", w.Code)
	}
	if w.Body.String() != "hello from app" {
		t.Fatalf("expected 'hello from app', got %q", w.Body.String())
	}
}

func TestDeregister(t *testing.T) {
	p := New()
	p.Register("myapp", 10001)
	p.Deregister("myapp")

	req := httptest.NewRequest("GET", "/", nil)
	req.Host = "myapp.selfstack.local"
	w := httptest.NewRecorder()
	p.ServeHTTP(w, req)

	if w.Code != 404 {
		t.Fatalf("expected 404 after deregister, got %d", w.Code)
	}
}

func TestListRoutes(t *testing.T) {
	p := New()
	p.Register("app1", 10001)
	p.Register("app2", 10002)
	routes := p.ListRoutes()
	if len(routes) != 2 {
		t.Fatalf("expected 2 routes, got %d", len(routes))
	}
	if !routes["app1"] || !routes["app2"] {
		t.Fatal("missing expected routes")
	}
}

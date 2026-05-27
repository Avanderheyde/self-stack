package api

import (
	"net/http/httptest"
	"strings"
	"testing"
)

// TestHandleInstallApp_LocalPathDispatchesToInstallLocal verifies that the
// handler routes to InstallLocal when local_path is set. We observe this
// indirectly: a nonexistent local_path triggers an InstallLocal-specific
// error ("resolve local path" or "stat local path"), streamed back as an
// SSE `error` event. If the handler instead called Install (registry flow),
// we'd see a registry lookup error, not a local-path error.
func TestHandleInstallApp_LocalPathDispatchesToInstallLocal(t *testing.T) {
	srv := testServer(t)
	body := `{"name":"myapp","local_path":"/definitely/does/not/exist"}`
	req := httptest.NewRequest("POST", "/api/apps/install", strings.NewReader(body))
	w := httptest.NewRecorder()
	srv.ServeHTTP(w, req)
	if w.Code != 200 {
		t.Fatalf("expected SSE 200, got %d: %s", w.Code, w.Body.String())
	}
	out := w.Body.String()
	if !strings.Contains(out, "local path") {
		t.Fatalf("expected local-path error (proves InstallLocal was called), got: %s", out)
	}
}

// TestHandleInstallApp_NoLocalPathStillCallsInstall confirms the existing
// registry path is unchanged when local_path is omitted. With no registry
// available and the registry client pointing at localhost, Install fails
// with a network/lookup error that does NOT mention a local path.
func TestHandleInstallApp_NoLocalPathStillCallsInstall(t *testing.T) {
	srv := testServer(t)
	body := `{"name":"myapp"}`
	req := httptest.NewRequest("POST", "/api/apps/install", strings.NewReader(body))
	w := httptest.NewRecorder()
	srv.ServeHTTP(w, req)
	if w.Code != 200 {
		t.Fatalf("expected SSE 200, got %d: %s", w.Code, w.Body.String())
	}
	out := w.Body.String()
	if strings.Contains(out, "local path") {
		t.Fatalf("unexpected InstallLocal dispatch when local_path is empty: %s", out)
	}
	if !strings.Contains(out, `"error"`) {
		t.Fatalf("expected SSE error event from registry failure, got: %s", out)
	}
}

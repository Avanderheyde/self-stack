package cmd

import (
	"bytes"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
)

// withFakeAPI spins up an httptest server and points apiURL at it for the
// duration of the test. The handler controls the SSE stream the install
// command will parse.
func withFakeAPI(t *testing.T, handler http.HandlerFunc) {
	t.Helper()
	srv := httptest.NewServer(handler)
	t.Cleanup(srv.Close)
	prev := apiURL
	apiURL = srv.URL
	t.Cleanup(func() { apiURL = prev })
}

// runInstall executes the install command through rootCmd so flag parsing
// and arg validation behave exactly like in production. Flag-bound package
// vars are reset before each invocation so tests can't leak state into
// each other.
func runInstall(t *testing.T, args ...string) error {
	t.Helper()
	prevLocal, prevSeed := installLocal, installSeedData
	t.Cleanup(func() {
		installLocal = prevLocal
		installSeedData = prevSeed
	})
	installLocal = ""
	installSeedData = ""

	rootCmd.SetArgs(append([]string{"install"}, args...))
	// Silence cobra's auto-help output on error so failed tests aren't
	// drowned in 30 lines of command listing.
	rootCmd.SetOut(new(bytes.Buffer))
	rootCmd.SetErr(new(bytes.Buffer))
	rootCmd.SilenceUsage = true
	rootCmd.SilenceErrors = true
	return rootCmd.Execute()
}

// captureRequestBody is a tiny helper that returns the JSON body the install
// command posts to the API.
func captureRequestBody(t *testing.T, w http.ResponseWriter, r *http.Request) map[string]string {
	t.Helper()
	var body map[string]string
	if err := json.NewDecoder(r.Body).Decode(&body); err != nil {
		t.Fatalf("decode request: %v", err)
	}
	w.Header().Set("Content-Type", "text/event-stream")
	return body
}

// TestInstall_LocalFlagSendsLocalPath verifies that --local <path> resolves
// to an absolute path and is sent as `local_path` in the API request body.
func TestInstall_LocalFlagSendsLocalPath(t *testing.T) {
	var received map[string]string
	withFakeAPI(t, func(w http.ResponseWriter, r *http.Request) {
		received = captureRequestBody(t, w, r)
		fmt.Fprint(w, "data: {\"done\":true}\n\n")
	})
	if err := runInstall(t, "myapp", "--local", "."); err != nil {
		t.Fatalf("install: %v", err)
	}
	if received["name"] != "myapp" {
		t.Errorf("name = %q, want myapp", received["name"])
	}
	if received["local_path"] == "" || !strings.HasPrefix(received["local_path"], "/") {
		t.Errorf("local_path should be absolute, got %q", received["local_path"])
	}
}

// TestInstall_SeedDataFlagSendsSeedData verifies that --seed-data <path> is
// resolved to an absolute path and sent as `seed_data`.
func TestInstall_SeedDataFlagSendsSeedData(t *testing.T) {
	var received map[string]string
	withFakeAPI(t, func(w http.ResponseWriter, r *http.Request) {
		received = captureRequestBody(t, w, r)
		fmt.Fprint(w, "data: {\"done\":true}\n\n")
	})
	if err := runInstall(t, "myapp", "--local", ".", "--seed-data", "."); err != nil {
		t.Fatalf("install: %v", err)
	}
	if received["seed_data"] == "" || !strings.HasPrefix(received["seed_data"], "/") {
		t.Errorf("seed_data should be absolute, got %q", received["seed_data"])
	}
}

// TestInstall_SSEDoneReturnsSuccess verifies that a `done:true` event causes
// the command to return nil.
func TestInstall_SSEDoneReturnsSuccess(t *testing.T) {
	withFakeAPI(t, func(w http.ResponseWriter, r *http.Request) {
		io.Copy(io.Discard, r.Body)
		fmt.Fprint(w, "data: {\"step\":\"Cloning\"}\n\n")
		fmt.Fprint(w, "data: {\"done\":true}\n\n")
	})
	if err := runInstall(t, "myapp"); err != nil {
		t.Fatalf("expected nil error on done, got %v", err)
	}
}

// TestInstall_SSEErrorReturnsError verifies that an `error` event in the
// stream is turned into a Go error.
func TestInstall_SSEErrorReturnsError(t *testing.T) {
	withFakeAPI(t, func(w http.ResponseWriter, r *http.Request) {
		io.Copy(io.Discard, r.Body)
		fmt.Fprint(w, "data: {\"error\":\"build failed\"}\n\n")
	})
	err := runInstall(t, "myapp")
	if err == nil || !strings.Contains(err.Error(), "build failed") {
		t.Fatalf("expected build failed error, got %v", err)
	}
}

// TestInstall_StreamEndsWithoutDoneIsError verifies the safety net branch:
// if the server closes the stream without sending done or error, the CLI
// surfaces a clear error rather than silently succeeding.
func TestInstall_StreamEndsWithoutDoneIsError(t *testing.T) {
	withFakeAPI(t, func(w http.ResponseWriter, r *http.Request) {
		io.Copy(io.Discard, r.Body)
		fmt.Fprint(w, "data: {\"step\":\"Cloning\"}\n\n")
		// stream ends without done/error
	})
	err := runInstall(t, "myapp")
	if err == nil || !strings.Contains(err.Error(), "ended without completion") {
		t.Fatalf("expected stream-ended error, got %v", err)
	}
}

// TestInstall_NonSSELinesAreIgnored verifies the `if !strings.HasPrefix(line,
// "data: ") { continue }` branch — lines without the data prefix shouldn't
// abort the stream parse.
func TestInstall_NonSSELinesAreIgnored(t *testing.T) {
	withFakeAPI(t, func(w http.ResponseWriter, r *http.Request) {
		io.Copy(io.Discard, r.Body)
		fmt.Fprint(w, ": this is a comment\n")
		fmt.Fprint(w, "retry: 1000\n")
		fmt.Fprint(w, "data: {\"done\":true}\n\n")
	})
	if err := runInstall(t, "myapp"); err != nil {
		t.Fatalf("expected success, got %v", err)
	}
}

// TestInstall_MalformedJSONInDataLineIsSkipped verifies that an invalid JSON
// payload doesn't crash the parser — it should be skipped and parsing should
// continue.
func TestInstall_MalformedJSONInDataLineIsSkipped(t *testing.T) {
	withFakeAPI(t, func(w http.ResponseWriter, r *http.Request) {
		io.Copy(io.Discard, r.Body)
		fmt.Fprint(w, "data: this-is-not-json\n\n")
		fmt.Fprint(w, "data: {\"done\":true}\n\n")
	})
	if err := runInstall(t, "myapp"); err != nil {
		t.Fatalf("expected success, got %v", err)
	}
}

// TestInstall_LocalAbsFails covers P41: filepath.Abs failure on --local flag.
// Normally unreachable on POSIX; stub installAbsFn to force it.
func TestInstall_LocalAbsFails(t *testing.T) {
	prev := installAbsFn
	t.Cleanup(func() { installAbsFn = prev })
	installAbsFn = func(string) (string, error) { return "", errors.New("synthetic abs failure") }
	// No fake API needed — we should error before the HTTP request.
	err := runInstall(t, "myapp", "--local", ".")
	if err == nil || !strings.Contains(err.Error(), "resolve --local path") {
		t.Fatalf("expected resolve --local path error, got %v", err)
	}
}

// TestInstall_SeedDataAbsFails covers P42: filepath.Abs failure on --seed-data.
// Stub installAbsFn so the seed branch hits the error path. Note: --local is
// resolved first, so we let it succeed by returning a value for the first
// call and an error for the second.
func TestInstall_SeedDataAbsFails(t *testing.T) {
	prev := installAbsFn
	t.Cleanup(func() { installAbsFn = prev })
	calls := 0
	installAbsFn = func(p string) (string, error) {
		calls++
		if calls == 1 {
			return "/abs/" + p, nil
		}
		return "", errors.New("synthetic seed abs failure")
	}
	err := runInstall(t, "myapp", "--local", ".", "--seed-data", "./seed")
	if err == nil || !strings.Contains(err.Error(), "resolve --seed-data path") {
		t.Fatalf("expected resolve --seed-data path error, got %v", err)
	}
}

// TestInstall_Non200StatusReturnsError covers the early non-200 short-circuit
// branch that bypasses SSE parsing.
func TestInstall_Non200StatusReturnsError(t *testing.T) {
	withFakeAPI(t, func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusBadRequest)
		fmt.Fprint(w, `{"error":"name is required"}`)
	})
	err := runInstall(t, "myapp")
	if err == nil || !strings.Contains(err.Error(), "install failed") {
		t.Fatalf("expected install failed error, got %v", err)
	}
}

// TestInstall_SeedDataWithoutLocalRejected covers the CLI guard that
// catches a silent footgun: --seed-data without --local would otherwise
// build a request body containing seed_data but no local_path. The API
// would route to the registry path and ignore seed_data, surfacing as
// a "successful install" that migrated no data.
func TestInstall_SeedDataWithoutLocalRejected(t *testing.T) {
	withFakeAPI(t, func(w http.ResponseWriter, r *http.Request) {
		t.Fatalf("API should not be called when --seed-data is used without --local")
	})
	err := runInstall(t, "myapp", "--seed-data", "/tmp/somedb")
	if err == nil {
		t.Fatal("expected error for --seed-data without --local")
	}
	if !strings.Contains(err.Error(), "--seed-data requires --local") {
		t.Fatalf("expected guard error, got %v", err)
	}
}

//go:build integration

package test

import (
	"context"
	"net/http"
	"os"
	"path/filepath"
	"testing"
	"time"

	"github.com/selfstack/selfstack/internal/container"
	"github.com/selfstack/selfstack/internal/manifest"
)

func TestSampleApp_BuildAndRun(t *testing.T) {
	if os.Getenv("SELFSTACK_INTEGRATION") == "" {
		t.Skip("set SELFSTACK_INTEGRATION=1 to run")
	}
	ctx := context.Background()
	mgr := container.NewManager()
	appDir, err := filepath.Abs("sample-app")
	if err != nil {
		t.Fatal(err)
	}

	m, err := manifest.ParseFile(appDir + "/selfstack.yml")
	if err != nil {
		t.Fatal(err)
	}

	if err := mgr.Build(ctx, appDir, m.Runtime.Entry, "sample-app"); err != nil {
		t.Fatal(err)
	}

	testPort := 19999
	if err := mgr.Up(ctx, appDir, m.Runtime.Entry, "sample-app", m.Expose.Port, testPort, nil); err != nil {
		t.Fatal(err)
	}
	defer mgr.Down(ctx, appDir, m.Runtime.Entry, "sample-app")

	time.Sleep(5 * time.Second)
	resp, err := http.Get("http://localhost:19999/api/health")
	if err != nil {
		t.Fatal(err)
	}
	if resp.StatusCode != 200 {
		t.Fatalf("health check failed: %d", resp.StatusCode)
	}
}

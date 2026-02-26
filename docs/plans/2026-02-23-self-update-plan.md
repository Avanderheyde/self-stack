# SelfStack Self-Update Implementation Plan

> **For Claude:** REQUIRED SUB-SKILL: Use superpowers:executing-plans to implement this plan task-by-task.

**Goal:** Let users update the SelfStack binary with `selfstack update` and install it via a one-liner curl script.

**Architecture:** A `Version` variable set via ldflags at build time. A `selfstack update` command that checks GitHub Releases API and downloads the latest binary. GoReleaser + GitHub Actions to produce cross-platform release binaries.

**Tech Stack:** Go stdlib (net/http, os, runtime), GoReleaser, GitHub Actions

---

### Task 1: Add Version variable to cmd package

**Files:**
- Modify: `cmd/root.go`

**Step 1: Add the Version variable**

In `cmd/root.go`, add after the `apiURL` var:

```go
var Version = "dev"
```

**Step 2: Verify it compiles**

Run: `go build ./...`
Expected: Success

**Step 3: Commit**

```bash
git add cmd/root.go
git commit -m "feat: add Version variable to cmd package"
```

---

### Task 2: Wire Version into status endpoint and CLI

**Files:**
- Modify: `internal/api/apps.go:18` (handleStatus)
- Modify: `internal/api/server.go` (NewServer signature)
- Modify: `internal/api/server_test.go:41` (version assertion)
- Modify: `cmd/serve.go:64` (pass version to NewServer)

**Step 1: Add version field to Server and pass it through**

In `internal/api/server.go`, add a `version` field to the Server struct:

```go
type Server struct {
	appSvc   *app.Service
	registry *registry.Client
	router   chi.Router
	version  string
}
```

Update `NewServer` to accept version:

```go
func NewServer(appSvc *app.Service, reg *registry.Client, version string) *Server {
	s := &Server{appSvc: appSvc, registry: reg, version: version}
```

**Step 2: Use `s.version` in handleStatus**

In `internal/api/apps.go`, change the handleStatus method:

```go
jsonResponse(w, 200, map[string]any{
    "version": s.version,
    "apps":    len(apps),
    "status":  "ok",
})
```

**Step 3: Update serve.go to pass Version**

In `cmd/serve.go`, change the NewServer call:

```go
srv := api.NewServer(appSvc, reg, Version)
```

**Step 4: Update test**

In `internal/api/server_test.go`, update `testServer` to pass a version:

```go
return NewServer(svc, r, "0.1.0-test")
```

Update the version assertion in `TestHandleStatus`:

```go
if body["version"] != "0.1.0-test" {
    t.Fatalf("expected version 0.1.0-test, got %v", body["version"])
}
```

**Step 5: Verify**

Run: `go test ./internal/api/...`
Expected: All tests pass

Run: `go build ./...`
Expected: Success

**Step 6: Commit**

```bash
git add internal/api/server.go internal/api/apps.go internal/api/server_test.go cmd/serve.go
git commit -m "feat: wire Version into status endpoint"
```

---

### Task 3: Add `selfstack update` command

**Files:**
- Create: `cmd/update.go`

**Step 1: Create the update command**

Create `cmd/update.go`:

```go
package cmd

import (
	"compress/gzip"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"os"
	"runtime"
	"strings"

	"github.com/spf13/cobra"
)

const githubRepo = "Avanderheyde/self-stack"

type ghRelease struct {
	TagName string    `json:"tag_name"`
	Assets  []ghAsset `json:"assets"`
}

type ghAsset struct {
	Name               string `json:"name"`
	BrowserDownloadURL string `json:"browser_download_url"`
}

var updateCmd = &cobra.Command{
	Use:   "update",
	Short: "Update SelfStack to the latest version",
	RunE: func(cmd *cobra.Command, args []string) error {
		fmt.Printf("Current version: %s\n", Version)

		release, err := fetchLatestRelease()
		if err != nil {
			return fmt.Errorf("check for updates: %w", err)
		}

		latest := strings.TrimPrefix(release.TagName, "v")
		if latest == Version {
			fmt.Println("Already up to date.")
			return nil
		}

		assetName := fmt.Sprintf("selfstack_%s_%s.tar.gz", runtime.GOOS, runtime.GOARCH)
		var downloadURL string
		for _, a := range release.Assets {
			if a.Name == assetName {
				downloadURL = a.BrowserDownloadURL
				break
			}
		}
		if downloadURL == "" {
			return fmt.Errorf("no release binary found for %s/%s", runtime.GOOS, runtime.GOARCH)
		}

		fmt.Printf("Downloading %s...\n", release.TagName)
		if err := downloadAndReplace(downloadURL); err != nil {
			return fmt.Errorf("update failed: %w", err)
		}

		fmt.Printf("Updated selfstack from v%s to %s\n", Version, release.TagName)
		return nil
	},
}

func fetchLatestRelease() (*ghRelease, error) {
	url := fmt.Sprintf("https://api.github.com/repos/%s/releases/latest", githubRepo)
	resp, err := http.Get(url)
	if err != nil {
		return nil, err
	}
	defer resp.Body.Close()
	if resp.StatusCode != 200 {
		return nil, fmt.Errorf("GitHub API returned %d", resp.StatusCode)
	}
	var rel ghRelease
	if err := json.NewDecoder(resp.Body).Decode(&rel); err != nil {
		return nil, err
	}
	return &rel, nil
}

func downloadAndReplace(url string) error {
	resp, err := http.Get(url)
	if err != nil {
		return err
	}
	defer resp.Body.Close()
	if resp.StatusCode != 200 {
		return fmt.Errorf("download returned %d", resp.StatusCode)
	}

	gr, err := gzip.NewReader(resp.Body)
	if err != nil {
		return fmt.Errorf("gzip: %w", err)
	}
	defer gr.Close()

	// tar contains a single file: selfstack
	// Skip the 512-byte tar header to get to file content
	header := make([]byte, 512)
	if _, err := io.ReadFull(gr, header); err != nil {
		return fmt.Errorf("read tar header: %w", err)
	}

	exe, err := os.Executable()
	if err != nil {
		return err
	}

	tmp, err := os.CreateTemp("", "selfstack-update-*")
	if err != nil {
		return err
	}
	defer os.Remove(tmp.Name())

	if _, err := io.Copy(tmp, gr); err != nil {
		tmp.Close()
		return err
	}
	tmp.Close()

	if err := os.Chmod(tmp.Name(), 0755); err != nil {
		return err
	}

	return os.Rename(tmp.Name(), exe)
}

func init() { rootCmd.AddCommand(updateCmd) }
```

**Step 2: Verify it compiles**

Run: `go build ./...`
Expected: Success

**Step 3: Verify the command shows up**

Run: `go run . update --help`
Expected: Shows "Update SelfStack to the latest version"

**Step 4: Commit**

```bash
git add cmd/update.go
git commit -m "feat: add selfstack update command"
```

---

### Task 4: Add startup version check nudge to serve

**Files:**
- Modify: `cmd/serve.go`

**Step 1: Add non-blocking version check**

In `cmd/serve.go`, add a goroutine before the `log.Printf("SelfStack dashboard on %s", addr)` line:

```go
// Non-blocking update check
go func() {
    ctx, cancel := context.WithTimeout(context.Background(), 3*time.Second)
    defer cancel()
    req, err := http.NewRequestWithContext(ctx, "GET",
        fmt.Sprintf("https://api.github.com/repos/%s/releases/latest", githubRepo),
        nil)
    if err != nil {
        return
    }
    resp, err := http.DefaultClient.Do(req)
    if err != nil || resp.StatusCode != 200 {
        return
    }
    defer resp.Body.Close()
    var rel struct{ TagName string `json:"tag_name"` }
    if json.NewDecoder(resp.Body).Decode(&rel) != nil {
        return
    }
    latest := strings.TrimPrefix(rel.TagName, "v")
    if latest != Version && Version != "dev" {
        log.Printf("A new version of SelfStack is available (%s). Run \"selfstack update\" to upgrade.", rel.TagName)
    }
}()
```

Add these imports to serve.go: `"context"`, `"encoding/json"`, `"strings"`, `"time"`.

Also reference `githubRepo` from `cmd/update.go` (it's in the same package, so it's accessible).

**Step 2: Verify it compiles**

Run: `go build ./...`
Expected: Success

**Step 3: Commit**

```bash
git add cmd/serve.go
git commit -m "feat: show update nudge on serve startup"
```

---

### Task 5: Add GoReleaser config

**Files:**
- Create: `.goreleaser.yml`

**Step 1: Create the config**

Create `.goreleaser.yml`:

```yaml
version: 2

builds:
  - main: .
    binary: selfstack
    ldflags:
      - -s -w -X github.com/selfstack/selfstack/cmd.Version={{ .Version }}
    goos:
      - darwin
      - linux
    goarch:
      - amd64
      - arm64

archives:
  - format: tar.gz

checksum:
  name_template: checksums.txt

release:
  github:
    owner: Avanderheyde
    name: self-stack
```

**Step 2: Commit**

```bash
git add .goreleaser.yml
git commit -m "feat: add GoReleaser config"
```

---

### Task 6: Add GitHub Actions release workflow

**Files:**
- Create: `.github/workflows/release.yml`

**Step 1: Create the workflow**

Create `.github/workflows/release.yml`:

```yaml
name: Release

on:
  push:
    tags:
      - 'v*'

permissions:
  contents: write

jobs:
  release:
    runs-on: ubuntu-latest
    steps:
      - uses: actions/checkout@v4
        with:
          fetch-depth: 0

      - uses: actions/setup-go@v5
        with:
          go-version-file: go.mod

      - uses: goreleaser/goreleaser-action@v6
        with:
          version: '~> v2'
          args: release --clean
        env:
          GITHUB_TOKEN: ${{ secrets.GITHUB_TOKEN }}
```

**Step 2: Commit**

```bash
mkdir -p .github/workflows
git add .github/workflows/release.yml
git commit -m "ci: add GitHub Actions release workflow"
```

---

### Task 7: Add install script

**Files:**
- Create: `scripts/install.sh`

**Step 1: Create the script**

Create `scripts/install.sh`:

```bash
#!/bin/sh
set -e

REPO="Avanderheyde/self-stack"

# Detect OS
OS=$(uname -s | tr '[:upper:]' '[:lower:]')
case "$OS" in
  darwin) OS="darwin" ;;
  linux)  OS="linux" ;;
  *)      echo "Unsupported OS: $OS"; exit 1 ;;
esac

# Detect architecture
ARCH=$(uname -m)
case "$ARCH" in
  x86_64)  ARCH="amd64" ;;
  aarch64) ARCH="arm64" ;;
  arm64)   ARCH="arm64" ;;
  *)       echo "Unsupported architecture: $ARCH"; exit 1 ;;
esac

# Get latest release tag
TAG=$(curl -fsSL "https://api.github.com/repos/${REPO}/releases/latest" | grep '"tag_name"' | head -1 | cut -d'"' -f4)
if [ -z "$TAG" ]; then
  echo "Failed to fetch latest release"
  exit 1
fi

ASSET="selfstack_${OS}_${ARCH}.tar.gz"
URL="https://github.com/${REPO}/releases/download/${TAG}/${ASSET}"

echo "Downloading SelfStack ${TAG} for ${OS}/${ARCH}..."

TMP=$(mktemp -d)
curl -fsSL "$URL" -o "${TMP}/${ASSET}"
tar -xzf "${TMP}/${ASSET}" -C "$TMP"

# Install binary
INSTALL_DIR="/usr/local/bin"
if [ ! -w "$INSTALL_DIR" ]; then
  INSTALL_DIR="${HOME}/.local/bin"
  mkdir -p "$INSTALL_DIR"
  echo "Installing to ${INSTALL_DIR} (add to PATH if needed)"
fi

mv "${TMP}/selfstack" "${INSTALL_DIR}/selfstack"
chmod +x "${INSTALL_DIR}/selfstack"
rm -rf "$TMP"

echo "SelfStack ${TAG} installed to ${INSTALL_DIR}/selfstack"
selfstack status 2>/dev/null || echo "Run 'selfstack serve' to start."
```

**Step 2: Make executable**

Run: `chmod +x scripts/install.sh`

**Step 3: Commit**

```bash
git add scripts/install.sh
git commit -m "feat: add install script for curl|sh installation"
```

---

### Task 8: Final verification

**Step 1: Full build**

Run: `go build ./...`
Expected: Success

**Step 2: Run all tests**

Run: `go test ./internal/...`
Expected: All pass

**Step 3: Verify update command**

Run: `go run . update`
Expected: Either "no release binary found" (no releases yet) or a GitHub API response

**Step 4: Verify version in status**

Run: `go run . serve &` then `curl -s localhost:8080/api/status | grep version`
Expected: `"version":"dev"`

**Step 5: Push**

```bash
git push
```

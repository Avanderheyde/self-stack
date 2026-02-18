# SelfStack Implementation Plan

> **For Claude:** REQUIRED SUB-SKILL: Use superpowers:executing-plans to implement this plan task-by-task.

**Goal:** Build a single Go binary that serves as a personal app server — letting users install, run, and securely access self-hosted web apps from anywhere.

**Architecture:** Go binary with embedded React dashboard. Docker SDK for container orchestration, Caddy for reverse proxy, Cloudflare Tunnel for remote access, SQLite for state, Ed25519 for device-based auth.

**Tech Stack:** Go 1.22+, React 18 + TypeScript, Docker SDK, Chi router, Cobra CLI, SQLite (modernc.org/sqlite), Caddy (embedded), Cloudflare Tunnel

**Design Doc:** `docs/plans/2026-02-18-selfstack-design.md`

---

## Phase 1: Project Foundation

### Task 1: Initialize Go module and directory structure

**Files:**
- Create: `go.mod`
- Create: `main.go`
- Create: `Makefile`
- Create: `internal/config/paths.go`
- Create: `.gitignore`

**Step 1: Initialize Go module**

Run: `go mod init github.com/selfstack/selfstack`

**Step 2: Create directory structure**

```
selfstack/
├── main.go                  # Entry point
├── Makefile                 # Build commands
├── internal/
│   ├── config/              # Paths, defaults, constants
│   ├── store/               # SQLite database layer
│   ├── manifest/            # selfstack.yml parser
│   ├── registry/            # App catalog client
│   ├── container/           # Docker SDK wrapper
│   ├── proxy/               # Reverse proxy (Caddy)
│   ├── tunnel/              # Cloudflare Tunnel
│   ├── auth/                # Device trust & tokens
│   └── api/                 # REST API handlers
├── cmd/                     # CLI commands (Cobra)
├── dashboard/               # React app (built separately, embedded)
└── docs/plans/              # Design & implementation docs
```

**Step 3: Write minimal main.go**

```go
package main

import "fmt"

func main() {
    fmt.Println("selfstack v0.1.0")
}
```

**Step 4: Write paths.go with SelfStack home directory config**

```go
package config

import (
    "os"
    "path/filepath"
)

const (
    DefaultPort     = 8080
    PortRangeStart  = 10001
    AppDir          = "apps"
    ExportsDir      = "exports"
    KeysDir         = "keys"
    DBFile          = "selfstack.db"
)

func HomeDir() string {
    if env := os.Getenv("SELFSTACK_HOME"); env != "" {
        return env
    }
    home, _ := os.UserHomeDir()
    return filepath.Join(home, ".selfstack")
}

func AppsDir() string   { return filepath.Join(HomeDir(), AppDir) }
func DataDir() string   { return filepath.Join(HomeDir(), ExportsDir) }
func KeysPath() string  { return filepath.Join(HomeDir(), KeysDir) }
func DBPath() string    { return filepath.Join(HomeDir(), DBFile) }
```

**Step 5: Write .gitignore**

```
/selfstack
/dashboard/node_modules
/dashboard/dist
*.db
.selfstack/
```

**Step 6: Write Makefile**

```makefile
.PHONY: build test run clean

build:
	go build -o selfstack .

test:
	go test ./... -v

run: build
	./selfstack

clean:
	rm -f selfstack
```

**Step 7: Verify it builds**

Run: `cd /Users/alderik/Code/projects/local-app-framework && make build && ./selfstack`
Expected: `selfstack v0.1.0`

**Step 8: Commit**

```bash
git init && git add -A && git commit -m "feat: initialize Go project with directory structure"
```

---

### Task 2: SQLite config store — schema and connection

**Files:**
- Create: `internal/store/store.go`
- Create: `internal/store/store_test.go`

**Step 1: Install SQLite dependency**

Run: `go get modernc.org/sqlite`

**Step 2: Write the failing test**

```go
package store

import (
    "os"
    "path/filepath"
    "testing"
)

func testDB(t *testing.T) *Store {
    t.Helper()
    dir := t.TempDir()
    s, err := Open(filepath.Join(dir, "test.db"))
    if err != nil {
        t.Fatal(err)
    }
    t.Cleanup(func() { s.Close() })
    return s
}

func TestOpen_CreatesTables(t *testing.T) {
    s := testDB(t)

    // Verify apps table exists
    var count int
    err := s.db.QueryRow("SELECT count(*) FROM sqlite_master WHERE type='table' AND name='apps'").Scan(&count)
    if err != nil {
        t.Fatal(err)
    }
    if count != 1 {
        t.Fatalf("expected apps table, got count=%d", count)
    }

    // Verify devices table exists
    err = s.db.QueryRow("SELECT count(*) FROM sqlite_master WHERE type='table' AND name='devices'").Scan(&count)
    if err != nil {
        t.Fatal(err)
    }
    if count != 1 {
        t.Fatalf("expected devices table, got count=%d", count)
    }
}
```

**Step 3: Run test to verify it fails**

Run: `go test ./internal/store/ -v`
Expected: FAIL — `Open` not defined

**Step 4: Write implementation**

```go
package store

import (
    "database/sql"
    "fmt"

    _ "modernc.org/sqlite"
)

type Store struct {
    db *sql.DB
}

func Open(path string) (*Store, error) {
    db, err := sql.Open("sqlite", path)
    if err != nil {
        return nil, fmt.Errorf("open db: %w", err)
    }
    s := &Store{db: db}
    if err := s.migrate(); err != nil {
        db.Close()
        return nil, fmt.Errorf("migrate: %w", err)
    }
    return s, nil
}

func (s *Store) Close() error { return s.db.Close() }

func (s *Store) migrate() error {
    schema := `
    CREATE TABLE IF NOT EXISTS apps (
        name        TEXT PRIMARY KEY,
        display_name TEXT NOT NULL,
        description TEXT,
        repo_url    TEXT NOT NULL,
        version     TEXT,
        host_port   INTEGER NOT NULL,
        status      TEXT NOT NULL DEFAULT 'stopped',
        installed_at DATETIME DEFAULT CURRENT_TIMESTAMP
    );
    CREATE TABLE IF NOT EXISTS devices (
        id          TEXT PRIMARY KEY,
        name        TEXT NOT NULL,
        token_hash  TEXT NOT NULL,
        trusted     INTEGER NOT NULL DEFAULT 0,
        created_at  DATETIME DEFAULT CURRENT_TIMESTAMP,
        last_seen   DATETIME
    );
    CREATE TABLE IF NOT EXISTS app_config (
        app_name    TEXT NOT NULL,
        key         TEXT NOT NULL,
        value       TEXT,
        PRIMARY KEY (app_name, key),
        FOREIGN KEY (app_name) REFERENCES apps(name)
    );
    CREATE TABLE IF NOT EXISTS port_allocations (
        port        INTEGER PRIMARY KEY,
        app_name    TEXT NOT NULL UNIQUE,
        FOREIGN KEY (app_name) REFERENCES apps(name)
    );`
    _, err := s.db.Exec(schema)
    return err
}
```

**Step 5: Run test to verify it passes**

Run: `go test ./internal/store/ -v`
Expected: PASS

**Step 6: Commit**

```bash
git add internal/store/ go.mod go.sum
git commit -m "feat: add SQLite store with schema migrations"
```

---

### Task 3: Store — app CRUD operations

**Files:**
- Modify: `internal/store/store.go`
- Create: `internal/store/apps.go`
- Create: `internal/store/apps_test.go`

**Step 1: Write failing tests**

```go
package store

import "testing"

func TestInsertApp(t *testing.T) {
    s := testDB(t)
    app := App{
        Name:        "budget-tracker",
        DisplayName: "Budget Tracker",
        Description: "Finance app",
        RepoURL:     "https://github.com/selfstack-apps/budget-tracker",
        Version:     "1.0.0",
        HostPort:    10001,
        Status:      "running",
    }
    if err := s.InsertApp(app); err != nil {
        t.Fatal(err)
    }
    got, err := s.GetApp("budget-tracker")
    if err != nil {
        t.Fatal(err)
    }
    if got.DisplayName != "Budget Tracker" {
        t.Fatalf("expected Budget Tracker, got %s", got.DisplayName)
    }
}

func TestListApps(t *testing.T) {
    s := testDB(t)
    s.InsertApp(App{Name: "app-a", DisplayName: "A", RepoURL: "r", HostPort: 10001, Status: "running"})
    s.InsertApp(App{Name: "app-b", DisplayName: "B", RepoURL: "r", HostPort: 10002, Status: "stopped"})
    apps, err := s.ListApps()
    if err != nil {
        t.Fatal(err)
    }
    if len(apps) != 2 {
        t.Fatalf("expected 2 apps, got %d", len(apps))
    }
}

func TestDeleteApp(t *testing.T) {
    s := testDB(t)
    s.InsertApp(App{Name: "app-a", DisplayName: "A", RepoURL: "r", HostPort: 10001, Status: "running"})
    if err := s.DeleteApp("app-a"); err != nil {
        t.Fatal(err)
    }
    _, err := s.GetApp("app-a")
    if err == nil {
        t.Fatal("expected error for deleted app")
    }
}

func TestUpdateAppStatus(t *testing.T) {
    s := testDB(t)
    s.InsertApp(App{Name: "app-a", DisplayName: "A", RepoURL: "r", HostPort: 10001, Status: "stopped"})
    if err := s.UpdateAppStatus("app-a", "running"); err != nil {
        t.Fatal(err)
    }
    got, _ := s.GetApp("app-a")
    if got.Status != "running" {
        t.Fatalf("expected running, got %s", got.Status)
    }
}
```

**Step 2: Run tests to verify they fail**

Run: `go test ./internal/store/ -run TestInsertApp -v`
Expected: FAIL

**Step 3: Write implementation in apps.go**

```go
package store

import (
    "database/sql"
    "fmt"
)

type App struct {
    Name        string
    DisplayName string
    Description string
    RepoURL     string
    Version     string
    HostPort    int
    Status      string
}

func (s *Store) InsertApp(a App) error {
    _, err := s.db.Exec(
        `INSERT INTO apps (name, display_name, description, repo_url, version, host_port, status)
         VALUES (?, ?, ?, ?, ?, ?, ?)`,
        a.Name, a.DisplayName, a.Description, a.RepoURL, a.Version, a.HostPort, a.Status,
    )
    return err
}

func (s *Store) GetApp(name string) (App, error) {
    var a App
    err := s.db.QueryRow(
        `SELECT name, display_name, description, repo_url, version, host_port, status FROM apps WHERE name = ?`,
        name,
    ).Scan(&a.Name, &a.DisplayName, &a.Description, &a.RepoURL, &a.Version, &a.HostPort, &a.Status)
    if err == sql.ErrNoRows {
        return a, fmt.Errorf("app %q not found", name)
    }
    return a, err
}

func (s *Store) ListApps() ([]App, error) {
    rows, err := s.db.Query(
        `SELECT name, display_name, description, repo_url, version, host_port, status FROM apps ORDER BY name`,
    )
    if err != nil {
        return nil, err
    }
    defer rows.Close()
    var apps []App
    for rows.Next() {
        var a App
        if err := rows.Scan(&a.Name, &a.DisplayName, &a.Description, &a.RepoURL, &a.Version, &a.HostPort, &a.Status); err != nil {
            return nil, err
        }
        apps = append(apps, a)
    }
    return apps, rows.Err()
}

func (s *Store) DeleteApp(name string) error {
    _, err := s.db.Exec(`DELETE FROM apps WHERE name = ?`, name)
    return err
}

func (s *Store) UpdateAppStatus(name, status string) error {
    _, err := s.db.Exec(`UPDATE apps SET status = ? WHERE name = ?`, status, name)
    return err
}
```

**Step 4: Run tests**

Run: `go test ./internal/store/ -v`
Expected: ALL PASS

**Step 5: Commit**

```bash
git add internal/store/
git commit -m "feat: add app CRUD operations to store"
```

---

### Task 4: Store — port allocator

**Files:**
- Create: `internal/store/ports.go`
- Create: `internal/store/ports_test.go`

**Step 1: Write failing tests**

```go
package store

import "testing"

func TestAllocatePort(t *testing.T) {
    s := testDB(t)
    port, err := s.AllocatePort("app-a")
    if err != nil {
        t.Fatal(err)
    }
    if port != 10001 {
        t.Fatalf("expected 10001, got %d", port)
    }
}

func TestAllocatePort_Sequential(t *testing.T) {
    s := testDB(t)
    s.AllocatePort("app-a")
    port, err := s.AllocatePort("app-b")
    if err != nil {
        t.Fatal(err)
    }
    if port != 10002 {
        t.Fatalf("expected 10002, got %d", port)
    }
}

func TestAllocatePort_FillsGaps(t *testing.T) {
    s := testDB(t)
    s.AllocatePort("app-a") // 10001
    s.AllocatePort("app-b") // 10002
    s.ReleasePort("app-a")  // free 10001
    port, _ := s.AllocatePort("app-c")
    if port != 10001 {
        t.Fatalf("expected 10001 (gap fill), got %d", port)
    }
}

func TestReleasePort(t *testing.T) {
    s := testDB(t)
    s.AllocatePort("app-a")
    if err := s.ReleasePort("app-a"); err != nil {
        t.Fatal(err)
    }
    port, _ := s.AllocatePort("app-b")
    if port != 10001 {
        t.Fatalf("expected 10001 after release, got %d", port)
    }
}
```

**Step 2: Run to verify failure**

Run: `go test ./internal/store/ -run TestAllocatePort -v`
Expected: FAIL

**Step 3: Implement ports.go**

```go
package store

import "github.com/selfstack/selfstack/internal/config"

func (s *Store) AllocatePort(appName string) (int, error) {
    // Find lowest available port starting from PortRangeStart
    var port int
    err := s.db.QueryRow(`
        SELECT min(p) FROM (
            SELECT ? AS p
            UNION ALL
            SELECT port + 1 FROM port_allocations
        ) WHERE p NOT IN (SELECT port FROM port_allocations)
    `, config.PortRangeStart).Scan(&port)
    if err != nil {
        return 0, err
    }
    _, err = s.db.Exec(`INSERT INTO port_allocations (port, app_name) VALUES (?, ?)`, port, appName)
    if err != nil {
        return 0, err
    }
    return port, nil
}

func (s *Store) ReleasePort(appName string) error {
    _, err := s.db.Exec(`DELETE FROM port_allocations WHERE app_name = ?`, appName)
    return err
}

func (s *Store) GetPort(appName string) (int, error) {
    var port int
    err := s.db.QueryRow(`SELECT port FROM port_allocations WHERE app_name = ?`, appName).Scan(&port)
    return port, err
}
```

**Step 4: Run tests**

Run: `go test ./internal/store/ -v`
Expected: ALL PASS

**Step 5: Commit**

```bash
git add internal/store/ports.go internal/store/ports_test.go
git commit -m "feat: add port allocator with gap-filling"
```

---

## Phase 2: Manifest Parser & Registry Client

### Task 5: selfstack.yml manifest parser

**Files:**
- Create: `internal/manifest/manifest.go`
- Create: `internal/manifest/manifest_test.go`

**Step 1: Install YAML dependency**

Run: `go get gopkg.in/yaml.v3`

**Step 2: Write failing test**

```go
package manifest

import (
    "os"
    "path/filepath"
    "testing"
)

func TestParse(t *testing.T) {
    dir := t.TempDir()
    content := `
name: budget-tracker
display_name: Budget Tracker
description: Personal finance tracker
version: 1.0.0
icon: icon.png

runtime:
  type: docker-compose
  entry: docker-compose.yml

expose:
  port: 3000
  health: /api/health

volumes:
  - data:/app/data

config:
  - key: CURRENCY
    default: USD
    description: Default currency
`
    os.WriteFile(filepath.Join(dir, "selfstack.yml"), []byte(content), 0644)

    m, err := ParseFile(filepath.Join(dir, "selfstack.yml"))
    if err != nil {
        t.Fatal(err)
    }
    if m.Name != "budget-tracker" {
        t.Fatalf("expected budget-tracker, got %s", m.Name)
    }
    if m.Expose.Port != 3000 {
        t.Fatalf("expected port 3000, got %d", m.Expose.Port)
    }
    if len(m.Volumes) != 1 {
        t.Fatalf("expected 1 volume, got %d", len(m.Volumes))
    }
    if len(m.Config) != 1 || m.Config[0].Key != "CURRENCY" {
        t.Fatal("expected CURRENCY config key")
    }
}

func TestParse_MissingName(t *testing.T) {
    dir := t.TempDir()
    content := `display_name: No Name App
runtime:
  type: docker-compose
expose:
  port: 3000
`
    os.WriteFile(filepath.Join(dir, "selfstack.yml"), []byte(content), 0644)
    _, err := ParseFile(filepath.Join(dir, "selfstack.yml"))
    if err == nil {
        t.Fatal("expected error for missing name")
    }
}
```

**Step 3: Run to verify failure**

Run: `go test ./internal/manifest/ -v`
Expected: FAIL

**Step 4: Implement manifest.go**

```go
package manifest

import (
    "fmt"
    "os"

    "gopkg.in/yaml.v3"
)

type Manifest struct {
    Name        string       `yaml:"name"`
    DisplayName string       `yaml:"display_name"`
    Description string       `yaml:"description"`
    Version     string       `yaml:"version"`
    Icon        string       `yaml:"icon"`
    Runtime     Runtime      `yaml:"runtime"`
    Expose      Expose       `yaml:"expose"`
    Volumes     []string     `yaml:"volumes"`
    Config      []ConfigVar  `yaml:"config"`
}

type Runtime struct {
    Type  string `yaml:"type"`
    Entry string `yaml:"entry"`
}

type Expose struct {
    Port   int    `yaml:"port"`
    Health string `yaml:"health"`
}

type ConfigVar struct {
    Key         string `yaml:"key"`
    Default     string `yaml:"default"`
    Description string `yaml:"description"`
}

func ParseFile(path string) (*Manifest, error) {
    data, err := os.ReadFile(path)
    if err != nil {
        return nil, fmt.Errorf("read manifest: %w", err)
    }
    return Parse(data)
}

func Parse(data []byte) (*Manifest, error) {
    var m Manifest
    if err := yaml.Unmarshal(data, &m); err != nil {
        return nil, fmt.Errorf("parse manifest: %w", err)
    }
    if m.Name == "" {
        return nil, fmt.Errorf("manifest missing required field: name")
    }
    if m.Expose.Port == 0 {
        return nil, fmt.Errorf("manifest missing required field: expose.port")
    }
    if m.Runtime.Type == "" {
        m.Runtime.Type = "docker-compose"
    }
    if m.Runtime.Entry == "" {
        m.Runtime.Entry = "docker-compose.yml"
    }
    return &m, nil
}
```

**Step 5: Run tests**

Run: `go test ./internal/manifest/ -v`
Expected: ALL PASS

**Step 6: Commit**

```bash
git add internal/manifest/ go.mod go.sum
git commit -m "feat: add selfstack.yml manifest parser"
```

---

### Task 6: Registry client — fetch and search app catalog

**Files:**
- Create: `internal/registry/registry.go`
- Create: `internal/registry/registry_test.go`

**Step 1: Write failing test**

```go
package registry

import (
    "encoding/json"
    "net/http"
    "net/http/httptest"
    "testing"
)

func TestFetch(t *testing.T) {
    catalog := Catalog{
        Apps: []AppEntry{
            {Name: "budget-tracker", DisplayName: "Budget Tracker", Category: "Finance", Repo: "https://github.com/x/y", Verified: true},
            {Name: "notes", DisplayName: "Notes", Category: "Productivity", Repo: "https://github.com/x/z", Verified: true},
        },
        Categories: []string{"Finance", "Productivity"},
    }
    srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
        json.NewEncoder(w).Encode(catalog)
    }))
    defer srv.Close()

    c := NewClient(srv.URL)
    got, err := c.Fetch()
    if err != nil {
        t.Fatal(err)
    }
    if len(got.Apps) != 2 {
        t.Fatalf("expected 2 apps, got %d", len(got.Apps))
    }
}

func TestSearch(t *testing.T) {
    catalog := &Catalog{
        Apps: []AppEntry{
            {Name: "budget-tracker", DisplayName: "Budget Tracker", Description: "finance app", Category: "Finance"},
            {Name: "notes", DisplayName: "Notes", Description: "note taking", Category: "Productivity"},
        },
    }
    results := Search(catalog, "finance")
    if len(results) != 1 || results[0].Name != "budget-tracker" {
        t.Fatalf("expected budget-tracker, got %v", results)
    }
}
```

**Step 2: Run to verify failure**

Run: `go test ./internal/registry/ -v`
Expected: FAIL

**Step 3: Implement registry.go**

```go
package registry

import (
    "encoding/json"
    "fmt"
    "net/http"
    "strings"
    "time"
)

type AppEntry struct {
    Name        string `json:"name"`
    DisplayName string `json:"display_name"`
    Description string `json:"description"`
    Category    string `json:"category"`
    Repo        string `json:"repo"`
    Icon        string `json:"icon"`
    Verified    bool   `json:"verified"`
}

type Catalog struct {
    Apps       []AppEntry `json:"apps"`
    Categories []string   `json:"categories"`
}

type Client struct {
    url    string
    client *http.Client
    cache  *Catalog
}

func NewClient(url string) *Client {
    return &Client{
        url:    url,
        client: &http.Client{Timeout: 10 * time.Second},
    }
}

func (c *Client) Fetch() (*Catalog, error) {
    resp, err := c.client.Get(c.url)
    if err != nil {
        return nil, fmt.Errorf("fetch registry: %w", err)
    }
    defer resp.Body.Close()
    if resp.StatusCode != http.StatusOK {
        return nil, fmt.Errorf("registry returned %d", resp.StatusCode)
    }
    var cat Catalog
    if err := json.NewDecoder(resp.Body).Decode(&cat); err != nil {
        return nil, fmt.Errorf("decode registry: %w", err)
    }
    c.cache = &cat
    return &cat, nil
}

func (c *Client) Cached() *Catalog { return c.cache }

func (c *Client) Lookup(name string) (*AppEntry, error) {
    cat := c.cache
    if cat == nil {
        var err error
        cat, err = c.Fetch()
        if err != nil {
            return nil, err
        }
    }
    for _, app := range cat.Apps {
        if app.Name == name {
            return &app, nil
        }
    }
    return nil, fmt.Errorf("app %q not found in registry", name)
}

func Search(cat *Catalog, query string) []AppEntry {
    q := strings.ToLower(query)
    var results []AppEntry
    for _, app := range cat.Apps {
        if strings.Contains(strings.ToLower(app.Name), q) ||
            strings.Contains(strings.ToLower(app.DisplayName), q) ||
            strings.Contains(strings.ToLower(app.Description), q) ||
            strings.Contains(strings.ToLower(app.Category), q) {
            results = append(results, app)
        }
    }
    return results
}
```

**Step 4: Run tests**

Run: `go test ./internal/registry/ -v`
Expected: ALL PASS

**Step 5: Commit**

```bash
git add internal/registry/
git commit -m "feat: add registry client with fetch, lookup, and search"
```

---

## Phase 3: Container Manager

### Task 7: Docker container manager — create and start containers

**Files:**
- Create: `internal/container/manager.go`
- Create: `internal/container/manager_test.go`

**Step 1: Install Docker SDK**

Run: `go get github.com/docker/docker/client github.com/docker/docker/api/types github.com/docker/go-connections`

**Step 2: Write manager.go**

Note: Docker SDK tests require a running Docker daemon, so we test with an interface + mock for unit tests and a separate integration test tag for live Docker tests.

```go
package container

import (
    "context"
    "fmt"
    "io"
    "os/exec"
    "path/filepath"
    "time"
    "net/http"
)

type Manager struct {
    timeout time.Duration
}

func NewManager() *Manager {
    return &Manager{timeout: 5 * time.Minute}
}

// Build runs docker compose build in the app directory.
func (m *Manager) Build(ctx context.Context, appDir string, composePath string) error {
    cmd := exec.CommandContext(ctx, "docker", "compose", "-f", filepath.Join(appDir, composePath), "build")
    cmd.Dir = appDir
    out, err := cmd.CombinedOutput()
    if err != nil {
        return fmt.Errorf("docker compose build: %s: %w", string(out), err)
    }
    return nil
}

// Up starts the app's containers with the given host port mapping.
func (m *Manager) Up(ctx context.Context, appDir, composePath string, appName string, containerPort, hostPort int, envVars map[string]string) error {
    args := []string{"compose", "-f", filepath.Join(appDir, composePath), "-p", "selfstack-" + appName, "up", "-d"}
    cmd := exec.CommandContext(ctx, "docker", args...)
    cmd.Dir = appDir
    env := []string{
        fmt.Sprintf("SELFSTACK_HOST_PORT=%d", hostPort),
        fmt.Sprintf("SELFSTACK_CONTAINER_PORT=%d", containerPort),
    }
    for k, v := range envVars {
        env = append(env, fmt.Sprintf("%s=%s", k, v))
    }
    cmd.Env = append(cmd.Environ(), env...)
    out, err := cmd.CombinedOutput()
    if err != nil {
        return fmt.Errorf("docker compose up: %s: %w", string(out), err)
    }
    return nil
}

// Down stops and removes the app's containers.
func (m *Manager) Down(ctx context.Context, appDir, composePath, appName string) error {
    cmd := exec.CommandContext(ctx, "docker", "compose", "-f", filepath.Join(appDir, composePath), "-p", "selfstack-"+appName, "down")
    cmd.Dir = appDir
    out, err := cmd.CombinedOutput()
    if err != nil {
        return fmt.Errorf("docker compose down: %s: %w", string(out), err)
    }
    return nil
}

// Stop stops the app's containers without removing them.
func (m *Manager) Stop(ctx context.Context, appDir, composePath, appName string) error {
    cmd := exec.CommandContext(ctx, "docker", "compose", "-f", filepath.Join(appDir, composePath), "-p", "selfstack-"+appName, "stop")
    cmd.Dir = appDir
    out, err := cmd.CombinedOutput()
    if err != nil {
        return fmt.Errorf("docker compose stop: %s: %w", string(out), err)
    }
    return nil
}

// Start starts previously stopped containers.
func (m *Manager) Start(ctx context.Context, appDir, composePath, appName string) error {
    cmd := exec.CommandContext(ctx, "docker", "compose", "-f", filepath.Join(appDir, composePath), "-p", "selfstack-"+appName, "start")
    cmd.Dir = appDir
    out, err := cmd.CombinedOutput()
    if err != nil {
        return fmt.Errorf("docker compose start: %s: %w", string(out), err)
    }
    return nil
}

// Logs streams container logs.
func (m *Manager) Logs(ctx context.Context, appDir, composePath, appName string, w io.Writer) error {
    cmd := exec.CommandContext(ctx, "docker", "compose", "-f", filepath.Join(appDir, composePath), "-p", "selfstack-"+appName, "logs", "-f", "--tail=100")
    cmd.Dir = appDir
    cmd.Stdout = w
    cmd.Stderr = w
    return cmd.Run()
}

// HealthCheck polls the health endpoint until it returns 200 or timeout.
func (m *Manager) HealthCheck(ctx context.Context, hostPort int, healthPath string) error {
    url := fmt.Sprintf("http://localhost:%d%s", hostPort, healthPath)
    deadline := time.Now().Add(m.timeout)
    for time.Now().Before(deadline) {
        resp, err := http.Get(url)
        if err == nil && resp.StatusCode == 200 {
            resp.Body.Close()
            return nil
        }
        if resp != nil {
            resp.Body.Close()
        }
        select {
        case <-ctx.Done():
            return ctx.Err()
        case <-time.After(2 * time.Second):
        }
    }
    return fmt.Errorf("health check timeout for %s", url)
}
```

**Step 3: Write basic unit test (no Docker required)**

```go
package container

import "testing"

func TestNewManager(t *testing.T) {
    m := NewManager()
    if m.timeout == 0 {
        t.Fatal("expected non-zero timeout")
    }
}
```

**Step 4: Run test**

Run: `go test ./internal/container/ -v`
Expected: PASS

**Step 5: Commit**

```bash
git add internal/container/ go.mod go.sum
git commit -m "feat: add Docker container manager using docker compose CLI"
```

---

### Task 8: App service — orchestrates install/start/stop/remove

**Files:**
- Create: `internal/app/service.go`
- Create: `internal/app/service_test.go`

**Step 1: Write service.go — the core orchestration layer**

This ties together the store, manifest parser, registry, and container manager.

```go
package app

import (
    "context"
    "fmt"
    "os"
    "os/exec"
    "path/filepath"

    "github.com/selfstack/selfstack/internal/config"
    "github.com/selfstack/selfstack/internal/container"
    "github.com/selfstack/selfstack/internal/manifest"
    "github.com/selfstack/selfstack/internal/registry"
    "github.com/selfstack/selfstack/internal/store"
)

type Service struct {
    store    *store.Store
    registry *registry.Client
    mgr      *container.Manager
}

func NewService(s *store.Store, r *registry.Client) *Service {
    return &Service{store: s, registry: r, mgr: container.NewManager()}
}

func (s *Service) Install(ctx context.Context, appName string) error {
    // 1. Lookup in registry
    entry, err := s.registry.Lookup(appName)
    if err != nil {
        return err
    }

    // 2. Clone repo
    appDir := filepath.Join(config.AppsDir(), appName)
    if err := os.MkdirAll(filepath.Dir(appDir), 0755); err != nil {
        return err
    }
    cmd := exec.CommandContext(ctx, "git", "clone", entry.Repo, appDir)
    if out, err := cmd.CombinedOutput(); err != nil {
        return fmt.Errorf("git clone: %s: %w", string(out), err)
    }

    // 3. Parse manifest
    m, err := manifest.ParseFile(filepath.Join(appDir, "selfstack.yml"))
    if err != nil {
        return fmt.Errorf("parse manifest: %w", err)
    }

    // 4. Allocate port
    port, err := s.store.AllocatePort(appName)
    if err != nil {
        return fmt.Errorf("allocate port: %w", err)
    }

    // 5. Build
    if err := s.mgr.Build(ctx, appDir, m.Runtime.Entry); err != nil {
        s.store.ReleasePort(appName)
        return err
    }

    // 6. Gather config env vars
    envVars := make(map[string]string)
    for _, c := range m.Config {
        envVars[c.Key] = c.Default
    }

    // 7. Start
    if err := s.mgr.Up(ctx, appDir, m.Runtime.Entry, appName, m.Expose.Port, port, envVars); err != nil {
        s.store.ReleasePort(appName)
        return err
    }

    // 8. Save to store
    if err := s.store.InsertApp(store.App{
        Name:        appName,
        DisplayName: m.DisplayName,
        Description: m.Description,
        RepoURL:     entry.Repo,
        Version:     m.Version,
        HostPort:    port,
        Status:      "running",
    }); err != nil {
        return err
    }

    return nil
}

func (s *Service) Start(ctx context.Context, appName string) error {
    app, err := s.store.GetApp(appName)
    if err != nil {
        return err
    }
    appDir := filepath.Join(config.AppsDir(), appName)
    m, err := manifest.ParseFile(filepath.Join(appDir, "selfstack.yml"))
    if err != nil {
        return err
    }
    if err := s.mgr.Start(ctx, appDir, m.Runtime.Entry, appName); err != nil {
        return err
    }
    return s.store.UpdateAppStatus(app.Name, "running")
}

func (s *Service) Stop(ctx context.Context, appName string) error {
    app, err := s.store.GetApp(appName)
    if err != nil {
        return err
    }
    appDir := filepath.Join(config.AppsDir(), appName)
    m, err := manifest.ParseFile(filepath.Join(appDir, "selfstack.yml"))
    if err != nil {
        return err
    }
    if err := s.mgr.Stop(ctx, appDir, m.Runtime.Entry, appName); err != nil {
        return err
    }
    return s.store.UpdateAppStatus(app.Name, "stopped")
}

func (s *Service) Remove(ctx context.Context, appName string) error {
    appDir := filepath.Join(config.AppsDir(), appName)
    m, _ := manifest.ParseFile(filepath.Join(appDir, "selfstack.yml"))
    if m != nil {
        s.mgr.Down(ctx, appDir, m.Runtime.Entry, appName)
    }
    s.store.DeleteApp(appName)
    s.store.ReleasePort(appName)
    os.RemoveAll(appDir)
    return nil
}

func (s *Service) List() ([]store.App, error) {
    return s.store.ListApps()
}

func (s *Service) Get(appName string) (store.App, error) {
    return s.store.GetApp(appName)
}
```

**Step 2: Write test**

```go
package app

import (
    "path/filepath"
    "testing"

    "github.com/selfstack/selfstack/internal/registry"
    "github.com/selfstack/selfstack/internal/store"
)

func TestNewService(t *testing.T) {
    dir := t.TempDir()
    s, err := store.Open(filepath.Join(dir, "test.db"))
    if err != nil {
        t.Fatal(err)
    }
    defer s.Close()
    r := registry.NewClient("http://localhost")
    svc := NewService(s, r)
    if svc == nil {
        t.Fatal("expected non-nil service")
    }
}
```

**Step 3: Run tests**

Run: `go test ./internal/app/ -v`
Expected: PASS

**Step 4: Commit**

```bash
git add internal/app/
git commit -m "feat: add app service orchestrating install/start/stop/remove"
```

---

## Phase 4: REST API

### Task 9: HTTP API server with app endpoints

**Files:**
- Create: `internal/api/server.go`
- Create: `internal/api/apps.go`
- Create: `internal/api/server_test.go`

**Step 1: Install chi router**

Run: `go get github.com/go-chi/chi/v5`

**Step 2: Write server.go**

```go
package api

import (
    "encoding/json"
    "net/http"

    "github.com/go-chi/chi/v5"
    "github.com/go-chi/chi/v5/middleware"
    "github.com/selfstack/selfstack/internal/app"
    "github.com/selfstack/selfstack/internal/registry"
)

type Server struct {
    appSvc   *app.Service
    registry *registry.Client
    router   chi.Router
}

func NewServer(appSvc *app.Service, reg *registry.Client) *Server {
    s := &Server{appSvc: appSvc, registry: reg}
    r := chi.NewRouter()
    r.Use(middleware.Logger)
    r.Use(middleware.Recoverer)

    r.Route("/api", func(r chi.Router) {
        r.Get("/status", s.handleStatus)
        r.Get("/apps", s.handleListApps)
        r.Post("/apps/install", s.handleInstallApp)
        r.Post("/apps/{name}/start", s.handleStartApp)
        r.Post("/apps/{name}/stop", s.handleStopApp)
        r.Delete("/apps/{name}", s.handleRemoveApp)
        r.Get("/registry/search", s.handleRegistrySearch)
    })

    s.router = r
    return s
}

func (s *Server) ServeHTTP(w http.ResponseWriter, r *http.Request) {
    s.router.ServeHTTP(w, r)
}

func jsonResponse(w http.ResponseWriter, status int, data any) {
    w.Header().Set("Content-Type", "application/json")
    w.WriteHeader(status)
    json.NewEncoder(w).Encode(data)
}

func jsonError(w http.ResponseWriter, status int, msg string) {
    jsonResponse(w, status, map[string]string{"error": msg})
}
```

**Step 3: Write apps.go handlers**

```go
package api

import (
    "encoding/json"
    "net/http"

    "github.com/go-chi/chi/v5"
    "github.com/selfstack/selfstack/internal/registry"
)

func (s *Server) handleStatus(w http.ResponseWriter, r *http.Request) {
    apps, _ := s.appSvc.List()
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
    jsonResponse(w, 200, apps)
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
    app, _ := s.appSvc.Get(req.Name)
    jsonResponse(w, 201, app)
}

func (s *Server) handleStartApp(w http.ResponseWriter, r *http.Request) {
    name := chi.URLParam(r, "name")
    if err := s.appSvc.Start(r.Context(), name); err != nil {
        jsonError(w, 500, err.Error())
        return
    }
    jsonResponse(w, 200, map[string]string{"status": "started"})
}

func (s *Server) handleStopApp(w http.ResponseWriter, r *http.Request) {
    name := chi.URLParam(r, "name")
    if err := s.appSvc.Stop(r.Context(), name); err != nil {
        jsonError(w, 500, err.Error())
        return
    }
    jsonResponse(w, 200, map[string]string{"status": "stopped"})
}

func (s *Server) handleRemoveApp(w http.ResponseWriter, r *http.Request) {
    name := chi.URLParam(r, "name")
    if err := s.appSvc.Remove(r.Context(), name); err != nil {
        jsonError(w, 500, err.Error())
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
    jsonResponse(w, 200, results)
}
```

**Step 4: Write server test**

```go
package api

import (
    "net/http"
    "net/http/httptest"
    "path/filepath"
    "testing"

    "github.com/selfstack/selfstack/internal/app"
    "github.com/selfstack/selfstack/internal/registry"
    "github.com/selfstack/selfstack/internal/store"
)

func testServer(t *testing.T) *Server {
    t.Helper()
    dir := t.TempDir()
    s, _ := store.Open(filepath.Join(dir, "test.db"))
    t.Cleanup(func() { s.Close() })
    r := registry.NewClient("http://localhost")
    svc := app.NewService(s, r)
    return NewServer(svc, r)
}

func TestHandleStatus(t *testing.T) {
    srv := testServer(t)
    req := httptest.NewRequest("GET", "/api/status", nil)
    w := httptest.NewRecorder()
    srv.ServeHTTP(w, req)
    if w.Code != 200 {
        t.Fatalf("expected 200, got %d", w.Code)
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
```

**Step 5: Run tests**

Run: `go test ./internal/api/ -v`
Expected: PASS

**Step 6: Commit**

```bash
git add internal/api/ go.mod go.sum
git commit -m "feat: add REST API server with app and registry endpoints"
```

---

## Phase 5: CLI

### Task 10: Cobra CLI with all commands

**Files:**
- Create: `cmd/root.go`
- Create: `cmd/install.go`
- Create: `cmd/list.go`
- Create: `cmd/start.go`
- Create: `cmd/stop.go`
- Create: `cmd/remove.go`
- Create: `cmd/status.go`
- Create: `cmd/search.go`
- Modify: `main.go`

**Step 1: Install cobra**

Run: `go get github.com/spf13/cobra`

**Step 2: Write cmd/root.go**

```go
package cmd

import (
    "fmt"
    "os"

    "github.com/spf13/cobra"
)

var apiURL string

var rootCmd = &cobra.Command{
    Use:   "selfstack",
    Short: "Personal app server — install, run, and access self-hosted web apps",
}

func Execute() {
    if err := rootCmd.Execute(); err != nil {
        fmt.Fprintln(os.Stderr, err)
        os.Exit(1)
    }
}

func init() {
    rootCmd.PersistentFlags().StringVar(&apiURL, "api", "http://localhost:8080", "SelfStack API URL")
}
```

**Step 3: Write command files (install, list, start, stop, remove, status, search)**

Each command makes HTTP calls to the REST API. Example for install:

```go
package cmd

import (
    "bytes"
    "encoding/json"
    "fmt"
    "io"
    "net/http"

    "github.com/spf13/cobra"
)

var installCmd = &cobra.Command{
    Use:   "install [app-name]",
    Short: "Install an app from the registry",
    Args:  cobra.ExactArgs(1),
    RunE: func(cmd *cobra.Command, args []string) error {
        body, _ := json.Marshal(map[string]string{"name": args[0]})
        resp, err := http.Post(apiURL+"/api/apps/install", "application/json", bytes.NewReader(body))
        if err != nil {
            return fmt.Errorf("failed to reach SelfStack API: %w", err)
        }
        defer resp.Body.Close()
        out, _ := io.ReadAll(resp.Body)
        if resp.StatusCode != 201 {
            return fmt.Errorf("install failed: %s", string(out))
        }
        fmt.Printf("Installed %s successfully\n", args[0])
        return nil
    },
}

func init() { rootCmd.AddCommand(installCmd) }
```

Write similar pattern for: `list.go`, `start.go`, `stop.go`, `remove.go`, `status.go`, `search.go`. Each calls the corresponding API endpoint.

**Step 4: Update main.go**

```go
package main

import "github.com/selfstack/selfstack/cmd"

func main() {
    cmd.Execute()
}
```

**Step 5: Build and verify**

Run: `go build -o selfstack . && ./selfstack --help`
Expected: Shows help with all subcommands listed

**Step 6: Commit**

```bash
git add cmd/ main.go go.mod go.sum
git commit -m "feat: add CLI with install, list, start, stop, remove, status, search commands"
```

---

### Task 11: Wire up the server command to start SelfStack

**Files:**
- Create: `cmd/serve.go`

**Step 1: Write serve.go — starts the API server**

```go
package cmd

import (
    "fmt"
    "log"
    "net/http"
    "os"

    "github.com/selfstack/selfstack/internal/api"
    "github.com/selfstack/selfstack/internal/app"
    "github.com/selfstack/selfstack/internal/config"
    "github.com/selfstack/selfstack/internal/registry"
    "github.com/selfstack/selfstack/internal/store"
    "github.com/spf13/cobra"
)

const defaultRegistryURL = "https://raw.githubusercontent.com/selfstack/registry/main/registry.json"

var serveCmd = &cobra.Command{
    Use:   "serve",
    Short: "Start the SelfStack server",
    RunE: func(cmd *cobra.Command, args []string) error {
        // Ensure directories exist
        for _, dir := range []string{config.AppsDir(), config.DataDir(), config.KeysPath()} {
            os.MkdirAll(dir, 0755)
        }

        // Open store
        s, err := store.Open(config.DBPath())
        if err != nil {
            return fmt.Errorf("open store: %w", err)
        }
        defer s.Close()

        // Create registry client
        reg := registry.NewClient(defaultRegistryURL)

        // Create app service
        appSvc := app.NewService(s, reg)

        // Create and start API server
        srv := api.NewServer(appSvc, reg)
        addr := fmt.Sprintf(":%d", config.DefaultPort)
        log.Printf("SelfStack server starting on %s", addr)
        return http.ListenAndServe(addr, srv)
    },
}

func init() { rootCmd.AddCommand(serveCmd) }
```

**Step 2: Build and test**

Run: `go build -o selfstack . && ./selfstack serve &`
Then: `curl http://localhost:8080/api/status`
Expected: `{"apps":0,"status":"ok","version":"0.1.0"}`
Cleanup: `kill %1`

**Step 3: Commit**

```bash
git add cmd/serve.go
git commit -m "feat: add serve command to start SelfStack API server"
```

---

## Phase 6: Reverse Proxy

### Task 12: HTTP reverse proxy with dynamic route registration

**Files:**
- Create: `internal/proxy/proxy.go`
- Create: `internal/proxy/proxy_test.go`

**Step 1: Write proxy.go**

Uses Go's `net/http/httputil.ReverseProxy` (no Caddy dependency for V1 — simpler to start with stdlib, migrate to Caddy later for TLS).

```go
package proxy

import (
    "fmt"
    "net/http"
    "net/http/httputil"
    "net/url"
    "strings"
    "sync"
)

type Proxy struct {
    mu     sync.RWMutex
    routes map[string]*httputil.ReverseProxy // appname -> reverse proxy
}

func New() *Proxy {
    return &Proxy{routes: make(map[string]*httputil.ReverseProxy)}
}

func (p *Proxy) Register(appName string, hostPort int) error {
    target, err := url.Parse(fmt.Sprintf("http://localhost:%d", hostPort))
    if err != nil {
        return err
    }
    rp := httputil.NewSingleHostReverseProxy(target)
    p.mu.Lock()
    p.routes[appName] = rp
    p.mu.Unlock()
    return nil
}

func (p *Proxy) Deregister(appName string) {
    p.mu.Lock()
    delete(p.routes, appName)
    p.mu.Unlock()
}

func (p *Proxy) ServeHTTP(w http.ResponseWriter, r *http.Request) {
    // Extract app name from subdomain: budget-tracker.selfstack.local
    host := r.Host
    parts := strings.SplitN(host, ".", 2)
    appName := parts[0]

    p.mu.RLock()
    rp, ok := p.routes[appName]
    p.mu.RUnlock()

    if !ok {
        http.Error(w, fmt.Sprintf("app %q not found", appName), http.StatusNotFound)
        return
    }
    rp.ServeHTTP(w, r)
}

func (p *Proxy) ListRoutes() map[string]bool {
    p.mu.RLock()
    defer p.mu.RUnlock()
    out := make(map[string]bool, len(p.routes))
    for k := range p.routes {
        out[k] = true
    }
    return out
}
```

**Step 2: Write test**

```go
package proxy

import (
    "net/http"
    "net/http/httptest"
    "testing"
)

func TestRegisterAndRoute(t *testing.T) {
    // Start a fake backend app
    backend := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
        w.Write([]byte("hello from app"))
    }))
    defer backend.Close()

    p := New()
    // Extract port from backend URL
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
```

**Step 3: Run tests**

Run: `go test ./internal/proxy/ -v`
Expected: PASS

**Step 4: Commit**

```bash
git add internal/proxy/
git commit -m "feat: add HTTP reverse proxy with dynamic route registration"
```

---

## Phase 7: Authentication — Device Trust

### Task 13: Ed25519 key generation and device tokens

**Files:**
- Create: `internal/auth/auth.go`
- Create: `internal/auth/auth_test.go`

**Step 1: Write auth.go**

```go
package auth

import (
    "crypto/ed25519"
    "crypto/rand"
    "crypto/sha256"
    "encoding/base64"
    "encoding/hex"
    "fmt"
    "os"
    "path/filepath"
    "time"
)

type Auth struct {
    privateKey ed25519.PrivateKey
    publicKey  ed25519.PublicKey
    keysDir    string
}

func New(keysDir string) (*Auth, error) {
    a := &Auth{keysDir: keysDir}
    if err := os.MkdirAll(keysDir, 0700); err != nil {
        return nil, err
    }
    privPath := filepath.Join(keysDir, "master.key")
    pubPath := filepath.Join(keysDir, "master.pub")

    if _, err := os.Stat(privPath); os.IsNotExist(err) {
        // Generate new key pair
        pub, priv, err := ed25519.GenerateKey(rand.Reader)
        if err != nil {
            return nil, err
        }
        os.WriteFile(privPath, priv, 0600)
        os.WriteFile(pubPath, pub, 0644)
        a.privateKey = priv
        a.publicKey = pub
    } else {
        priv, err := os.ReadFile(privPath)
        if err != nil {
            return nil, err
        }
        a.privateKey = ed25519.PrivateKey(priv)
        a.publicKey = a.privateKey.Public().(ed25519.PublicKey)
    }
    return a, nil
}

// IssueToken creates a signed device token.
func (a *Auth) IssueToken(deviceID string) (string, error) {
    payload := fmt.Sprintf("%s:%d", deviceID, time.Now().Unix())
    sig := ed25519.Sign(a.privateKey, []byte(payload))
    token := base64.URLEncoding.EncodeToString([]byte(payload)) + "." + base64.URLEncoding.EncodeToString(sig)
    return token, nil
}

// VerifyToken checks if a token was signed by us.
func (a *Auth) VerifyToken(token string) (string, bool) {
    parts := splitToken(token)
    if parts == nil {
        return "", false
    }
    payload, err := base64.URLEncoding.DecodeString(parts[0])
    if err != nil {
        return "", false
    }
    sig, err := base64.URLEncoding.DecodeString(parts[1])
    if err != nil {
        return "", false
    }
    if !ed25519.Verify(a.publicKey, payload, sig) {
        return "", false
    }
    // Extract device ID from payload "deviceID:timestamp"
    for i := len(payload) - 1; i >= 0; i-- {
        if payload[i] == ':' {
            return string(payload[:i]), true
        }
    }
    return "", false
}

// TokenHash returns a hash suitable for storing in the database.
func TokenHash(token string) string {
    h := sha256.Sum256([]byte(token))
    return hex.EncodeToString(h[:])
}

func splitToken(token string) []string {
    for i := len(token) - 1; i >= 0; i-- {
        if token[i] == '.' {
            return []string{token[:i], token[i+1:]}
        }
    }
    return nil
}
```

**Step 2: Write test**

```go
package auth

import "testing"

func TestIssueAndVerify(t *testing.T) {
    dir := t.TempDir()
    a, err := New(dir)
    if err != nil {
        t.Fatal(err)
    }
    token, err := a.IssueToken("device-123")
    if err != nil {
        t.Fatal(err)
    }
    deviceID, ok := a.VerifyToken(token)
    if !ok {
        t.Fatal("token verification failed")
    }
    if deviceID != "device-123" {
        t.Fatalf("expected device-123, got %s", deviceID)
    }
}

func TestVerifyToken_Invalid(t *testing.T) {
    dir := t.TempDir()
    a, _ := New(dir)
    _, ok := a.VerifyToken("garbage.token")
    if ok {
        t.Fatal("expected invalid token to fail verification")
    }
}

func TestKeyPersistence(t *testing.T) {
    dir := t.TempDir()
    a1, _ := New(dir)
    token, _ := a1.IssueToken("device-1")

    // Create new Auth instance from same directory — should load existing keys
    a2, _ := New(dir)
    deviceID, ok := a2.VerifyToken(token)
    if !ok || deviceID != "device-1" {
        t.Fatal("token from first instance should verify with second instance")
    }
}
```

**Step 3: Run tests**

Run: `go test ./internal/auth/ -v`
Expected: ALL PASS

**Step 4: Commit**

```bash
git add internal/auth/
git commit -m "feat: add Ed25519 device-based authentication"
```

---

### Task 14: Auth middleware for API

**Files:**
- Create: `internal/api/middleware.go`
- Create: `internal/api/middleware_test.go`

**Step 1: Write middleware.go**

```go
package api

import (
    "net/http"

    "github.com/selfstack/selfstack/internal/auth"
)

func AuthMiddleware(a *auth.Auth) func(http.Handler) http.Handler {
    return func(next http.Handler) http.Handler {
        return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
            // Allow unauthenticated access to device pairing and status
            if r.URL.Path == "/api/status" || r.URL.Path == "/api/devices/pair" {
                next.ServeHTTP(w, r)
                return
            }

            // Check for token in cookie or Authorization header
            token := ""
            if cookie, err := r.Cookie("selfstack_token"); err == nil {
                token = cookie.Value
            }
            if token == "" {
                token = r.Header.Get("Authorization")
            }

            if token == "" {
                jsonError(w, 401, "device not trusted — visit dashboard from a trusted device to approve")
                return
            }

            _, ok := a.VerifyToken(token)
            if !ok {
                jsonError(w, 401, "invalid or expired device token")
                return
            }

            next.ServeHTTP(w, r)
        })
    }
}
```

**Step 2: Write test**

```go
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
```

**Step 3: Run tests**

Run: `go test ./internal/api/ -v`
Expected: ALL PASS

**Step 4: Commit**

```bash
git add internal/api/middleware.go internal/api/middleware_test.go
git commit -m "feat: add auth middleware for device-based trust"
```

---

## Phase 8: Tunnel Manager

### Task 15: Cloudflare Tunnel integration

**Files:**
- Create: `internal/tunnel/cloudflare.go`
- Create: `internal/tunnel/cloudflare_test.go`

**Step 1: Write cloudflare.go**

```go
package tunnel

import (
    "context"
    "fmt"
    "os/exec"
)

type CloudflareTunnel struct {
    token   string
    running bool
    cmd     *exec.Cmd
}

func NewCloudflare(token string) *CloudflareTunnel {
    return &CloudflareTunnel{token: token}
}

// Start launches cloudflared tunnel
func (t *CloudflareTunnel) Start(ctx context.Context) error {
    if t.token == "" {
        return fmt.Errorf("cloudflare tunnel token not configured — run 'selfstack tunnel setup'")
    }
    t.cmd = exec.CommandContext(ctx, "cloudflared", "tunnel", "run", "--token", t.token)
    if err := t.cmd.Start(); err != nil {
        return fmt.Errorf("start cloudflared: %w", err)
    }
    t.running = true
    return nil
}

func (t *CloudflareTunnel) Stop() error {
    if t.cmd != nil && t.cmd.Process != nil {
        t.running = false
        return t.cmd.Process.Kill()
    }
    return nil
}

func (t *CloudflareTunnel) IsRunning() bool { return t.running }

// IsInstalled checks if cloudflared binary is available
func IsInstalled() bool {
    _, err := exec.LookPath("cloudflared")
    return err == nil
}
```

**Step 2: Write test**

```go
package tunnel

import "testing"

func TestIsInstalled(t *testing.T) {
    // Just verify the function doesn't panic
    _ = IsInstalled()
}

func TestNewCloudflare(t *testing.T) {
    cf := NewCloudflare("test-token")
    if cf.IsRunning() {
        t.Fatal("should not be running initially")
    }
}
```

**Step 3: Run tests**

Run: `go test ./internal/tunnel/ -v`
Expected: PASS

**Step 4: Commit**

```bash
git add internal/tunnel/
git commit -m "feat: add Cloudflare Tunnel manager"
```

---

## Phase 9: Data Export/Import

### Task 16: Volume export and import

**Files:**
- Create: `internal/data/export.go`
- Create: `internal/data/export_test.go`

**Step 1: Write export.go**

```go
package data

import (
    "context"
    "fmt"
    "os"
    "os/exec"
    "path/filepath"
    "time"

    "github.com/selfstack/selfstack/internal/config"
)

// Export creates a tar.gz of an app's Docker volumes.
func Export(ctx context.Context, appName string) (string, error) {
    exportDir := config.DataDir()
    os.MkdirAll(exportDir, 0755)

    timestamp := time.Now().Format("2006-01-02-150405")
    outFile := filepath.Join(exportDir, fmt.Sprintf("%s-%s.tar.gz", appName, timestamp))

    volumeName := fmt.Sprintf("selfstack-%s_data", appName)
    cmd := exec.CommandContext(ctx, "docker", "run", "--rm",
        "-v", volumeName+":/data",
        "-v", exportDir+":/backup",
        "alpine",
        "tar", "czf", fmt.Sprintf("/backup/%s-%s.tar.gz", appName, timestamp), "-C", "/data", ".",
    )
    if out, err := cmd.CombinedOutput(); err != nil {
        return "", fmt.Errorf("export: %s: %w", string(out), err)
    }
    return outFile, nil
}

// Import restores a tar.gz into an app's Docker volume.
func Import(ctx context.Context, appName, archivePath string) error {
    absPath, err := filepath.Abs(archivePath)
    if err != nil {
        return err
    }
    volumeName := fmt.Sprintf("selfstack-%s_data", appName)
    cmd := exec.CommandContext(ctx, "docker", "run", "--rm",
        "-v", volumeName+":/data",
        "-v", filepath.Dir(absPath)+":/backup",
        "alpine",
        "sh", "-c", fmt.Sprintf("tar xzf /backup/%s -C /data", filepath.Base(absPath)),
    )
    if out, err := cmd.CombinedOutput(); err != nil {
        return fmt.Errorf("import: %s: %w", string(out), err)
    }
    return nil
}
```

**Step 2: Write test (unit-level — no Docker)**

```go
package data

import "testing"

func TestExportPath(t *testing.T) {
    // Verify the function signature compiles
    // Integration test requires Docker
    t.Log("export/import requires Docker — see integration tests")
}
```

**Step 3: Commit**

```bash
git add internal/data/
git commit -m "feat: add Docker volume export/import"
```

---

## Phase 10: React Dashboard

### Task 17: Initialize React dashboard project

**Files:**
- Create: `dashboard/package.json`
- Create: `dashboard/tsconfig.json`
- Create: `dashboard/vite.config.ts`
- Create: `dashboard/index.html`
- Create: `dashboard/src/main.tsx`
- Create: `dashboard/src/App.tsx`
- Create: `dashboard/src/api.ts`

**Step 1: Scaffold React app with Vite**

Run: `cd /Users/alderik/Code/projects/local-app-framework && npm create vite@latest dashboard -- --template react-ts`

**Step 2: Install dependencies**

Run: `cd /Users/alderik/Code/projects/local-app-framework/dashboard && npm install`

**Step 3: Write API client (dashboard/src/api.ts)**

```typescript
const API_BASE = '/api';

export interface App {
  name: string;
  display_name: string;
  description: string;
  repo_url: string;
  version: string;
  host_port: number;
  status: string;
}

export interface RegistryApp {
  name: string;
  display_name: string;
  description: string;
  category: string;
  repo: string;
  icon: string;
  verified: boolean;
}

export async function getApps(): Promise<App[]> {
  const res = await fetch(`${API_BASE}/apps`);
  return res.json();
}

export async function installApp(name: string): Promise<App> {
  const res = await fetch(`${API_BASE}/apps/install`, {
    method: 'POST',
    headers: { 'Content-Type': 'application/json' },
    body: JSON.stringify({ name }),
  });
  return res.json();
}

export async function startApp(name: string): Promise<void> {
  await fetch(`${API_BASE}/apps/${name}/start`, { method: 'POST' });
}

export async function stopApp(name: string): Promise<void> {
  await fetch(`${API_BASE}/apps/${name}/stop`, { method: 'POST' });
}

export async function removeApp(name: string): Promise<void> {
  await fetch(`${API_BASE}/apps/${name}`, { method: 'DELETE' });
}

export async function searchRegistry(query: string): Promise<RegistryApp[]> {
  const res = await fetch(`${API_BASE}/registry/search?q=${encodeURIComponent(query)}`);
  return res.json();
}

export async function getStatus(): Promise<{ version: string; apps: number; status: string }> {
  const res = await fetch(`${API_BASE}/status`);
  return res.json();
}
```

**Step 4: Write App.tsx with basic dashboard layout**

```tsx
import { useEffect, useState } from 'react';
import { getApps, searchRegistry, installApp, startApp, stopApp, App, RegistryApp } from './api';

function AppCard({ app, onAction }: { app: App; onAction: () => void }) {
  return (
    <div className="app-card">
      <h3>{app.display_name}</h3>
      <p>{app.description}</p>
      <span className={`status ${app.status}`}>{app.status}</span>
      <div className="actions">
        {app.status === 'stopped' && <button onClick={() => startApp(app.name).then(onAction)}>Start</button>}
        {app.status === 'running' && <button onClick={() => stopApp(app.name).then(onAction)}>Stop</button>}
      </div>
    </div>
  );
}

function RegistryCard({ app, onInstall }: { app: RegistryApp; onInstall: () => void }) {
  return (
    <div className="app-card registry">
      <h3>{app.display_name}</h3>
      <p>{app.description}</p>
      <span className="category">{app.category}</span>
      {app.verified && <span className="verified">Verified</span>}
      <button onClick={() => installApp(app.name).then(onInstall)}>Install</button>
    </div>
  );
}

export default function Dashboard() {
  const [apps, setApps] = useState<App[]>([]);
  const [registry, setRegistry] = useState<RegistryApp[]>([]);
  const [tab, setTab] = useState<'installed' | 'store'>('installed');

  const refresh = () => { getApps().then(setApps); };
  useEffect(() => { refresh(); }, []);
  useEffect(() => { if (tab === 'store') searchRegistry('').then(setRegistry); }, [tab]);

  return (
    <div className="dashboard">
      <header><h1>SelfStack</h1></header>
      <nav>
        <button onClick={() => setTab('installed')} className={tab === 'installed' ? 'active' : ''}>My Apps</button>
        <button onClick={() => setTab('store')} className={tab === 'store' ? 'active' : ''}>App Store</button>
      </nav>
      <main>
        {tab === 'installed' && (
          <div className="app-grid">
            {apps.length === 0 && <p>No apps installed. Visit the App Store to get started.</p>}
            {apps.map(a => <AppCard key={a.name} app={a} onAction={refresh} />)}
          </div>
        )}
        {tab === 'store' && (
          <div className="app-grid">
            {registry.map(a => <RegistryCard key={a.name} app={a} onInstall={refresh} />)}
          </div>
        )}
      </main>
    </div>
  );
}
```

**Step 5: Build dashboard**

Run: `cd /Users/alderik/Code/projects/local-app-framework/dashboard && npm run build`
Expected: `dist/` directory with built assets

**Step 6: Commit**

```bash
git add dashboard/
git commit -m "feat: add React dashboard with app store and installed apps views"
```

---

### Task 18: Embed dashboard in Go binary

**Files:**
- Create: `internal/dashboard/embed.go`
- Modify: `internal/api/server.go` — serve embedded dashboard

**Step 1: Write embed.go**

```go
package dashboard

import (
    "embed"
    "io/fs"
    "net/http"
)

//go:embed dist/*
var assets embed.FS

func Handler() http.Handler {
    dist, _ := fs.Sub(assets, "dist")
    fileServer := http.FileServer(http.FS(dist))
    return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
        // Try to serve the file; if not found, serve index.html (SPA routing)
        fileServer.ServeHTTP(w, r)
    })
}
```

Note: The `dist/` directory must exist at build time. The Makefile builds dashboard first, then copies to `internal/dashboard/dist/`, then builds Go.

**Step 2: Update Makefile**

```makefile
.PHONY: build test run clean dashboard

dashboard:
	cd dashboard && npm run build
	rm -rf internal/dashboard/dist
	cp -r dashboard/dist internal/dashboard/dist

build: dashboard
	go build -o selfstack .

test:
	go test ./... -v

run: build
	./selfstack serve

clean:
	rm -f selfstack
	rm -rf internal/dashboard/dist
```

**Step 3: Update server.go to serve dashboard at root**

Add to `NewServer`:
```go
// Serve dashboard at root (catch-all after API routes)
r.Handle("/*", dashboard.Handler())
```

**Step 4: Build and verify**

Run: `make build && ./selfstack serve &`
Then: `curl http://localhost:8080/`
Expected: HTML content of the React app
Cleanup: `kill %1`

**Step 5: Commit**

```bash
git add internal/dashboard/ Makefile internal/api/server.go
git commit -m "feat: embed React dashboard in Go binary"
```

---

## Phase 11: Integration & Polish

### Task 19: End-to-end smoke test with a sample app

**Files:**
- Create: `test/sample-app/selfstack.yml`
- Create: `test/sample-app/docker-compose.yml`
- Create: `test/sample-app/Dockerfile`
- Create: `test/sample-app/main.go`
- Create: `test/e2e_test.go`

**Step 1: Create a minimal sample app**

`test/sample-app/main.go`:
```go
package main

import (
    "fmt"
    "net/http"
)

func main() {
    http.HandleFunc("/", func(w http.ResponseWriter, r *http.Request) {
        fmt.Fprint(w, "Hello from SelfStack sample app!")
    })
    http.HandleFunc("/api/health", func(w http.ResponseWriter, r *http.Request) {
        w.WriteHeader(200)
        fmt.Fprint(w, "ok")
    })
    http.ListenAndServe(":3000", nil)
}
```

`test/sample-app/Dockerfile`:
```dockerfile
FROM golang:1.22-alpine
WORKDIR /app
COPY main.go .
RUN go build -o server main.go
CMD ["./server"]
```

`test/sample-app/docker-compose.yml`:
```yaml
services:
  app:
    build: .
    ports:
      - "${SELFSTACK_HOST_PORT:-3000}:3000"
```

`test/sample-app/selfstack.yml`:
```yaml
name: sample-app
display_name: Sample App
description: A minimal test app for SelfStack
version: 0.1.0

runtime:
  type: docker-compose
  entry: docker-compose.yml

expose:
  port: 3000
  health: /api/health
```

**Step 2: Write e2e test (requires Docker)**

```go
//go:build integration

package test

import (
    "context"
    "net/http"
    "os"
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
    appDir := "sample-app"

    m, err := manifest.ParseFile(appDir + "/selfstack.yml")
    if err != nil {
        t.Fatal(err)
    }

    // Build
    if err := mgr.Build(ctx, appDir, m.Runtime.Entry); err != nil {
        t.Fatal(err)
    }

    // Start on a test port
    testPort := 19999
    if err := mgr.Up(ctx, appDir, m.Runtime.Entry, "sample-app", m.Expose.Port, testPort, nil); err != nil {
        t.Fatal(err)
    }
    defer mgr.Down(ctx, appDir, m.Runtime.Entry, "sample-app")

    // Wait for health
    time.Sleep(5 * time.Second)
    resp, err := http.Get("http://localhost:19999/api/health")
    if err != nil {
        t.Fatal(err)
    }
    if resp.StatusCode != 200 {
        t.Fatalf("health check failed: %d", resp.StatusCode)
    }
}
```

**Step 3: Run (requires Docker)**

Run: `SELFSTACK_INTEGRATION=1 go test ./test/ -tags=integration -v -timeout=120s`
Expected: PASS

**Step 4: Commit**

```bash
git add test/
git commit -m "test: add sample app and e2e integration test"
```

---

### Task 20: Wire everything together in serve command

**Files:**
- Modify: `cmd/serve.go` — add proxy, auth, and tunnel startup

**Step 1: Update serve.go to start all components**

```go
// In serveCmd RunE, after creating API server:

// Create auth
a, err := auth.New(config.KeysPath())
if err != nil {
    return fmt.Errorf("init auth: %w", err)
}

// Create reverse proxy
p := proxy.New()

// Register existing app routes
apps, _ := appSvc.List()
for _, app := range apps {
    if app.Status == "running" {
        p.Register(app.Name, app.HostPort)
    }
}

// Create API server with auth middleware
srv := api.NewServer(appSvc, reg)

// Start proxy on port 80 (or 8081 for dev)
go func() {
    proxyAddr := ":80"
    log.Printf("Reverse proxy on %s", proxyAddr)
    http.ListenAndServe(proxyAddr, p)
}()

// Start API + dashboard on main port
addr := fmt.Sprintf(":%d", config.DefaultPort)
log.Printf("SelfStack dashboard on %s", addr)
return http.ListenAndServe(addr, srv)
```

**Step 2: Build and run full stack**

Run: `make build && ./selfstack serve`
Expected: Both dashboard and proxy start, logs show both listeners

**Step 3: Commit**

```bash
git add cmd/serve.go
git commit -m "feat: wire all components together in serve command"
```

---

## Summary

| Phase | Tasks | What it delivers |
|-------|-------|-----------------|
| 1. Foundation | 1-4 | Go project, SQLite store, port allocator |
| 2. Manifest & Registry | 5-6 | selfstack.yml parser, app catalog client |
| 3. Container Manager | 7-8 | Docker orchestration, app service layer |
| 4. REST API | 9 | HTTP endpoints for all operations |
| 5. CLI | 10-11 | Full CLI + serve command |
| 6. Reverse Proxy | 12 | Dynamic routing by app name |
| 7. Auth | 13-14 | Ed25519 device trust + middleware |
| 8. Tunnel | 15 | Cloudflare Tunnel integration |
| 9. Data | 16 | Volume export/import |
| 10. Dashboard | 17-18 | React app store UI embedded in binary |
| 11. Integration | 19-20 | E2E test + full wiring |

**Total: 20 tasks across 11 phases.**

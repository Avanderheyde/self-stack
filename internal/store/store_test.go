package store

import (
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

	tables := []string{"apps", "devices", "app_config", "port_allocations"}
	for _, table := range tables {
		var name string
		err := s.db.QueryRow(
			"SELECT name FROM sqlite_master WHERE type='table' AND name=?", table,
		).Scan(&name)
		if err != nil {
			t.Errorf("table %q not created: %v", table, err)
		}
	}
}

// --- App CRUD tests ---

func TestInsertAndGetApp(t *testing.T) {
	s := testDB(t)

	app := App{
		Name:        "myapp",
		DisplayName: "My App",
		Description: "A test app",
		RepoURL:     "https://github.com/test/myapp",
		Version:     "1.0.0",
		HostPort:    10001,
		Status:      "stopped",
	}

	if err := s.InsertApp(app); err != nil {
		t.Fatalf("InsertApp: %v", err)
	}

	got, err := s.GetApp("myapp")
	if err != nil {
		t.Fatalf("GetApp: %v", err)
	}
	if got.Name != app.Name {
		t.Errorf("Name = %q, want %q", got.Name, app.Name)
	}
	if got.DisplayName != app.DisplayName {
		t.Errorf("DisplayName = %q, want %q", got.DisplayName, app.DisplayName)
	}
	if got.Description != app.Description {
		t.Errorf("Description = %q, want %q", got.Description, app.Description)
	}
	if got.RepoURL != app.RepoURL {
		t.Errorf("RepoURL = %q, want %q", got.RepoURL, app.RepoURL)
	}
	if got.Version != app.Version {
		t.Errorf("Version = %q, want %q", got.Version, app.Version)
	}
	if got.HostPort != app.HostPort {
		t.Errorf("HostPort = %d, want %d", got.HostPort, app.HostPort)
	}
	if got.Status != app.Status {
		t.Errorf("Status = %q, want %q", got.Status, app.Status)
	}
}

func TestGetApp_NotFound(t *testing.T) {
	s := testDB(t)

	_, err := s.GetApp("nonexistent")
	if err == nil {
		t.Fatal("expected error for nonexistent app, got nil")
	}
}

func TestInsertApp_Duplicate(t *testing.T) {
	s := testDB(t)

	app := App{
		Name:        "dup",
		DisplayName: "Dup",
		RepoURL:     "https://example.com/dup",
		HostPort:    10001,
		Status:      "stopped",
	}
	if err := s.InsertApp(app); err != nil {
		t.Fatal(err)
	}
	if err := s.InsertApp(app); err == nil {
		t.Fatal("expected error inserting duplicate app")
	}
}

func TestListApps(t *testing.T) {
	s := testDB(t)

	apps := []App{
		{Name: "charlie", DisplayName: "Charlie", RepoURL: "https://example.com/c", HostPort: 10003, Status: "stopped"},
		{Name: "alpha", DisplayName: "Alpha", RepoURL: "https://example.com/a", HostPort: 10001, Status: "stopped"},
		{Name: "bravo", DisplayName: "Bravo", RepoURL: "https://example.com/b", HostPort: 10002, Status: "running"},
	}
	for _, a := range apps {
		if err := s.InsertApp(a); err != nil {
			t.Fatal(err)
		}
	}

	got, err := s.ListApps()
	if err != nil {
		t.Fatalf("ListApps: %v", err)
	}
	if len(got) != 3 {
		t.Fatalf("ListApps returned %d apps, want 3", len(got))
	}
	// Should be ordered by name
	if got[0].Name != "alpha" || got[1].Name != "bravo" || got[2].Name != "charlie" {
		t.Errorf("ListApps order = [%s, %s, %s], want [alpha, bravo, charlie]",
			got[0].Name, got[1].Name, got[2].Name)
	}
}

func TestDeleteApp(t *testing.T) {
	s := testDB(t)

	app := App{Name: "todelete", DisplayName: "Del", RepoURL: "https://example.com/d", HostPort: 10001, Status: "stopped"}
	if err := s.InsertApp(app); err != nil {
		t.Fatal(err)
	}

	if err := s.DeleteApp("todelete"); err != nil {
		t.Fatalf("DeleteApp: %v", err)
	}

	_, err := s.GetApp("todelete")
	if err == nil {
		t.Fatal("expected error after deleting app")
	}
}

func TestUpdateAppStatus(t *testing.T) {
	s := testDB(t)

	app := App{Name: "statusapp", DisplayName: "Status", RepoURL: "https://example.com/s", HostPort: 10001, Status: "stopped"}
	if err := s.InsertApp(app); err != nil {
		t.Fatal(err)
	}

	if err := s.UpdateAppStatus("statusapp", "running"); err != nil {
		t.Fatalf("UpdateAppStatus: %v", err)
	}

	got, err := s.GetApp("statusapp")
	if err != nil {
		t.Fatal(err)
	}
	if got.Status != "running" {
		t.Errorf("Status = %q, want %q", got.Status, "running")
	}
}

func TestUpdateAppStatus_NotFound(t *testing.T) {
	s := testDB(t)

	err := s.UpdateAppStatus("ghost", "running")
	if err == nil {
		t.Fatal("expected error updating nonexistent app")
	}
}

// --- Port allocator tests ---

func TestAllocatePort_StartsAt10001(t *testing.T) {
	s := testDB(t)

	port, err := s.AllocatePort("app1")
	if err != nil {
		t.Fatalf("AllocatePort: %v", err)
	}
	if port != 10001 {
		t.Errorf("first port = %d, want 10001", port)
	}
}

func TestAllocatePort_Sequential(t *testing.T) {
	s := testDB(t)

	p1, err := s.AllocatePort("app1")
	if err != nil {
		t.Fatal(err)
	}
	p2, err := s.AllocatePort("app2")
	if err != nil {
		t.Fatal(err)
	}
	p3, err := s.AllocatePort("app3")
	if err != nil {
		t.Fatal(err)
	}

	if p1 != 10001 || p2 != 10002 || p3 != 10003 {
		t.Errorf("ports = [%d, %d, %d], want [10001, 10002, 10003]", p1, p2, p3)
	}
}

func TestAllocatePort_GapFilling(t *testing.T) {
	s := testDB(t)

	s.AllocatePort("app1") // 10001
	s.AllocatePort("app2") // 10002
	s.AllocatePort("app3") // 10003

	if err := s.ReleasePort("app2"); err != nil {
		t.Fatalf("ReleasePort: %v", err)
	}

	port, err := s.AllocatePort("app4")
	if err != nil {
		t.Fatal(err)
	}
	if port != 10002 {
		t.Errorf("gap-filled port = %d, want 10002", port)
	}
}

func TestReleasePort(t *testing.T) {
	s := testDB(t)

	s.AllocatePort("app1")

	if err := s.ReleasePort("app1"); err != nil {
		t.Fatalf("ReleasePort: %v", err)
	}

	_, err := s.GetPort("app1")
	if err == nil {
		t.Fatal("expected error after releasing port")
	}
}

func TestGetPort(t *testing.T) {
	s := testDB(t)

	s.AllocatePort("app1")

	port, err := s.GetPort("app1")
	if err != nil {
		t.Fatalf("GetPort: %v", err)
	}
	if port != 10001 {
		t.Errorf("GetPort = %d, want 10001", port)
	}
}

func TestGetPort_NotFound(t *testing.T) {
	s := testDB(t)

	_, err := s.GetPort("noapp")
	if err == nil {
		t.Fatal("expected error for nonexistent port allocation")
	}
}

func TestAllocatePort_DuplicateApp(t *testing.T) {
	s := testDB(t)

	if _, err := s.AllocatePort("app1"); err != nil {
		t.Fatal(err)
	}
	if _, err := s.AllocatePort("app1"); err == nil {
		t.Fatal("expected error allocating port for same app twice")
	}
}

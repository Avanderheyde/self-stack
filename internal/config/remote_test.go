package config

import (
	"os"
	"path/filepath"
	"testing"
)

func TestLoadRemoteConfig_NoFile(t *testing.T) {
	dir := t.TempDir()
	t.Setenv("SELFSTACK_HOME", dir)

	rc, err := LoadRemoteConfig()
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if rc.Host != "" {
		t.Errorf("expected empty host, got %q", rc.Host)
	}
	if rc.IsConfigured() {
		t.Error("expected not configured")
	}
}

func TestSaveAndLoadRemoteConfig(t *testing.T) {
	dir := t.TempDir()
	t.Setenv("SELFSTACK_HOME", dir)

	rc := RemoteConfig{
		Host:    "deploy@192.168.1.100",
		APIPort: 8080,
	}
	if err := SaveRemoteConfig(rc); err != nil {
		t.Fatalf("save: %v", err)
	}

	// Verify file was created
	path := filepath.Join(dir, "remote.yml")
	if _, err := os.Stat(path); err != nil {
		t.Fatalf("config file not created: %v", err)
	}

	loaded, err := LoadRemoteConfig()
	if err != nil {
		t.Fatalf("load: %v", err)
	}
	if loaded.Host != "deploy@192.168.1.100" {
		t.Errorf("host = %q, want %q", loaded.Host, "deploy@192.168.1.100")
	}
	if loaded.APIPort != 8080 {
		t.Errorf("api_port = %d, want 8080", loaded.APIPort)
	}
	if !loaded.IsConfigured() {
		t.Error("expected configured")
	}
}

func TestRemoteConfig_APIURL(t *testing.T) {
	rc := RemoteConfig{Host: "user@10.0.0.1", APIPort: 8080}
	want := "http://10.0.0.1:8080"
	if got := rc.APIURL(); got != want {
		t.Errorf("APIURL() = %q, want %q", got, want)
	}
}

func TestRemoteConfig_APIURL_DefaultPort(t *testing.T) {
	rc := RemoteConfig{Host: "user@10.0.0.1"}
	want := "http://10.0.0.1:8080"
	if got := rc.APIURL(); got != want {
		t.Errorf("APIURL() = %q, want %q", got, want)
	}
}

func TestRemoteConfig_SSHTarget(t *testing.T) {
	rc := RemoteConfig{Host: "deploy@myserver.com"}
	if got := rc.SSHTarget(); got != "deploy@myserver.com" {
		t.Errorf("SSHTarget() = %q, want %q", got, "deploy@myserver.com")
	}
}

func TestRemoteConfig_Hostname(t *testing.T) {
	tests := []struct {
		host string
		want string
	}{
		{"user@10.0.0.1", "10.0.0.1"},
		{"10.0.0.1", "10.0.0.1"},
		{"deploy@myserver.com", "myserver.com"},
	}
	for _, tt := range tests {
		rc := RemoteConfig{Host: tt.host}
		if got := rc.Hostname(); got != tt.want {
			t.Errorf("Hostname(%q) = %q, want %q", tt.host, got, tt.want)
		}
	}
}

package manifest

import (
	"testing"
)

func TestParse(t *testing.T) {
	input := []byte(`
name: ghost
display_name: Ghost Blog
description: A publishing platform
version: "5.0"
icon: ghost.png
runtime:
  type: docker-compose
  entry: docker-compose.yml
expose:
  port: 2368
  health: /ghost/api/v4/admin/site/
volumes:
  - ghost_data:/var/lib/ghost/content
config:
  - key: GHOST_URL
    default: http://localhost:2368
    description: Public URL
`)
	m, err := Parse(input)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if m.Name != "ghost" {
		t.Errorf("Name = %q, want %q", m.Name, "ghost")
	}
	if m.DisplayName != "Ghost Blog" {
		t.Errorf("DisplayName = %q, want %q", m.DisplayName, "Ghost Blog")
	}
	if m.Description != "A publishing platform" {
		t.Errorf("Description = %q, want %q", m.Description, "A publishing platform")
	}
	if m.Version != "5.0" {
		t.Errorf("Version = %q, want %q", m.Version, "5.0")
	}
	if m.Icon != "ghost.png" {
		t.Errorf("Icon = %q, want %q", m.Icon, "ghost.png")
	}
	if m.Runtime.Type != "docker-compose" {
		t.Errorf("Runtime.Type = %q, want %q", m.Runtime.Type, "docker-compose")
	}
	if m.Runtime.Entry != "docker-compose.yml" {
		t.Errorf("Runtime.Entry = %q, want %q", m.Runtime.Entry, "docker-compose.yml")
	}
	if m.Expose.Port != 2368 {
		t.Errorf("Expose.Port = %d, want %d", m.Expose.Port, 2368)
	}
	if m.Expose.Health != "/ghost/api/v4/admin/site/" {
		t.Errorf("Expose.Health = %q, want %q", m.Expose.Health, "/ghost/api/v4/admin/site/")
	}
	if len(m.Volumes) != 1 || m.Volumes[0] != "ghost_data:/var/lib/ghost/content" {
		t.Errorf("Volumes = %v, want [ghost_data:/var/lib/ghost/content]", m.Volumes)
	}
	if len(m.Config) != 1 {
		t.Fatalf("Config length = %d, want 1", len(m.Config))
	}
	if m.Config[0].Key != "GHOST_URL" {
		t.Errorf("Config[0].Key = %q, want %q", m.Config[0].Key, "GHOST_URL")
	}
	if m.Config[0].Default != "http://localhost:2368" {
		t.Errorf("Config[0].Default = %q, want %q", m.Config[0].Default, "http://localhost:2368")
	}
	if m.Config[0].Description != "Public URL" {
		t.Errorf("Config[0].Description = %q, want %q", m.Config[0].Description, "Public URL")
	}
}

func TestParse_MissingName(t *testing.T) {
	input := []byte(`
expose:
  port: 8080
`)
	_, err := Parse(input)
	if err == nil {
		t.Fatal("expected error for missing name, got nil")
	}
}

func TestParse_MissingPort(t *testing.T) {
	input := []byte(`
name: myapp
`)
	_, err := Parse(input)
	if err == nil {
		t.Fatal("expected error for missing port, got nil")
	}
}

func TestParse_Defaults(t *testing.T) {
	input := []byte(`
name: myapp
expose:
  port: 3000
`)
	m, err := Parse(input)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if m.Runtime.Type != "docker-compose" {
		t.Errorf("Runtime.Type = %q, want default %q", m.Runtime.Type, "docker-compose")
	}
	if m.Runtime.Entry != "docker-compose.yml" {
		t.Errorf("Runtime.Entry = %q, want default %q", m.Runtime.Entry, "docker-compose.yml")
	}
}

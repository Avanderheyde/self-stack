package container

import (
	"errors"
	"testing"
)

func TestNewManager(t *testing.T) {
	m := NewManager()
	if m.timeout == 0 {
		t.Fatal("expected non-zero timeout")
	}
}

func TestComposeCommandPartsPrefersStandaloneCompose(t *testing.T) {
	old := execLookPath
	defer func() { execLookPath = old }()
	execLookPath = func(name string) (string, error) {
		if name == "docker-compose" {
			return "/usr/local/bin/docker-compose", nil
		}
		return "", errors.New("not found")
	}

	name, prefix := composeCommandParts()
	if name != "docker-compose" {
		t.Fatalf("name = %q, want docker-compose", name)
	}
	if len(prefix) != 0 {
		t.Fatalf("prefix = %v, want empty", prefix)
	}
}

func TestComposeCommandPartsFallsBackToDockerPlugin(t *testing.T) {
	old := execLookPath
	defer func() { execLookPath = old }()
	execLookPath = func(name string) (string, error) {
		return "", errors.New("not found")
	}

	name, prefix := composeCommandParts()
	if name != "docker" {
		t.Fatalf("name = %q, want docker", name)
	}
	if len(prefix) != 1 || prefix[0] != "compose" {
		t.Fatalf("prefix = %v, want [compose]", prefix)
	}
}

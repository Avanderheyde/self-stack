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

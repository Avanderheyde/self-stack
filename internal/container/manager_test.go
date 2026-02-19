package container

import "testing"

func TestNewManager(t *testing.T) {
	m := NewManager()
	if m.timeout == 0 {
		t.Fatal("expected non-zero timeout")
	}
}

package tunnel

import (
	"context"
	"testing"
)

func TestIsInstalled(t *testing.T) {
	_ = IsInstalled()
}

func TestNewCloudflare(t *testing.T) {
	cf := NewCloudflare("test-token")
	if cf.IsRunning() {
		t.Fatal("should not be running initially")
	}
}

func TestStartWithEmptyToken(t *testing.T) {
	cf := NewCloudflare("")
	err := cf.Start(context.Background())
	if err == nil {
		t.Fatal("expected error for empty token")
	}
}

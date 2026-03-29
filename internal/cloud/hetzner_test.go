package cloud

import (
	"testing"
)

func TestSetupScript(t *testing.T) {
	script := SetupScript("ts-auth-key-123")
	if script == "" {
		t.Fatal("empty setup script")
	}
	// Must install Docker
	if !contains(script, "docker") {
		t.Error("script missing docker install")
	}
	// Must install Tailscale
	if !contains(script, "tailscale") {
		t.Error("script missing tailscale install")
	}
	// Must install selfstack
	if !contains(script, "selfstack") {
		t.Error("script missing selfstack install")
	}
	// Must set up systemd service
	if !contains(script, "systemctl") {
		t.Error("script missing systemd setup")
	}
	// Must use the Tailscale auth key
	if !contains(script, "ts-auth-key-123") {
		t.Error("script missing tailscale auth key")
	}
}

func TestSetupScript_NoTailscale(t *testing.T) {
	script := SetupScript("")
	// Should still work without Tailscale
	if !contains(script, "docker") {
		t.Error("script missing docker install")
	}
	// Should skip Tailscale auth
	if contains(script, "tailscale up --authkey") {
		t.Error("script should not auth tailscale without key")
	}
}

func TestDefaultServerConfig(t *testing.T) {
	cfg := DefaultServerConfig()
	if cfg.ServerType == "" {
		t.Error("empty server type")
	}
	if cfg.Image == "" {
		t.Error("empty image")
	}
	if cfg.Location == "" {
		t.Error("empty location")
	}
}

func contains(s, substr string) bool {
	return len(s) >= len(substr) && containsSub(s, substr)
}

func containsSub(s, sub string) bool {
	for i := 0; i <= len(s)-len(sub); i++ {
		if s[i:i+len(sub)] == sub {
			return true
		}
	}
	return false
}

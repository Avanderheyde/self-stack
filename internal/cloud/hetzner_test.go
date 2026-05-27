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

func TestSetupScript_UsesRawGitHubInstallURL(t *testing.T) {
	// The default install URL must point at a real, reachable source.
	// selfstack.dev is not a domain we control yet — a hardcoded reference
	// to it would silently 404 inside cloud-init.
	t.Setenv("SELFSTACK_INSTALL_URL", "") // ensure no override leaks in
	script := SetupScript("")
	if contains(script, "selfstack.dev") {
		t.Error("setup script still references selfstack.dev — should use raw GitHub URL")
	}
	if !contains(script, "raw.githubusercontent.com/Avanderheyde/self-stack") {
		t.Error("setup script missing raw GitHub install URL")
	}
}

func TestSetupScript_InstallURLOverride(t *testing.T) {
	t.Setenv("SELFSTACK_INSTALL_URL", "https://example.test/install.sh")
	script := SetupScript("")
	if !contains(script, "https://example.test/install.sh") {
		t.Error("setup script ignored SELFSTACK_INSTALL_URL override")
	}
	if contains(script, "raw.githubusercontent.com") {
		t.Error("setup script used default URL when override was set")
	}
}

func TestSetupScriptWithInstallURLFlagOverride(t *testing.T) {
	t.Setenv("SELFSTACK_INSTALL_URL", "https://env.example/install.sh")
	script := SetupScriptWithInstallURL("", "https://flag.example/install.sh")
	if !contains(script, "https://flag.example/install.sh") {
		t.Error("setup script ignored explicit install URL")
	}
	if contains(script, "https://env.example/install.sh") {
		t.Error("setup script used env URL when explicit install URL was set")
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

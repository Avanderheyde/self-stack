package tailscale

import (
	"testing"
)

func TestParseStatus_Running(t *testing.T) {
	raw := `{"Self":{"HostName":"mybox","DNSName":"mybox.tail1234.ts.net.","Online":true,"TailscaleIPs":["100.64.0.1"]}}`
	s, err := parseStatus([]byte(raw))
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if !s.Running {
		t.Error("expected Running=true")
	}
	if s.Hostname != "mybox" {
		t.Errorf("Hostname = %q, want %q", s.Hostname, "mybox")
	}
	if s.DNSName != "mybox.tail1234.ts.net." {
		t.Errorf("DNSName = %q, want %q", s.DNSName, "mybox.tail1234.ts.net.")
	}
	if s.IP != "100.64.0.1" {
		t.Errorf("IP = %q, want %q", s.IP, "100.64.0.1")
	}
}

func TestParseStatus_Empty(t *testing.T) {
	_, err := parseStatus([]byte(""))
	if err == nil {
		t.Error("expected error for empty input")
	}
}

func TestServeURL(t *testing.T) {
	tests := []struct {
		dnsName string
		port    int
		want    string
	}{
		{"mybox.tail1234.ts.net.", 10001, "https://mybox.tail1234.ts.net:10001"},
		{"server.example.ts.net.", 3000, "https://server.example.ts.net:3000"},
	}
	for _, tt := range tests {
		got := serveURL(tt.dnsName, tt.port)
		if got != tt.want {
			t.Errorf("serveURL(%q, %d) = %q, want %q", tt.dnsName, tt.port, got, tt.want)
		}
	}
}

func TestServeURL_TrailingDot(t *testing.T) {
	// DNS names from tailscale status have trailing dot
	got := serveURL("mybox.tail1234.ts.net.", 8080)
	if got != "https://mybox.tail1234.ts.net:8080" {
		t.Errorf("got %q, trailing dot should be stripped", got)
	}
}

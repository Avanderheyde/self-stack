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

func TestServeArgs_UsesPerPortHTTPS(t *testing.T) {
	// The shorthand `tailscale serve <port>` only configures the default
	// 443 listener so a second app would collide. We must use the
	// --https=<port> form to give each app its own listener.
	got := serveArgs(10005)
	want := []string{"serve", "--bg", "--https=10005", "http://localhost:10005"}
	if len(got) != len(want) {
		t.Fatalf("serveArgs len = %d, want %d (%v)", len(got), len(want), got)
	}
	for i := range got {
		if got[i] != want[i] {
			t.Errorf("serveArgs[%d] = %q, want %q", i, got[i], want[i])
		}
	}
}

func TestResetArgs_TargetsSamePort(t *testing.T) {
	got := resetArgs(10005)
	want := []string{"serve", "--https=10005", "off"}
	if len(got) != len(want) {
		t.Fatalf("resetArgs len = %d, want %d (%v)", len(got), len(want), got)
	}
	for i := range got {
		if got[i] != want[i] {
			t.Errorf("resetArgs[%d] = %q, want %q", i, got[i], want[i])
		}
	}
}

func TestParseServedPorts(t *testing.T) {
	tests := []struct {
		name string
		in   string
		want map[int]bool
	}{
		{
			name: "two ports",
			in:   `{"TCP":{"10001":{"HTTPS":true},"10002":{"HTTPS":true}}}`,
			want: map[int]bool{10001: true, 10002: true},
		},
		{
			name: "empty",
			in:   `{"TCP":{}}`,
			want: map[int]bool{},
		},
		{
			name: "missing TCP",
			in:   `{}`,
			want: map[int]bool{},
		},
		{
			name: "garbage",
			in:   `not json`,
			want: nil,
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := parseServedPorts([]byte(tt.in))
			if tt.want == nil {
				if got != nil {
					t.Errorf("got %v, want nil", got)
				}
				return
			}
			gotSet := map[int]bool{}
			for _, p := range got {
				gotSet[p] = true
			}
			if len(gotSet) != len(tt.want) {
				t.Errorf("got %d ports, want %d", len(gotSet), len(tt.want))
			}
			for p := range tt.want {
				if !gotSet[p] {
					t.Errorf("missing port %d", p)
				}
			}
		})
	}
}

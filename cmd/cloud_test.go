package cmd

import "testing"

func TestParseServerID(t *testing.T) {
	tests := []struct {
		name    string
		input   string
		want    int64
		wantErr bool
	}{
		{"simple", "12345", 12345, false},
		{"trims whitespace", "  9876\n", 9876, false},
		{"zero is valid", "0", 0, false},
		{"empty string", "", 0, true},
		{"non-numeric", "abc", 0, true},
		{"trailing junk", "123abc", 0, true},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got, err := parseServerID(tt.input)
			if (err != nil) != tt.wantErr {
				t.Fatalf("parseServerID(%q) err = %v, wantErr = %v", tt.input, err, tt.wantErr)
			}
			if got != tt.want {
				t.Errorf("parseServerID(%q) = %d, want %d", tt.input, got, tt.want)
			}
		})
	}
}

func TestShortErr(t *testing.T) {
	tests := []struct {
		in   string
		want string
	}{
		{`Get "http://x:8080/api/status": dial tcp: connection refused`, "connection refused"},
		{"connection refused", "connection refused"},
		{"", ""},
	}
	for _, tt := range tests {
		got := shortErr(stringErr(tt.in))
		if got != tt.want {
			t.Errorf("shortErr(%q) = %q, want %q", tt.in, got, tt.want)
		}
	}
}

type stringErr string

func (s stringErr) Error() string { return string(s) }

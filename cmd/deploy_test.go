package cmd

import (
	"os"
	"path/filepath"
	"testing"
)

func TestParseEnvFile(t *testing.T) {
	tests := []struct {
		name     string
		content  string
		expected map[string]string
	}{
		{
			name:     "standard key=value",
			content:  "FOO=bar\nBAZ=qux",
			expected: map[string]string{"FOO": "bar", "BAZ": "qux"},
		},
		{
			name:     "comments ignored",
			content:  "# comment\nFOO=bar\n# another\nBAZ=qux",
			expected: map[string]string{"FOO": "bar", "BAZ": "qux"},
		},
		{
			name:     "double-quoted values",
			content:  `FOO="hello world"`,
			expected: map[string]string{"FOO": "hello world"},
		},
		{
			name:     "single-quoted values",
			content:  `FOO='hello world'`,
			expected: map[string]string{"FOO": "hello world"},
		},
		{
			name:     "values with equals signs",
			content:  "DATABASE_URL=postgres://user:pass@host/db?sslmode=require",
			expected: map[string]string{"DATABASE_URL": "postgres://user:pass@host/db?sslmode=require"},
		},
		{
			name:     "empty lines skipped",
			content:  "FOO=bar\n\n\nBAZ=qux\n",
			expected: map[string]string{"FOO": "bar", "BAZ": "qux"},
		},
		{
			name:     "empty file",
			content:  "",
			expected: map[string]string{},
		},
		{
			name:     "whitespace around key and value",
			content:  "  FOO  =  bar  ",
			expected: map[string]string{"FOO": "bar"},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			dir := t.TempDir()
			path := filepath.Join(dir, ".env")
			if err := os.WriteFile(path, []byte(tt.content), 0644); err != nil {
				t.Fatal(err)
			}
			got, err := parseEnvFile(path)
			if err != nil {
				t.Fatalf("unexpected error: %v", err)
			}
			if len(got) != len(tt.expected) {
				t.Fatalf("got %d vars, want %d: %v", len(got), len(tt.expected), got)
			}
			for k, v := range tt.expected {
				if got[k] != v {
					t.Errorf("got[%q] = %q, want %q", k, got[k], v)
				}
			}
		})
	}
}

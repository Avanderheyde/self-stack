package detect

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func TestDetectProjectType(t *testing.T) {
	tests := []struct {
		name     string
		files    map[string]string // filename -> content
		expected ProjectType
	}{
		{
			name:     "docker-compose.yml present",
			files:    map[string]string{"docker-compose.yml": "version: '3'"},
			expected: TypeDockerCompose,
		},
		{
			name:     "Dockerfile present",
			files:    map[string]string{"Dockerfile": "FROM node:22"},
			expected: TypeDockerfile,
		},
		{
			name:     "Dockerfile wins over package.json",
			files:    map[string]string{"Dockerfile": "FROM node:22", "package.json": `{"name":"app"}`},
			expected: TypeDockerfile,
		},
		{
			name:     "docker-compose.yml wins over Dockerfile",
			files:    map[string]string{"docker-compose.yml": "version: '3'", "Dockerfile": "FROM node:22"},
			expected: TypeDockerCompose,
		},
		{
			name:     "package.json detected as Node",
			files:    map[string]string{"package.json": `{"name":"app","scripts":{"start":"node index.js"}}`},
			expected: TypeNode,
		},
		{
			name:     "go.mod detected as Go",
			files:    map[string]string{"go.mod": "module example.com/app"},
			expected: TypeGo,
		},
		{
			name:     "requirements.txt detected as Python",
			files:    map[string]string{"requirements.txt": "flask"},
			expected: TypePython,
		},
		{
			name:     "pyproject.toml detected as Python",
			files:    map[string]string{"pyproject.toml": "[project]\nname = 'app'"},
			expected: TypePython,
		},
		{
			name:     "empty directory returns unknown",
			files:    map[string]string{},
			expected: TypeUnknown,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			dir := t.TempDir()
			for name, content := range tt.files {
				if err := os.WriteFile(filepath.Join(dir, name), []byte(content), 0644); err != nil {
					t.Fatal(err)
				}
			}
			got := DetectProjectType(dir)
			if got != tt.expected {
				t.Errorf("DetectProjectType() = %v, want %v", got, tt.expected)
			}
		})
	}
}

func TestGenerateDockerfile(t *testing.T) {
	tests := []struct {
		name        string
		projectType ProjectType
		files       map[string]string
		wantErr     bool
		contains    []string // strings the Dockerfile should contain
	}{
		{
			name:        "Node.js with start script",
			projectType: TypeNode,
			files:       map[string]string{"package.json": `{"name":"app","scripts":{"start":"node index.js"}}`},
			contains:    []string{"FROM node:22-slim", "npm install", `"npm", "start"`},
		},
		{
			name:        "Node.js with next dependency",
			projectType: TypeNode,
			files:       map[string]string{"package.json": `{"name":"app","dependencies":{"next":"14.0.0"},"scripts":{"build":"next build","start":"next start"}}`},
			contains:    []string{"FROM node:22-slim", "npm run build", `"npm", "start"`},
		},
		{
			name:        "Go app",
			projectType: TypeGo,
			files:       map[string]string{"go.mod": "module example.com/app\n\ngo 1.22", "main.go": "package main"},
			contains:    []string{"FROM golang:", "go build", "/app"},
		},
		{
			name:        "Python with requirements.txt",
			projectType: TypePython,
			files:       map[string]string{"requirements.txt": "flask", "app.py": "from flask import Flask"},
			contains:    []string{"FROM python:3.12-slim", "pip install", "requirements.txt"},
		},
		{
			name:        "Unknown type returns error",
			projectType: TypeUnknown,
			wantErr:     true,
		},
		{
			name:        "DockerCompose returns error (no generation needed)",
			projectType: TypeDockerCompose,
			wantErr:     true,
		},
		{
			name:        "Dockerfile type returns error (already exists)",
			projectType: TypeDockerfile,
			wantErr:     true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			dir := t.TempDir()
			for name, content := range tt.files {
				if err := os.WriteFile(filepath.Join(dir, name), []byte(content), 0644); err != nil {
					t.Fatal(err)
				}
			}
			content, err := GenerateDockerfile(dir, tt.projectType)
			if tt.wantErr {
				if err == nil {
					t.Error("expected error, got nil")
				}
				return
			}
			if err != nil {
				t.Fatalf("unexpected error: %v", err)
			}
			for _, s := range tt.contains {
				if !containsString(content, s) {
					t.Errorf("Dockerfile missing %q:\n%s", s, content)
				}
			}
		})
	}
}

func TestDetectEntrypoint(t *testing.T) {
	tests := []struct {
		name     string
		files    map[string]string
		expected string
	}{
		{
			name:     "Python main.py exists",
			files:    map[string]string{"main.py": "print('hello')"},
			expected: "main.py",
		},
		{
			name:     "Python app.py exists",
			files:    map[string]string{"app.py": "from flask import Flask"},
			expected: "app.py",
		},
		{
			name:     "Python manage.py (Django)",
			files:    map[string]string{"manage.py": "#!/usr/bin/env python"},
			expected: "manage.py",
		},
		{
			name:     "server.py detected",
			files:    map[string]string{"server.py": "import http.server"},
			expected: "server.py",
		},
		{
			name:     "No known entrypoint falls back to main.py",
			files:    map[string]string{"utils.py": "def helper(): pass"},
			expected: "main.py",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			dir := t.TempDir()
			for name, content := range tt.files {
				os.WriteFile(filepath.Join(dir, name), []byte(content), 0644)
			}
			got := detectPythonEntrypoint(dir)
			if got != tt.expected {
				t.Errorf("detectPythonEntrypoint() = %q, want %q", got, tt.expected)
			}
		})
	}
}

func TestValidateAppName(t *testing.T) {
	valid := []string{"my-app", "app1", "budget-tracker", "a", "abc123"}
	invalid := []string{"../etc", "my app", "MY_APP", "", "-start", "end-", "a/b",
		"this-name-is-way-too-long-and-exceeds-the-sixty-three-character-dns-label-limit-by-far"}

	for _, name := range valid {
		t.Run("valid/"+name, func(t *testing.T) {
			if err := ValidateAppName(name); err != nil {
				t.Errorf("ValidateAppName(%q) = %v, want nil", name, err)
			}
		})
	}
	for _, name := range invalid {
		t.Run("invalid/"+name, func(t *testing.T) {
			if err := ValidateAppName(name); err == nil {
				t.Errorf("ValidateAppName(%q) = nil, want error", name)
			}
		})
	}
}

func TestSlugifyDirName(t *testing.T) {
	tests := []struct {
		input    string
		expected string
	}{
		{"My Cool App", "my-cool-app"},
		{"budget_tracker", "budget-tracker"},
		{"App v2.0", "app-v2-0"},
		{"  spaces  ", "spaces"},
		{"UPPER", "upper"},
	}
	for _, tt := range tests {
		t.Run(tt.input, func(t *testing.T) {
			got := SlugifyDirName(tt.input)
			if got != tt.expected {
				t.Errorf("SlugifyDirName(%q) = %q, want %q", tt.input, got, tt.expected)
			}
		})
	}
}

func containsString(haystack, needle string) bool {
	return strings.Contains(haystack, needle)
}

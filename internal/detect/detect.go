package detect

import (
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"regexp"
	"strings"
)

type ProjectType int

const (
	TypeUnknown ProjectType = iota
	TypeDockerCompose
	TypeDockerfile
	TypeNode
	TypeGo
	TypePython
)

func (t ProjectType) String() string {
	switch t {
	case TypeDockerCompose:
		return "docker-compose"
	case TypeDockerfile:
		return "dockerfile"
	case TypeNode:
		return "node"
	case TypeGo:
		return "go"
	case TypePython:
		return "python"
	default:
		return "unknown"
	}
}

// DetectProjectType checks for project markers in priority order.
// docker-compose.yml > Dockerfile > package.json > go.mod > requirements.txt/pyproject.toml
func DetectProjectType(dir string) ProjectType {
	if fileExists(dir, "docker-compose.yml") || fileExists(dir, "docker-compose.yaml") {
		return TypeDockerCompose
	}
	if fileExists(dir, "Dockerfile") {
		return TypeDockerfile
	}
	if fileExists(dir, "package.json") {
		return TypeNode
	}
	if fileExists(dir, "go.mod") {
		return TypeGo
	}
	if fileExists(dir, "requirements.txt") || fileExists(dir, "pyproject.toml") {
		return TypePython
	}
	return TypeUnknown
}

// GenerateDockerfile produces Dockerfile content for the given project type.
// Returns an error for types that don't need generation (DockerCompose, Dockerfile, Unknown).
func GenerateDockerfile(dir string, pt ProjectType) (string, error) {
	switch pt {
	case TypeNode:
		return generateNodeDockerfile(dir)
	case TypeGo:
		return generateGoDockerfile(dir)
	case TypePython:
		return generatePythonDockerfile(dir)
	case TypeUnknown:
		return "", fmt.Errorf("cannot generate Dockerfile: unknown project type. Add a Dockerfile or docker-compose.yml")
	case TypeDockerCompose:
		return "", fmt.Errorf("docker-compose.yml already exists, no Dockerfile generation needed")
	case TypeDockerfile:
		return "", fmt.Errorf("Dockerfile already exists, no generation needed")
	default:
		return "", fmt.Errorf("unsupported project type: %v", pt)
	}
}

func generateNodeDockerfile(dir string) (string, error) {
	pkg := readPackageJSON(dir)
	hasNext := pkg.hasDep("next")
	hasBuild := pkg.hasScript("build")

	var b strings.Builder
	b.WriteString("FROM node:22-slim\nWORKDIR /app\n")
	b.WriteString("COPY package*.json ./\n")
	b.WriteString("RUN npm install --production\n")
	b.WriteString("COPY . .\n")

	if hasBuild {
		b.WriteString("RUN npm run build\n")
	}

	if hasNext {
		b.WriteString("EXPOSE 3000\n")
		b.WriteString("CMD [\"npm\", \"start\"]\n")
	} else if pkg.hasScript("start") {
		b.WriteString("EXPOSE 3000\n")
		b.WriteString("CMD [\"npm\", \"start\"]\n")
	} else {
		b.WriteString("EXPOSE 3000\n")
		b.WriteString("CMD [\"node\", \"index.js\"]\n")
	}
	return b.String(), nil
}

func generateGoDockerfile(dir string) (string, error) {
	// Use golang image (not scratch) to handle CGO deps like modernc.org/sqlite
	var b strings.Builder
	b.WriteString("FROM golang:1.22 AS builder\nWORKDIR /src\n")
	b.WriteString("COPY go.mod go.sum* ./\n")
	b.WriteString("RUN go mod download\n")
	b.WriteString("COPY . .\n")
	b.WriteString("RUN CGO_ENABLED=0 go build -o /app .\n\n")
	b.WriteString("FROM debian:bookworm-slim\n")
	b.WriteString("COPY --from=builder /app /app\n")
	b.WriteString("EXPOSE 8080\n")
	b.WriteString("CMD [\"/app\"]\n")
	return b.String(), nil
}

func generatePythonDockerfile(dir string) (string, error) {
	entrypoint := detectPythonEntrypoint(dir)

	var b strings.Builder
	b.WriteString("FROM python:3.12-slim\nWORKDIR /app\n")

	if fileExists(dir, "requirements.txt") {
		b.WriteString("COPY requirements.txt .\n")
		b.WriteString("RUN pip install --no-cache-dir -r requirements.txt\n")
	} else if fileExists(dir, "pyproject.toml") {
		b.WriteString("COPY pyproject.toml .\n")
		b.WriteString("RUN pip install --no-cache-dir .\n")
	}

	b.WriteString("COPY . .\n")
	b.WriteString("EXPOSE 8000\n")

	// Detect framework for proper start command
	if fileExists(dir, "manage.py") {
		b.WriteString("CMD [\"python\", \"manage.py\", \"runserver\", \"0.0.0.0:8000\"]\n")
	} else if containsInFile(dir, "requirements.txt", "fastapi") || containsInFile(dir, "requirements.txt", "uvicorn") {
		b.WriteString(fmt.Sprintf("CMD [\"uvicorn\", \"%s:app\", \"--host\", \"0.0.0.0\", \"--port\", \"8000\"]\n",
			strings.TrimSuffix(entrypoint, ".py")))
	} else if containsInFile(dir, "requirements.txt", "flask") {
		b.WriteString(fmt.Sprintf("CMD [\"python\", \"%s\"]\n", entrypoint))
	} else {
		b.WriteString(fmt.Sprintf("CMD [\"python\", \"%s\"]\n", entrypoint))
	}
	return b.String(), nil
}

// detectPythonEntrypoint checks for common Python entry points in priority order.
func detectPythonEntrypoint(dir string) string {
	candidates := []string{"main.py", "app.py", "manage.py", "run.py", "server.py"}
	for _, c := range candidates {
		if fileExists(dir, c) {
			return c
		}
	}
	return "main.py"
}

// ValidateAppName checks that an app name is DNS-label compatible.
var appNamePattern = regexp.MustCompile(`^[a-z0-9]([a-z0-9-]*[a-z0-9])?$`)

func ValidateAppName(name string) error {
	if name == "" {
		return fmt.Errorf("app name cannot be empty")
	}
	if len(name) > 63 {
		return fmt.Errorf("app name %q exceeds 63 characters", name)
	}
	if !appNamePattern.MatchString(name) {
		return fmt.Errorf("app name %q must contain only lowercase letters, numbers, and hyphens, must start and end with a letter or number", name)
	}
	return nil
}

// SlugifyDirName converts a directory name to a valid app name.
func SlugifyDirName(name string) string {
	name = strings.ToLower(strings.TrimSpace(name))
	// Replace underscores and spaces with hyphens
	name = strings.ReplaceAll(name, "_", "-")
	name = strings.ReplaceAll(name, " ", "-")
	// Remove anything that's not alphanumeric or hyphen
	var b strings.Builder
	for _, r := range name {
		if (r >= 'a' && r <= 'z') || (r >= '0' && r <= '9') || r == '-' {
			b.WriteRune(r)
		} else {
			b.WriteRune('-')
		}
	}
	result := b.String()
	// Collapse multiple hyphens
	for strings.Contains(result, "--") {
		result = strings.ReplaceAll(result, "--", "-")
	}
	result = strings.Trim(result, "-")
	if len(result) > 63 {
		result = result[:63]
		result = strings.TrimRight(result, "-")
	}
	return result
}

// helpers

func fileExists(dir, name string) bool {
	_, err := os.Stat(filepath.Join(dir, name))
	return err == nil
}

func containsInFile(dir, filename, search string) bool {
	data, err := os.ReadFile(filepath.Join(dir, filename))
	if err != nil {
		return false
	}
	return strings.Contains(strings.ToLower(string(data)), strings.ToLower(search))
}

type packageJSON struct {
	Scripts      map[string]string `json:"scripts"`
	Dependencies map[string]string `json:"dependencies"`
}

func readPackageJSON(dir string) packageJSON {
	data, err := os.ReadFile(filepath.Join(dir, "package.json"))
	if err != nil {
		return packageJSON{}
	}
	var pkg packageJSON
	json.Unmarshal(data, &pkg)
	return pkg
}

func (p packageJSON) hasScript(name string) bool {
	_, ok := p.Scripts[name]
	return ok
}

func (p packageJSON) hasDep(name string) bool {
	_, ok := p.Dependencies[name]
	return ok
}

package detect

import (
	"fmt"
	"strings"
)

// DefaultPort returns the conventional port for a project type.
func DefaultPort(pt ProjectType) int {
	switch pt {
	case TypeNode:
		return 3000
	case TypeGo:
		return 8080
	case TypePython:
		return 8000
	default:
		return 3000
	}
}

// GenerateComposeFile produces a docker-compose.yml that wraps a Dockerfile.
func GenerateComposeFile(appName string, containerPort int) string {
	var b strings.Builder
	fmt.Fprintf(&b, "version: '3.8'\nservices:\n  %s:\n", appName)
	b.WriteString("    build: .\n")
	fmt.Fprintf(&b, "    ports:\n      - \"${SELFSTACK_HOST_PORT}:%d\"\n", containerPort)
	b.WriteString("    restart: unless-stopped\n")
	return b.String()
}

// GenerateManifestYAML produces a selfstack.yml manifest for a deployed app.
func GenerateManifestYAML(appName string, containerPort int) string {
	var b strings.Builder
	fmt.Fprintf(&b, "name: %s\n", appName)
	fmt.Fprintf(&b, "display_name: %s\n", appName)
	b.WriteString("description: Deployed via selfstack deploy\n")
	b.WriteString("version: 0.0.1\n\n")
	b.WriteString("runtime:\n")
	b.WriteString("  type: docker-compose\n")
	b.WriteString("  entry: docker-compose.yml\n\n")
	fmt.Fprintf(&b, "expose:\n  port: %d\n  health: /\n", containerPort)
	return b.String()
}

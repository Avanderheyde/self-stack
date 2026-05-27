package manifest

import (
	"fmt"
	"os"

	"gopkg.in/yaml.v3"
)

type Manifest struct {
	Name        string      `yaml:"name"`
	DisplayName string      `yaml:"display_name"`
	Description string      `yaml:"description"`
	Version     string      `yaml:"version"`
	Icon        string      `yaml:"icon"`
	Runtime     Runtime     `yaml:"runtime"`
	Expose      Expose      `yaml:"expose"`
	Volumes     []string    `yaml:"volumes"`
	Config      []ConfigVar `yaml:"config"`
}

type Runtime struct {
	Type  string `yaml:"type"`
	Entry string `yaml:"entry"`
}

type Expose struct {
	Port   int    `yaml:"port"`
	Health string `yaml:"health"`
}

type ConfigVar struct {
	Key         string `yaml:"key"`
	Default     string `yaml:"default"`
	Description string `yaml:"description"`
}

func ParseFile(path string) (*Manifest, error) {
	data, err := os.ReadFile(path)
	if err != nil {
		return nil, fmt.Errorf("read manifest: %w", err)
	}
	return Parse(data)
}

func Parse(data []byte) (*Manifest, error) {
	var m Manifest
	if err := yaml.Unmarshal(data, &m); err != nil {
		return nil, fmt.Errorf("parse manifest: %w", err)
	}
	if m.Name == "" {
		return nil, fmt.Errorf("manifest missing required field: name")
	}
	if m.Expose.Port == 0 {
		return nil, fmt.Errorf("manifest missing required field: expose.port")
	}
	if m.Runtime.Type == "" {
		m.Runtime.Type = "docker-compose"
	}
	if m.Runtime.Entry == "" {
		m.Runtime.Entry = "docker-compose.yml"
	}
	return &m, nil
}

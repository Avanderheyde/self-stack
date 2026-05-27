package config

import (
	"fmt"
	"os"
	"path/filepath"
	"strings"

	"gopkg.in/yaml.v3"
)

// RemoteConfig holds the VPS connection settings for selfstack deploy.
type RemoteConfig struct {
	Host     string `yaml:"host"`      // user@host SSH target
	APIPort  int    `yaml:"api_port"`  // port where selfstack serve runs on the VPS
	ServerID string `yaml:"server_id"` // Hetzner server ID (for cloud destroy)
}

func (r RemoteConfig) IsConfigured() bool {
	return r.Host != ""
}

// SSHTarget returns the full user@host string for SSH commands.
func (r RemoteConfig) SSHTarget() string {
	return r.Host
}

// Hostname extracts just the hostname/IP from the user@host string.
func (r RemoteConfig) Hostname() string {
	if idx := strings.LastIndex(r.Host, "@"); idx >= 0 {
		return r.Host[idx+1:]
	}
	return r.Host
}

// APIURL returns the HTTP URL to reach the selfstack API on the VPS.
func (r RemoteConfig) APIURL() string {
	port := r.APIPort
	if port == 0 {
		port = DefaultPort
	}
	return fmt.Sprintf("http://%s:%d", r.Hostname(), port)
}

func remoteConfigPath() string {
	return filepath.Join(HomeDir(), "remote.yml")
}

// LoadRemoteConfig reads the remote configuration. Returns a zero-value
// config (not an error) if the file doesn't exist.
func LoadRemoteConfig() (RemoteConfig, error) {
	var rc RemoteConfig
	data, err := os.ReadFile(remoteConfigPath())
	if os.IsNotExist(err) {
		return rc, nil
	}
	if err != nil {
		return rc, fmt.Errorf("read remote config: %w", err)
	}
	if err := yaml.Unmarshal(data, &rc); err != nil {
		return rc, fmt.Errorf("parse remote config: %w", err)
	}
	return rc, nil
}

// SaveRemoteConfig writes the remote configuration to disk.
func SaveRemoteConfig(rc RemoteConfig) error {
	if err := os.MkdirAll(HomeDir(), 0755); err != nil {
		return fmt.Errorf("create config dir: %w", err)
	}
	data, err := yaml.Marshal(rc)
	if err != nil {
		return fmt.Errorf("marshal remote config: %w", err)
	}
	return os.WriteFile(remoteConfigPath(), data, 0600)
}

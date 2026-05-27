package cloud

import (
	"fmt"
	"os"
	"path/filepath"
	"strings"

	"github.com/selfstack/selfstack/internal/config"
)

func secretsDir() string {
	return filepath.Join(config.HomeDir(), "secrets")
}

func writeSecretFile(name, value string) error {
	dir := secretsDir()
	if err := os.MkdirAll(dir, 0700); err != nil {
		return fmt.Errorf("create secrets dir: %w", err)
	}
	return os.WriteFile(filepath.Join(dir, name), []byte(value), 0600)
}

func readSecretFile(name string) (string, error) {
	data, err := os.ReadFile(filepath.Join(secretsDir(), name))
	if err != nil {
		return "", err
	}
	return strings.TrimSpace(string(data)), nil
}

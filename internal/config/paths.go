package config

import (
	"os"
	"path/filepath"
)

const (
	DefaultPort    = 8080
	PortRangeStart = 10001
	AppDir         = "apps"
	ExportsDir     = "exports"
	KeysDir        = "keys"
	DBFile         = "selfstack.db"
)

func HomeDir() string {
	if env := os.Getenv("SELFSTACK_HOME"); env != "" {
		return env
	}
	home, _ := os.UserHomeDir()
	return filepath.Join(home, ".selfstack")
}

func AppsDir() string  { return filepath.Join(HomeDir(), AppDir) }
func DataDir() string  { return filepath.Join(HomeDir(), ExportsDir) }
func KeysPath() string { return filepath.Join(HomeDir(), KeysDir) }
func DBPath() string   { return filepath.Join(HomeDir(), DBFile) }

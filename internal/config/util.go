package config

import (
	"os"
	"path/filepath"
)

// GetDir returns the directory where the config file should be located
func GetDir() (string, error) {
	execPath, err := os.Executable()
	if err != nil {
		return "", err
	}
	return filepath.Dir(execPath), nil
}

// GetConfigPath returns the full path to the config file
func GetConfigPath() (string, error) {
	dir, err := GetDir()
	if err != nil {
		return "", err
	}
	return filepath.Join(dir, "config.json"), nil
}

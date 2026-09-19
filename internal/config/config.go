// Package config manages saving and loading device credentials to disk.
package config

import (
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"

	"github.com/juange87/kindlecli/internal/amazon"
)

const authFile = "auth.json"

// DefaultDir returns the default configuration directory (~/.config/kindlecli).
func DefaultDir() string {
	home, err := os.UserHomeDir()
	if err != nil {
		return ""
	}
	return filepath.Join(home, ".config", "kindlecli")
}

// Save writes the DeviceInfo to auth.json inside dir with 0600 permissions.
func Save(dir string, info amazon.DeviceInfo) error {
	return SaveJSON(dir, authFile, info)
}

// Load reads the DeviceInfo from auth.json inside dir.
func Load(dir string) (amazon.DeviceInfo, error) {
	path := filepath.Join(dir, authFile)
	data, err := os.ReadFile(path)
	if err != nil {
		return amazon.DeviceInfo{}, fmt.Errorf("reading config: %w", err)
	}
	var info amazon.DeviceInfo
	if err := json.Unmarshal(data, &info); err != nil {
		return amazon.DeviceInfo{}, fmt.Errorf("parsing config: %w", err)
	}
	return info, nil
}

// Delete removes the auth.json file from dir.
func Delete(dir string) error {
	path := filepath.Join(dir, authFile)
	return os.Remove(path)
}

// SaveJSON atomically replaces a private JSON file in dir. A failed write leaves
// the previous file intact. Rename replaces symlinks instead of following them.
func SaveJSON(dir, name string, value any) error {
	if filepath.Base(name) != name || name == "." || name == ".." {
		return fmt.Errorf("invalid config filename")
	}
	data, err := json.MarshalIndent(value, "", "  ")
	if err != nil {
		return fmt.Errorf("marshaling config: %w", err)
	}
	if err := os.MkdirAll(dir, 0700); err != nil {
		return fmt.Errorf("creating config dir: %w", err)
	}
	f, err := os.CreateTemp(dir, ".kindlecli-*")
	if err != nil {
		return err
	}
	defer os.Remove(f.Name())
	defer f.Close()
	if _, err := f.Write(data); err != nil {
		return err
	}
	if err := f.Sync(); err != nil {
		return err
	}
	if err := f.Close(); err != nil {
		return err
	}
	if err := os.Rename(f.Name(), filepath.Join(dir, name)); err != nil {
		return fmt.Errorf("replacing config: %w", err)
	}
	return nil
}

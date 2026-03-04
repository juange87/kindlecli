// internal/config/config_test.go
package config

import (
	"os"
	"path/filepath"
	"testing"

	"github.com/juange/kindlecli/internal/amazon"
)

func TestSaveAndLoad(t *testing.T) {
	dir := t.TempDir()

	info := amazon.DeviceInfo{
		DevicePrivateKey: "-----BEGIN RSA PRIVATE KEY-----\ntest\n-----END RSA PRIVATE KEY-----",
		ADPToken:         "adp-token-123",
		GivenName:        "Juan",
		DeviceType:       "A1K6D1WRW0MALS",
		Name:             "Juan Garcia",
		AccountPool:      "Amazon",
		UserDirectedID:   "uid",
		UserDeviceName:   "Kindlecli",
	}

	err := Save(dir, info)
	if err != nil {
		t.Fatalf("Save failed: %v", err)
	}

	// Verify file permissions
	stat, _ := os.Stat(filepath.Join(dir, "auth.json"))
	if stat.Mode().Perm() != 0600 {
		t.Errorf("file permissions = %o, want 0600", stat.Mode().Perm())
	}

	loaded, err := Load(dir)
	if err != nil {
		t.Fatalf("Load failed: %v", err)
	}

	if loaded.ADPToken != info.ADPToken {
		t.Errorf("ADPToken = %q, want %q", loaded.ADPToken, info.ADPToken)
	}
	if loaded.GivenName != info.GivenName {
		t.Errorf("GivenName = %q, want %q", loaded.GivenName, info.GivenName)
	}
}

func TestLoadNotFound(t *testing.T) {
	dir := t.TempDir()
	_, err := Load(dir)
	if err == nil {
		t.Error("expected error for missing auth.json")
	}
}

func TestDelete(t *testing.T) {
	dir := t.TempDir()

	info := amazon.DeviceInfo{ADPToken: "test"}
	_ = Save(dir, info)
	err := Delete(dir)
	if err != nil {
		t.Fatalf("Delete failed: %v", err)
	}

	_, err = Load(dir)
	if err == nil {
		t.Error("expected error after delete")
	}
}

func TestDefaultDir(t *testing.T) {
	dir := DefaultDir()
	if dir == "" {
		t.Error("DefaultDir returned empty string")
	}
}

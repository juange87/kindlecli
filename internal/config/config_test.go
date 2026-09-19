// internal/config/config_test.go
package config

import (
	"os"
	"path/filepath"
	"runtime"
	"testing"

	"github.com/juange87/kindlecli/internal/amazon"
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
	if runtime.GOOS != "windows" && stat.Mode().Perm() != 0600 {
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

func TestSaveReplacesPermissionsAndPreservesOnFailure(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, authFile)
	if err := os.WriteFile(path, []byte("old"), 0644); err != nil {
		t.Fatal(err)
	}
	if err := Save(dir, amazon.DeviceInfo{ADPToken: "new"}); err != nil {
		t.Fatal(err)
	}
	stat, err := os.Stat(path)
	if err != nil {
		t.Fatal(err)
	}
	if runtime.GOOS != "windows" && stat.Mode().Perm() != 0600 {
		t.Fatalf("mode: %o", stat.Mode().Perm())
	}
	before, _ := os.ReadFile(path)
	if err := SaveJSON(dir, authFile, make(chan int)); err == nil {
		t.Fatal("expected marshal failure")
	}
	after, _ := os.ReadFile(path)
	if string(before) != string(after) {
		t.Fatal("previous session damaged")
	}
	files, _ := os.ReadDir(dir)
	if len(files) != 1 {
		t.Fatal("temporary files left behind")
	}
}

func TestSaveDoesNotFollowSymlink(t *testing.T) {
	if runtime.GOOS == "windows" {
		t.Skip("symlink creation requires privileges")
	}
	dir := t.TempDir()
	target := filepath.Join(t.TempDir(), "target")
	if err := os.WriteFile(target, []byte("untouched"), 0600); err != nil {
		t.Fatal(err)
	}
	if err := os.Symlink(target, filepath.Join(dir, authFile)); err != nil {
		t.Fatal(err)
	}
	if err := Save(dir, amazon.DeviceInfo{ADPToken: "new"}); err != nil {
		t.Fatal(err)
	}
	data, _ := os.ReadFile(target)
	if string(data) != "untouched" {
		t.Fatal("followed symlink")
	}
}

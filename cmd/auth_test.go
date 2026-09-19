package cmd

import (
	"crypto/rand"
	"crypto/rsa"
	"crypto/x509"
	"encoding/pem"
	"errors"
	"os"
	"path/filepath"
	"testing"

	"github.com/juange87/kindlecli/internal/amazon"
	"github.com/juange87/kindlecli/internal/config"
)

func testCredentials(t *testing.T) amazon.DeviceInfo {
	t.Helper()
	key, err := rsa.GenerateKey(rand.Reader, 2048)
	if err != nil {
		t.Fatal(err)
	}
	return amazon.DeviceInfo{ADPToken: "test", DevicePrivateKey: string(pem.EncodeToMemory(&pem.Block{Type: "RSA PRIVATE KEY", Bytes: x509.MarshalPKCS1PrivateKey(key)}))}
}

func TestInspectSession(t *testing.T) {
	dir := t.TempDir()
	probe := func(amazon.DeviceInfo) error { t.Fatal("unexpected request"); return nil }
	if got := inspectSession(dir, probe); got.State != "missing" {
		t.Fatal(got)
	}
	if err := os.WriteFile(filepath.Join(dir, "auth.json"), []byte("{"), 0600); err != nil {
		t.Fatal(err)
	}
	if got := inspectSession(dir, probe); got.State != "unavailable" {
		t.Fatal(got)
	}
	if err := config.Save(dir, amazon.DeviceInfo{}); err != nil {
		t.Fatal(err)
	}
	if got := inspectSession(dir, probe); got.State != "invalid" {
		t.Fatal(got)
	}
	if err := config.Save(dir, testCredentials(t)); err != nil {
		t.Fatal(err)
	}
	for _, tt := range []struct {
		err   error
		state string
	}{{nil, "valid"}, {amazon.ErrSessionRejected, "rejected"}, {errors.New("network"), "unavailable"}} {
		before, _ := os.ReadFile(filepath.Join(dir, "auth.json"))
		got := inspectSession(dir, func(amazon.DeviceInfo) error { return tt.err })
		if got.State != tt.state {
			t.Fatal(got)
		}
		after, _ := os.ReadFile(filepath.Join(dir, "auth.json"))
		if string(before) != string(after) {
			t.Fatal("credentials changed")
		}
	}
}

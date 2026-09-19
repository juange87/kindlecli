package cmd

import (
	"bytes"
	"encoding/json"
	"os"
	"path/filepath"
	"testing"
	"time"

	"github.com/juange87/kindlecli/internal/amazon"
	"github.com/juange87/kindlecli/internal/config"
	"github.com/spf13/cobra"
)

func TestPendingLoginExpiry(t *testing.T) {
	dir := t.TempDir()
	now := time.Now().UTC()
	for _, age := range []time.Duration{0, loginLifetime, loginLifetime + time.Second, -time.Second} {
		attempt := pendingLogin{OAuth: *amazon.NewOAuth2(), CreatedAt: now.Add(-age)}
		if err := config.SaveJSON(dir, pendingLoginFile, attempt); err != nil {
			t.Fatal(err)
		}
		_, err := readPendingLogin(dir, now)
		if (err == nil) != (age == 0) {
			t.Fatalf("age %v: %v", age, err)
		}
	}
}

func TestLoginLock(t *testing.T) {
	dir := t.TempDir()
	unlock, err := lockLogin(dir)
	if err != nil {
		t.Fatal(err)
	}
	if _, err := lockLogin(dir); err == nil {
		t.Fatal("concurrent login allowed")
	}
	unlock()
	unlock, err = lockLogin(dir)
	if err != nil {
		t.Fatal(err)
	}
	unlock()
}

func TestStartAndBadFinishPreserveSavedSession(t *testing.T) {
	dir := t.TempDir()
	old := cfgDir
	cfgDir = dir
	t.Cleanup(func() { cfgDir = old })
	if err := config.Save(dir, amazon.DeviceInfo{ADPToken: "old"}); err != nil {
		t.Fatal(err)
	}
	before, _ := os.ReadFile(filepath.Join(dir, "auth.json"))
	cmd := &cobra.Command{}
	for _, name := range []string{"start", "force", "json"} {
		cmd.Flags().Bool(name, true, "")
	}
	var out bytes.Buffer
	cmd.SetOut(&out)
	if err := runLogin(cmd, nil); err != nil {
		t.Fatal(err)
	}
	var result struct{ State, URL string }
	if err := json.Unmarshal(out.Bytes(), &result); err != nil {
		t.Fatal(err)
	}
	if result.State != "pending" || result.URL == "" {
		t.Fatal(result)
	}
	cmd = &cobra.Command{}
	cmd.SetIn(bytes.NewBufferString("https://evil.example/?secret=code\n"))
	if err := finishLogin(cmd, dir, false, false); err == nil {
		t.Fatal("bad redirect accepted")
	}
	after, _ := os.ReadFile(filepath.Join(dir, "auth.json"))
	if !bytes.Equal(before, after) {
		t.Fatal("old session overwritten")
	}
	if _, err := os.Stat(filepath.Join(dir, pendingLoginFile)); err != nil {
		t.Fatal("pending login removed on validation error")
	}
}

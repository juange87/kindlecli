package cmd

import (
	"io"
	"os"
	"path/filepath"
	"testing"

	"github.com/spf13/cobra"
)

func TestLogoutCleansCorruptAndPendingCredentials(t *testing.T) {
	dir := t.TempDir()
	old := cfgDir
	cfgDir = dir
	t.Cleanup(func() { cfgDir = old })
	for _, name := range []string{"auth.json", pendingLoginFile} {
		if err := os.WriteFile(filepath.Join(dir, name), []byte("bad json"), 0600); err != nil {
			t.Fatal(err)
		}
	}
	cmd := &cobra.Command{}
	cmd.SetOut(io.Discard)
	cmd.SetErr(io.Discard)
	if err := runLogout(cmd, nil); err != nil {
		t.Fatal(err)
	}
	entries, err := os.ReadDir(dir)
	if err != nil || len(entries) != 0 {
		t.Fatal(entries, err)
	}
}

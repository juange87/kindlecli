package cmd

import (
	"bytes"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"strings"
	"testing"

	"github.com/juange87/kindlecli/internal/amazon"
	"github.com/juange87/kindlecli/internal/config"
	"github.com/spf13/cobra"
)

func TestInspectSessionFlagsSharedSerial(t *testing.T) {
	dir := t.TempDir()
	legacy := testCredentials(t)
	if err := config.Save(dir, legacy); err != nil {
		t.Fatal(err)
	}
	got := inspectSession(dir, func(amazon.DeviceInfo) error { return nil })
	if got.State != "valid" || !got.LegacySerial || got.ExitCode != exitOK || !strings.Contains(got.Message, "kindlecli login") {
		t.Fatal(got)
	}
	got = inspectSession(dir, func(amazon.DeviceInfo) error { return amazon.ErrSessionRejected })
	if got.State != "rejected" || !got.LegacySerial || got.ExitCode != exitLoginRequired || !strings.Contains(got.Message, "shared") {
		t.Fatal(got)
	}

	private := testCredentials(t)
	private.DeviceSerialNumber = amazon.NewDeviceSerial()
	if err := config.Save(dir, private); err != nil {
		t.Fatal(err)
	}
	got = inspectSession(dir, func(amazon.DeviceInfo) error { return nil })
	if got.State != "valid" || got.LegacySerial || got.ExitCode != exitOK {
		t.Fatal(got)
	}
	got = inspectSession(dir, func(amazon.DeviceInfo) error { return amazon.ErrSessionRejected })
	if got.State != "rejected" || got.LegacySerial || strings.Contains(got.Message, "shared") {
		t.Fatal(got)
	}
}

func TestStatusExitCodes(t *testing.T) {
	for state, want := range map[string]int{
		"valid": exitOK, "missing": exitLoginRequired, "invalid": exitLoginRequired,
		"rejected": exitLoginRequired, "unavailable": exitUnavailable, "unknown": exitFailure,
	} {
		if got := statusExitCode(state); got != want {
			t.Errorf("%s: got %d want %d", state, got, want)
		}
	}
	// A corrupt config is a local problem: retrying later will not fix it.
	dir := t.TempDir()
	if err := config.SaveJSON(dir, "auth.json", "not an object"); err != nil {
		t.Fatal(err)
	}
	if got := inspectSession(dir, nil); got.ExitCode != exitLoginRequired {
		t.Fatal(got)
	}
}

func TestErrorExitCodes(t *testing.T) {
	legacy := amazon.DeviceInfo{}
	for _, tt := range []struct {
		name string
		err  error
		want int
	}{
		{"success", nil, exitOK},
		{"generic", errors.New("boom"), exitFailure},
		{"rejected by devices", explainRejection(legacy, fmt.Errorf("getting devices: %w", amazon.ErrSessionRejected)), exitLoginRequired},
		{"rejected during send", fmt.Errorf("0 accepted, 1 failed: %w", amazon.ErrSessionRejected), exitLoginRequired},
		{"missing session", loginRequired(errors.New("no saved session")), exitLoginRequired},
		{"network", fmt.Errorf("getting devices: %w", &amazon.NetworkError{Operation: "x", Err: errors.New("down")}), exitUnavailable},
		{"explicit", &codedError{exitUnavailable, errors.New("later")}, exitUnavailable},
	} {
		if got := exitCode(tt.err); got != tt.want {
			t.Errorf("%s: got %d want %d", tt.name, got, tt.want)
		}
	}
	err := explainRejection(legacy, amazon.ErrSessionRejected)
	if !errors.Is(err, amazon.ErrSessionRejected) || !strings.Contains(err.Error(), "kindlecli login") {
		t.Fatal(err)
	}
	private := amazon.DeviceInfo{DeviceSerialNumber: amazon.NewDeviceSerial()}
	if got := explainRejection(private, amazon.ErrSessionRejected); got != amazon.ErrSessionRejected {
		t.Fatal("private session rejection should not blame the shared serial")
	}
}

func TestSendRejectionRequiresLogin(t *testing.T) {
	cmd := &cobra.Command{}
	cmd.SetOut(io.Discard)
	cmd.SetErr(io.Discard)
	sender := &fakeSender{results: []error{amazon.ErrSessionRejected}}
	err := sendDocuments(cmd, sender, []document{{path: "a.epub"}}, 0)
	if exitCode(explainRejection(amazon.DeviceInfo{}, err)) != exitLoginRequired {
		t.Fatal(err)
	}
}

func TestDeviceSerialForReusesOnlyPrivateSerials(t *testing.T) {
	dir := t.TempDir()
	first := deviceSerialFor(dir)
	if !amazon.IsPrivateDeviceSerial(first) {
		t.Fatalf("missing session got %q", first)
	}
	private := amazon.NewDeviceSerial()
	for _, tt := range []struct {
		stored string
		reuse  bool
	}{{private, true}, {amazon.LegacyDeviceSerial, false}, {"", false}, {"malformed", false}} {
		if err := config.Save(dir, amazon.DeviceInfo{ADPToken: "t", DeviceSerialNumber: tt.stored}); err != nil {
			t.Fatal(err)
		}
		got := deviceSerialFor(dir)
		if !amazon.IsPrivateDeviceSerial(got) || (got == tt.stored) != tt.reuse {
			t.Fatalf("stored %q: got %q", tt.stored, got)
		}
	}
}

func runLoginStart(t *testing.T, dir string) string {
	t.Helper()
	old := cfgDir
	cfgDir = dir
	t.Cleanup(func() { cfgDir = old })
	cmd := &cobra.Command{}
	for _, name := range []string{"force", "fresh", "start", "finish", "json", "no-browser", "clipboard"} {
		cmd.Flags().Bool(name, name == "start" || name == "json", "")
	}
	var out bytes.Buffer
	cmd.SetOut(&out)
	cmd.SetErr(io.Discard)
	if err := runLogin(cmd, nil); err != nil {
		t.Fatal(err)
	}
	var result struct{ State string }
	if err := json.Unmarshal(out.Bytes(), &result); err != nil {
		t.Fatal(err)
	}
	return result.State
}

func TestLoginMigratesValidLegacySession(t *testing.T) {
	old := probeSession
	probeSession = func(amazon.DeviceInfo) error { return nil }
	t.Cleanup(func() { probeSession = old })

	dir := t.TempDir()
	if err := config.Save(dir, testCredentials(t)); err != nil {
		t.Fatal(err)
	}
	if state := runLoginStart(t, dir); state != "pending" {
		t.Fatalf("legacy session reused: %s", state)
	}

	private := testCredentials(t)
	private.DeviceSerialNumber = amazon.NewDeviceSerial()
	if err := config.Save(dir, private); err != nil {
		t.Fatal(err)
	}
	if state := runLoginStart(t, dir); state != "valid" {
		t.Fatalf("private session not reused: %s", state)
	}
}

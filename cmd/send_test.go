package cmd

import (
	"errors"
	"io"
	"os"
	"path/filepath"
	"testing"

	"github.com/juange87/kindlecli/internal/amazon"
	"github.com/spf13/cobra"
)

type fakeSender struct {
	results []error
	calls   int
}

func (f *fakeSender) GetOwnedDevices() ([]amazon.OwnedDevice, error) {
	return []amazon.OwnedDevice{{DeviceSerialNumber: "test"}}, nil
}
func (f *fakeSender) SendFile(string, []string, string, string) (string, error) {
	err := f.results[f.calls]
	f.calls++
	return "sku", err
}

func TestBatchFailuresReturnError(t *testing.T) {
	for _, tt := range []struct {
		name    string
		results []error
		initial int
		wantErr bool
		calls   int
	}{
		{"all accepted", []error{nil, nil}, 0, false, 2},
		{"partial failure", []error{errors.New("upload"), nil}, 0, true, 2},
		{"all failed", []error{errors.New("upload"), errors.New("upload")}, 0, true, 2},
		{"invalid input", []error{nil, nil}, 1, true, 2},
		{"session rejected", []error{amazon.ErrSessionRejected, nil}, 0, true, 1},
	} {
		t.Run(tt.name, func(t *testing.T) {
			cmd := &cobra.Command{}
			cmd.SetOut(io.Discard)
			cmd.SetErr(io.Discard)
			sender := &fakeSender{results: tt.results}
			err := sendDocuments(cmd, sender, []document{{path: "a.epub"}, {path: "b.epub"}}, tt.initial)
			if (err != nil) != tt.wantErr || sender.calls != tt.calls {
				t.Fatalf("err=%v calls=%d", err, sender.calls)
			}
		})
	}
}

func TestPrepareRejectsInvalidInputs(t *testing.T) {
	dir := t.TempDir()
	good := filepath.Join(dir, "book.EPUB")
	empty := filepath.Join(dir, "empty.pdf")
	folder := filepath.Join(dir, "folder.epub")
	if err := os.WriteFile(good, []byte("book"), 0600); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(empty, nil, 0600); err != nil {
		t.Fatal(err)
	}
	if err := os.Mkdir(folder, 0700); err != nil {
		t.Fatal(err)
	}
	docs, failures := prepareDocuments([]string{good, empty, folder, filepath.Join(dir, "missing.epub"), "book.exe"}, "", "")
	if len(docs) != 1 || len(failures) != 4 {
		t.Fatal(docs, failures)
	}
	if docs[0].title != "book" || docs[0].author != "Unknown" {
		t.Fatal(docs[0])
	}
}

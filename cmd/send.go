// cmd/send.go
package cmd

import (
	"errors"
	"fmt"
	"github.com/juange87/kindlecli/internal/amazon"
	"os"
	"path/filepath"
	"strings"
	"time"

	"github.com/spf13/cobra"
)

var sendTitle string
var sendAuthor string

var sendCmd = &cobra.Command{
	Use:   "send <file> [file...]",
	Short: "Send files to your Kindle",
	Long:  "Upload EPUB or PDF files and send them to all your Kindle devices.",
	Args:  cobra.MinimumNArgs(1),
	RunE:  runSend,
}

func init() {
	sendCmd.Flags().StringVar(&sendTitle, "title", "", "document title (default: filename)")
	sendCmd.Flags().StringVar(&sendAuthor, "author", "", "document author")
	sendCmd.Flags().Duration("upload-timeout", 10*time.Minute, "maximum time per file upload (e.g. 20m)")
	rootCmd.AddCommand(sendCmd)
}

var supportedFormats = map[string]bool{
	".epub": true,
	".pdf":  true,
	".mobi": true,
	".azw":  true,
	".azw3": true,
	".doc":  true,
	".docx": true,
	".txt":  true,
	".rtf":  true,
	".htm":  true,
	".html": true,
}

type document struct {
	path, title, author string
	size                int64
}

type documentSender interface {
	GetOwnedDevices() ([]amazon.OwnedDevice, error)
	SendFile(string, []string, string, string) (string, error)
}

func prepareDocuments(paths []string, title, author string) ([]document, []error) {
	var documents []document
	var failures []error
	if author == "" {
		author = "Unknown"
	}
	for _, path := range paths {
		ext := strings.ToLower(filepath.Ext(path))
		if !supportedFormats[ext] {
			failures = append(failures, fmt.Errorf("%s: unsupported format %q", path, ext))
			continue
		}
		stat, err := os.Stat(path)
		if err != nil {
			failures = append(failures, fmt.Errorf("%s: %w", path, err))
			continue
		}
		if !stat.Mode().IsRegular() || stat.Size() == 0 {
			failures = append(failures, fmt.Errorf("%s: expected a nonempty regular file", path))
			continue
		}
		name := title
		if name == "" {
			name = strings.TrimSuffix(filepath.Base(path), filepath.Ext(path))
		}
		documents = append(documents, document{path, name, author, stat.Size()})
	}
	return documents, failures
}

func runSend(cmd *cobra.Command, args []string) error {
	timeout, _ := cmd.Flags().GetDuration("upload-timeout")
	if timeout <= 0 {
		return fmt.Errorf("--upload-timeout must be greater than zero")
	}
	documents, failures := prepareDocuments(args, sendTitle, sendAuthor)
	for _, err := range failures {
		fmt.Fprintln(cmd.ErrOrStderr(), "Skipping:", err)
	}
	if len(documents) == 0 {
		return fmt.Errorf("no files sent: %d invalid input(s)", len(failures))
	}
	client, err := loadClient()
	if err != nil {
		return err
	}
	client.UploadTimeout = timeout
	return sendDocuments(cmd, client, documents, len(failures))
}

func sendDocuments(cmd *cobra.Command, client documentSender, documents []document, failed int) error {
	devices, err := client.GetOwnedDevices()
	if err != nil {
		return fmt.Errorf("getting devices: %w", err)
	}
	if len(devices) == 0 {
		return fmt.Errorf("no Kindle devices found on your account")
	}
	serials := make([]string, len(devices))
	for i, d := range devices {
		serials[i] = d.DeviceSerialNumber
	}
	sent := 0
	for i, doc := range documents {
		fmt.Fprintf(cmd.OutOrStdout(), "Sending %s (%.1f MB)...\n", filepath.Base(doc.path), float64(doc.size)/1024/1024)
		_, err := client.SendFile(doc.path, serials, doc.title, doc.author)
		if err != nil {
			failed++
			fmt.Fprintf(cmd.ErrOrStderr(), "Failed to send %s: %v\n", doc.path, err)
			if errors.Is(err, amazon.ErrSessionRejected) {
				return fmt.Errorf("%d accepted, %d failed, %d unattempted: %w", sent, failed, len(documents)-i-1, err)
			}
			continue
		}
		sent++
		fmt.Fprintf(cmd.OutOrStdout(), "Accepted by Amazon for %d device(s).\n", len(devices))
	}
	fmt.Fprintf(cmd.OutOrStdout(), "Result: %d accepted, %d failed.\n", sent, failed)
	if failed > 0 {
		return fmt.Errorf("%d file(s) failed; retry only those files to avoid duplicates", failed)
	}
	return nil
}

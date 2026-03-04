// cmd/send.go
package cmd

import (
	"fmt"
	"os"
	"path/filepath"
	"strings"

	"github.com/juange/kindlecli/internal/amazon"
	"github.com/juange/kindlecli/internal/config"
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

func runSend(cmd *cobra.Command, args []string) error {
	dir := cfgDir
	if dir == "" {
		dir = config.DefaultDir()
	}

	deviceInfo, err := config.Load(dir)
	if err != nil {
		return fmt.Errorf("not logged in. Run 'kindlecli login' first")
	}

	client, err := amazon.NewClient(deviceInfo)
	if err != nil {
		return fmt.Errorf("creating client: %w", err)
	}

	// Get all devices
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

	// Send each file
	for _, filePath := range args {
		ext := strings.ToLower(filepath.Ext(filePath))
		if !supportedFormats[ext] {
			fmt.Fprintf(os.Stderr, "Skipping %s: unsupported format %q\n", filePath, ext)
			continue
		}

		stat, err := os.Stat(filePath)
		if err != nil {
			fmt.Fprintf(os.Stderr, "Skipping %s: %v\n", filePath, err)
			continue
		}

		title := sendTitle
		if title == "" {
			title = strings.TrimSuffix(filepath.Base(filePath), filepath.Ext(filePath))
		}

		sizeMB := float64(stat.Size()) / 1024 / 1024
		fmt.Printf("Sending %s (%.1f MB)...\n", filepath.Base(filePath), sizeMB)

		_, err = client.SendFile(filePath, serials, title, sendAuthor)
		if err != nil {
			fmt.Fprintf(os.Stderr, "Failed to send %s: %v\n", filePath, err)
			continue
		}
		fmt.Printf("Sent to %d device(s).\n", len(devices))
	}

	return nil
}

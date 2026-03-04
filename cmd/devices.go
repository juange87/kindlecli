// cmd/devices.go
package cmd

import (
	"fmt"

	"github.com/juange/kindlecli/internal/amazon"
	"github.com/juange/kindlecli/internal/config"
	"github.com/spf13/cobra"
)

var devicesCmd = &cobra.Command{
	Use:   "devices",
	Short: "List your Kindle devices",
	RunE:  runDevices,
}

func init() {
	rootCmd.AddCommand(devicesCmd)
}

func runDevices(cmd *cobra.Command, args []string) error {
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

	devices, err := client.GetOwnedDevices()
	if err != nil {
		return fmt.Errorf("getting devices: %w", err)
	}

	if len(devices) == 0 {
		fmt.Println("No Kindle devices found.")
		return nil
	}

	fmt.Printf("Found %d device(s):\n", len(devices))
	for i, d := range devices {
		fmt.Printf("  %d. %s (%s)\n", i+1, d.DeviceName, d.DeviceSerialNumber)
	}
	return nil
}

// cmd/logout.go
package cmd

import (
	"fmt"

	"github.com/juange/kindlecli/internal/amazon"
	"github.com/juange/kindlecli/internal/config"
	"github.com/spf13/cobra"
)

var logoutCmd = &cobra.Command{
	Use:   "logout",
	Short: "Log out and delete saved session",
	RunE:  runLogout,
}

func init() {
	rootCmd.AddCommand(logoutCmd)
}

func runLogout(cmd *cobra.Command, args []string) error {
	dir := cfgDir
	if dir == "" {
		dir = config.DefaultDir()
	}

	deviceInfo, err := config.Load(dir)
	if err != nil {
		fmt.Println("Not logged in.")
		return nil
	}

	// Try to logout remotely (best effort)
	client, err := amazon.NewClient(deviceInfo)
	if err == nil {
		if err := client.Logout(); err != nil && verbose {
			fmt.Printf("Remote logout warning: %v\n", err)
		}
	}

	if err := config.Delete(dir); err != nil {
		return fmt.Errorf("deleting session: %w", err)
	}

	fmt.Println("Logged out successfully.")
	return nil
}

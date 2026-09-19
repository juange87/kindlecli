package cmd

import (
	"errors"
	"fmt"
	"os"
	"path/filepath"

	"github.com/juange87/kindlecli/internal/amazon"
	"github.com/juange87/kindlecli/internal/config"
	"github.com/spf13/cobra"
)

var logoutCmd = &cobra.Command{Use: "logout", Short: "Deregister the device and delete local credentials", Args: cobra.NoArgs, RunE: runLogout}

func init() { rootCmd.AddCommand(logoutCmd) }

func runLogout(cmd *cobra.Command, args []string) error {
	dir := configDir()
	unlock, err := lockLogin(dir)
	if err != nil {
		return err
	}
	defer unlock()
	info, err := config.Load(dir)
	if err == nil {
		client, clientErr := amazon.NewClient(info)
		if clientErr == nil {
			clientErr = client.Logout()
		}
		if clientErr != nil {
			fmt.Fprintf(cmd.ErrOrStderr(), "Remote deregistration could not be confirmed: %v\n", clientErr)
		}
	} else if !errors.Is(err, os.ErrNotExist) {
		fmt.Fprintln(cmd.ErrOrStderr(), "Saved credentials could not be read; remote deregistration was not attempted.")
	}
	if err := config.Delete(dir); err != nil && !errors.Is(err, os.ErrNotExist) {
		return fmt.Errorf("deleting session: %w", err)
	}
	if err := os.Remove(filepath.Join(dir, pendingLoginFile)); err != nil && !errors.Is(err, os.ErrNotExist) {
		return fmt.Errorf("deleting pending login: %w", err)
	}
	fmt.Fprintln(cmd.OutOrStdout(), "Local session and pending login deleted.")
	return nil
}

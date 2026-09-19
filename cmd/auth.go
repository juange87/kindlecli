package cmd

import (
	"encoding/json"
	"errors"
	"fmt"
	"os"

	"github.com/juange87/kindlecli/internal/amazon"
	"github.com/juange87/kindlecli/internal/config"
	"github.com/spf13/cobra"
)

type sessionStatus struct {
	State   string `json:"state"`
	Message string `json:"message"`
}

func configDir() string {
	if cfgDir != "" {
		return cfgDir
	}
	return config.DefaultDir()
}

func probeSession(info amazon.DeviceInfo) error {
	client, err := amazon.NewClient(info)
	if err != nil {
		return err
	}
	_, err = client.GetOwnedDevices()
	return err
}

func inspectSession(dir string, probe func(amazon.DeviceInfo) error) sessionStatus {
	info, err := config.Load(dir)
	if errors.Is(err, os.ErrNotExist) {
		return sessionStatus{"missing", "No saved session. Run 'kindlecli login'."}
	}
	if err != nil {
		return sessionStatus{"unavailable", "Cannot read saved session. Check config permissions and JSON, or use 'kindlecli login --force'."}
	}
	if _, err := amazon.NewClient(info); err != nil {
		return sessionStatus{"invalid", "Saved credentials are incomplete or invalid. Run 'kindlecli login'."}
	}
	if err := probe(info); err != nil {
		if errors.Is(err, amazon.ErrSessionRejected) {
			return sessionStatus{"rejected", amazon.ErrSessionRejected.Error()}
		}
		return sessionStatus{"unavailable", "Could not verify session (network or service error). Credentials were preserved; try again later."}
	}
	return sessionStatus{"valid", "Amazon accepts the saved session."}
}

func loadClient() (*amazon.Client, error) {
	info, err := config.Load(configDir())
	if errors.Is(err, os.ErrNotExist) {
		return nil, fmt.Errorf("no saved session; run 'kindlecli login'")
	}
	if err != nil {
		return nil, fmt.Errorf("loading session: %w", err)
	}
	client, err := amazon.NewClient(info)
	if err != nil {
		return nil, fmt.Errorf("invalid saved credentials; run 'kindlecli login': %w", err)
	}
	return client, nil
}

func init() {
	authCmd := &cobra.Command{Use: "auth", Short: "Inspect authentication"}
	statusCmd := &cobra.Command{Use: "status", Short: "Check the saved session with Amazon", Args: cobra.NoArgs,
		RunE: func(cmd *cobra.Command, args []string) error {
			status := inspectSession(configDir(), probeSession)
			asJSON, _ := cmd.Flags().GetBool("json")
			if asJSON {
				if err := json.NewEncoder(cmd.OutOrStdout()).Encode(status); err != nil {
					return err
				}
			} else {
				fmt.Fprintln(cmd.OutOrStdout(), status.Message)
			}
			if status.State != "valid" {
				return fmt.Errorf("session status: %s", status.State)
			}
			return nil
		},
	}
	statusCmd.Flags().Bool("json", false, "output machine-readable session status")
	authCmd.AddCommand(statusCmd)
	rootCmd.AddCommand(authCmd)
}

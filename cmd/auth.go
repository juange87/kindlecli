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
	// LegacySerial means the session was registered with the shared legacy
	// device serial, which any other stkclient-based login can revoke.
	LegacySerial bool `json:"legacy_serial,omitempty"`
	ExitCode     int  `json:"exit_code"`
}

const legacySerialHint = "This session uses the device serial shared by older kindlecli versions and other " +
	"Send-to-Kindle clients; any login elsewhere with that serial revokes it. Run 'kindlecli login' once " +
	"with this version to register a private device that stays valid."

func newStatus(state, message string) sessionStatus {
	return sessionStatus{State: state, Message: message, ExitCode: statusExitCode(state)}
}

// explainRejection adds the cause and the fix to a session rejection from a
// session registered with the shared legacy serial.
func explainRejection(info amazon.DeviceInfo, err error) error {
	if errors.Is(err, amazon.ErrSessionRejected) && info.UsesSharedSerial() {
		return fmt.Errorf("%w\n%s", err, legacySerialHint)
	}
	return err
}

func configDir() string {
	if cfgDir != "" {
		return cfgDir
	}
	return config.DefaultDir()
}

// probeSession asks Amazon whether it accepts info. A variable so tests can
// replace the network call.
var probeSession = func(info amazon.DeviceInfo) error {
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
		return newStatus("missing", "No saved session. Run 'kindlecli login'.")
	}
	if err != nil {
		// A local problem: retrying later will not fix it, so it requires login.
		return newStatus("invalid", "Cannot read saved session. Check config permissions and JSON, or replace it with 'kindlecli login --force'.")
	}
	if _, err := amazon.NewClient(info); err != nil {
		return newStatus("invalid", "Saved credentials are incomplete or invalid. Run 'kindlecli login'.")
	}
	legacy := info.UsesSharedSerial()
	var status sessionStatus
	if err := probe(info); err != nil {
		if errors.Is(err, amazon.ErrSessionRejected) {
			status = newStatus("rejected", explainRejection(info, amazon.ErrSessionRejected).Error())
		} else {
			status = newStatus("unavailable", "Could not verify session (network or service error). Credentials were preserved; try again later.")
		}
	} else if legacy {
		status = newStatus("valid", "Amazon accepts the saved session, but it can stop working at any time. "+legacySerialHint)
	} else {
		status = newStatus("valid", "Amazon accepts the saved session.")
	}
	status.LegacySerial = legacy
	return status
}

func loadClient() (*amazon.Client, amazon.DeviceInfo, error) {
	info, err := config.Load(configDir())
	if errors.Is(err, os.ErrNotExist) {
		return nil, info, loginRequired(fmt.Errorf("no saved session; run 'kindlecli login'"))
	}
	if err != nil {
		return nil, info, loginRequired(fmt.Errorf("loading session: %w", err))
	}
	client, err := amazon.NewClient(info)
	if err != nil {
		return nil, info, loginRequired(fmt.Errorf("invalid saved credentials; run 'kindlecli login': %w", err))
	}
	return client, info, nil
}

func init() {
	authCmd := &cobra.Command{Use: "auth", Short: "Inspect authentication"}
	statusCmd := &cobra.Command{Use: "status", Short: "Check the saved session with Amazon", Args: cobra.NoArgs,
		Long: "Ask Amazon whether the saved session is accepted. Never opens a browser or changes credentials.\n\n" +
			"Exit codes: 0 valid, 2 login required (missing, invalid or rejected), " +
			"3 Amazon could not be reached (try again later), 1 other errors.",
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
			if status.ExitCode != exitOK {
				return &codedError{status.ExitCode, fmt.Errorf("session status: %s", status.State)}
			}
			return nil
		},
	}
	statusCmd.Flags().Bool("json", false, "output machine-readable session status")
	authCmd.AddCommand(statusCmd)
	rootCmd.AddCommand(authCmd)
}

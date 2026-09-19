package cmd

import (
	"bufio"
	"encoding/json"
	"fmt"
	"io"
	"os"
	"os/exec"
	"path/filepath"
	"runtime"
	"strings"
	"time"

	"github.com/juange87/kindlecli/internal/amazon"
	"github.com/juange87/kindlecli/internal/config"
	"github.com/spf13/cobra"
)

const pendingLoginFile = "pending-login.json"
const loginLifetime = 10 * time.Minute

type pendingLogin struct {
	OAuth     amazon.OAuth2 `json:"oauth"`
	CreatedAt time.Time     `json:"created_at"`
}

var loginCmd = &cobra.Command{
	Use: "login", Short: "Log in to your Amazon account", Args: cobra.NoArgs,
	Long: "Reuse a valid session or authenticate through your browser. Use --start and --finish for automation.",
	RunE: runLogin,
}

func init() {
	f := loginCmd.Flags()
	f.Bool("force", false, "replace an existing session without checking it first")
	f.Bool("fresh", false, "ask Amazon for fresh browser authentication")
	f.Bool("start", false, "save a pending login and print its URL without waiting")
	f.Bool("finish", false, "complete the pending login using the redirect URL from stdin")
	f.Bool("json", false, "output machine-readable results")
	f.Bool("no-browser", false, "print the URL instead of opening a browser")
	f.Bool("clipboard", false, "read the redirect URL from the clipboard")
	loginCmd.MarkFlagsMutuallyExclusive("start", "finish")
	rootCmd.AddCommand(loginCmd)
}

// Each invocation holds the lock only until it exits. Two-step logins release it
// while the user is in the browser. An interrupted process may leave a stale lock.
func lockLogin(dir string) (func(), error) {
	if err := os.MkdirAll(dir, 0700); err != nil {
		return nil, err
	}
	path := filepath.Join(dir, ".login-lock")
	if err := os.Mkdir(path, 0700); err != nil {
		return nil, fmt.Errorf("cannot acquire login lock: %w; if no login is running, remove %s", err, path)
	}
	return func() { _ = os.Remove(path) }, nil
}

func runLogin(cmd *cobra.Command, args []string) error {
	dir := configDir()
	finish, _ := cmd.Flags().GetBool("finish")
	start, _ := cmd.Flags().GetBool("start")
	asJSON, _ := cmd.Flags().GetBool("json")
	clipboard, _ := cmd.Flags().GetBool("clipboard")
	force, _ := cmd.Flags().GetBool("force")
	fresh, _ := cmd.Flags().GetBool("fresh")
	noBrowser, _ := cmd.Flags().GetBool("no-browser")
	if finish && (force || fresh || noBrowser) {
		return fmt.Errorf("--finish cannot be combined with --force, --fresh or --no-browser")
	}
	if start && clipboard {
		return fmt.Errorf("--clipboard is only used when completing login")
	}
	if asJSON && !start && !finish {
		return fmt.Errorf("use --json with --start or --finish")
	}
	unlock, err := lockLogin(dir)
	if err != nil {
		return err
	}
	defer unlock()
	amazon.Verbose = verbose
	if finish {
		return finishLogin(cmd, dir, clipboard, asJSON)
	}
	if !force {
		status := inspectSession(dir, probeSession)
		if status.State == "valid" {
			return printLoginResult(cmd, asJSON, map[string]any{"state": "valid"}, status.Message)
		}
		if status.State == "unavailable" {
			return fmt.Errorf("%s", status.Message)
		}
	}
	attempt := pendingLogin{OAuth: *amazon.NewOAuth2(), CreatedAt: time.Now().UTC()}
	if err := config.SaveJSON(dir, pendingLoginFile, attempt); err != nil {
		return fmt.Errorf("saving pending login: %w", err)
	}
	signinURL := attempt.OAuth.SignInURL(fresh)
	if start {
		return printLoginResult(cmd, asJSON, map[string]any{"state": "pending", "url": signinURL, "expires_at": attempt.CreatedAt.Add(loginLifetime)}, signinURL)
	}
	fmt.Fprintln(cmd.OutOrStdout(), "Open this URL to authenticate with Amazon:")
	fmt.Fprintln(cmd.OutOrStdout(), signinURL)
	if !noBrowser {
		if err := openBrowser(signinURL); err != nil {
			fmt.Fprintln(cmd.ErrOrStderr(), "Could not open browser. Open the URL above manually.")
		}
	}
	if clipboard {
		fmt.Fprintln(cmd.OutOrStdout(), "Copy the FULL redirect URL, then press Enter to read the clipboard.")
		if _, err := bufio.NewReader(cmd.InOrStdin()).ReadString('\n'); err != nil {
			return fmt.Errorf("reading input: %w; complete later with 'kindlecli login --finish'", err)
		}
	} else {
		fmt.Fprintln(cmd.OutOrStdout(), "After signing in, paste the FULL Send to Kindle redirect URL and press Enter:")
	}
	return finishLogin(cmd, dir, clipboard, false)
}

func readPendingLogin(dir string, now time.Time) (pendingLogin, error) {
	var pending pendingLogin
	data, err := os.ReadFile(filepath.Join(dir, pendingLoginFile))
	if err != nil {
		return pending, fmt.Errorf("cannot read pending login; run 'kindlecli login --start': %w", err)
	}
	if err := json.Unmarshal(data, &pending); err != nil {
		return pending, fmt.Errorf("invalid pending login; run 'kindlecli login --start' again")
	}
	if pending.CreatedAt.IsZero() || now.Before(pending.CreatedAt) || now.Sub(pending.CreatedAt) >= loginLifetime {
		return pending, fmt.Errorf("pending login expired; run 'kindlecli login --start' again")
	}
	if len(pending.OAuth.Verifier) != 43 || strings.ContainsAny(pending.OAuth.Verifier, " \n\r\t") {
		return pending, fmt.Errorf("invalid pending verifier; restart login")
	}
	return pending, nil
}

func finishLogin(cmd *cobra.Command, dir string, clipboard, asJSON bool) error {
	attempt, err := readPendingLogin(dir, time.Now().UTC())
	if err != nil {
		return err
	}
	var redirectURL string
	if clipboard {
		redirectURL, err = readClipboard()
	} else {
		// Limit input; never pass authorization codes as command-line arguments.
		var data []byte
		data, err = bufio.NewReader(io.LimitReader(cmd.InOrStdin(), 64*1024+1)).ReadBytes('\n')
		if err == io.EOF {
			err = nil
		}
		if len(data) > 64*1024 {
			return fmt.Errorf("redirect URL too long")
		}
		redirectURL = string(data)
	}
	if err != nil {
		return fmt.Errorf("reading redirect URL: %w", err)
	}
	redirectURL = strings.TrimSpace(redirectURL)
	if redirectURL == "" {
		return fmt.Errorf("redirect URL is empty")
	}
	info, err := attempt.OAuth.CreateClient(redirectURL)
	if err != nil {
		return fmt.Errorf("authentication failed: %w; restart with 'kindlecli login --start' if the code has expired or was already used", err)
	}
	if err := config.Save(dir, info); err != nil {
		return fmt.Errorf("saving session: %w", err)
	}
	if err := os.Remove(filepath.Join(dir, pendingLoginFile)); err != nil {
		return fmt.Errorf("session saved, but could not remove pending login: %w", err)
	}
	return printLoginResult(cmd, asJSON, map[string]any{"state": "authenticated"}, "Authenticated. Session saved. Run 'kindlecli auth status' to verify it.")
}

func printLoginResult(cmd *cobra.Command, asJSON bool, value any, message string) error {
	if asJSON {
		return json.NewEncoder(cmd.OutOrStdout()).Encode(value)
	}
	_, err := fmt.Fprintln(cmd.OutOrStdout(), message)
	return err
}

func readClipboard() (string, error) {
	switch runtime.GOOS {
	case "darwin":
		out, err := exec.Command("pbpaste").Output()
		return string(out), err
	case "linux":
		if _, err := exec.LookPath("wl-paste"); err == nil {
			out, err := exec.Command("wl-paste", "--no-newline").Output()
			return string(out), err
		}
		out, err := exec.Command("xclip", "-selection", "clipboard", "-o").Output()
		return string(out), err
	case "windows":
		out, err := exec.Command("powershell", "-NoProfile", "-command", "Get-Clipboard").Output()
		return string(out), err
	default:
		return "", fmt.Errorf("unsupported platform for clipboard; paste the URL through stdin")
	}
}

func openBrowser(url string) error {
	var cmd *exec.Cmd
	switch runtime.GOOS {
	case "darwin":
		cmd = exec.Command("open", url)
	case "linux":
		cmd = exec.Command("xdg-open", url)
	case "windows":
		cmd = exec.Command("rundll32", "url.dll,FileProtocolHandler", url)
	default:
		return fmt.Errorf("unsupported platform")
	}
	return cmd.Run()
}

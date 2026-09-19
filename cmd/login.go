// cmd/login.go
package cmd

import (
	"fmt"
	"os/exec"
	"runtime"
	"strings"

	"github.com/juange87/kindlecli/internal/amazon"
	"github.com/juange87/kindlecli/internal/config"
	"github.com/spf13/cobra"
)

var loginCmd = &cobra.Command{
	Use:   "login",
	Short: "Log in to your Amazon account",
	Long:  "Opens your browser for Amazon OAuth2 login. After signing in, the URL is read from your clipboard.",
	RunE:  runLogin,
}

func init() {
	loginCmd.Flags().Bool("force", false, "authenticate even if a saved session exists")
	rootCmd.AddCommand(loginCmd)
}

func runLogin(cmd *cobra.Command, args []string) error {
	dir := cfgDir
	if dir == "" {
		dir = config.DefaultDir()
	}

	force, _ := cmd.Flags().GetBool("force")
	if !force {
		status := inspectSession(dir, probeSession)
		switch status.State {
		case "valid":
			fmt.Fprintln(cmd.OutOrStdout(), status.Message)
			return nil
		case "unavailable":
			return fmt.Errorf("%s", status.Message)
		}
	}

	amazon.Verbose = verbose
	oauth := amazon.NewOAuth2()
	signinURL := oauth.GetSignInURL()

	fmt.Println("Opening browser for Amazon login...")
	if err := openBrowser(signinURL); err != nil {
		fmt.Println("Could not open browser. Please open this URL manually:")
		fmt.Println(signinURL)
	}

	fmt.Println()
	fmt.Println("After signing in, you will be redirected to the Send to Kindle page.")
	fmt.Println("Copy the FULL URL from your browser's address bar (Cmd+L, Cmd+C).")
	fmt.Println()
	fmt.Print("Then press Enter here to read from clipboard...")
	// Wait for Enter
	fmt.Scanln()

	redirectURL, err := readClipboard()
	if err != nil {
		return fmt.Errorf("could not read clipboard: %w\nCopy the URL and try again", err)
	}
	redirectURL = strings.TrimSpace(redirectURL)

	if redirectURL == "" {
		return fmt.Errorf("clipboard is empty. Copy the URL from the browser and try again")
	}

	if !strings.Contains(redirectURL, "openid.oa2.authorization_code") {
		return fmt.Errorf("URL does not contain an authorization code. Make sure you copied the full URL from the browser after signing in")
	}

	if verbose {
		fmt.Printf("  URL length: %d chars\n", len(redirectURL))
	}

	fmt.Println("Authenticating...")
	deviceInfo, err := oauth.CreateClient(redirectURL)
	if err != nil {
		return fmt.Errorf("authentication failed: %w", err)
	}

	if err := config.Save(dir, deviceInfo); err != nil {
		return fmt.Errorf("saving session: %w", err)
	}

	name := deviceInfo.GivenName
	if name == "" {
		name = deviceInfo.Name
	}
	fmt.Printf("Logged in as %s. Session saved.\n", name)
	return nil
}

func readClipboard() (string, error) {
	switch runtime.GOOS {
	case "darwin":
		out, err := exec.Command("pbpaste").Output()
		return string(out), err
	case "linux":
		out, err := exec.Command("xclip", "-selection", "clipboard", "-o").Output()
		return string(out), err
	case "windows":
		out, err := exec.Command("powershell", "-command", "Get-Clipboard").Output()
		return string(out), err
	default:
		return "", fmt.Errorf("unsupported platform for clipboard")
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
	return cmd.Start()
}

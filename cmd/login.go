// cmd/login.go
package cmd

import (
	"bufio"
	"fmt"
	"os"
	"os/exec"
	"runtime"
	"strings"

	"github.com/juange/kindlecli/internal/amazon"
	"github.com/juange/kindlecli/internal/config"
	"github.com/spf13/cobra"
)

var loginCmd = &cobra.Command{
	Use:   "login",
	Short: "Log in to your Amazon account",
	Long:  "Opens your browser for Amazon OAuth2 login. After signing in, paste the redirect URL back here.",
	RunE:  runLogin,
}

func init() {
	rootCmd.AddCommand(loginCmd)
}

func runLogin(cmd *cobra.Command, args []string) error {
	dir := cfgDir
	if dir == "" {
		dir = config.DefaultDir()
	}

	// Check if already logged in
	if _, err := config.Load(dir); err == nil {
		fmt.Println("Already logged in. Use 'kindlecli logout' first to re-authenticate.")
		return nil
	}

	oauth := amazon.NewOAuth2()
	signinURL := oauth.GetSignInURL()

	fmt.Println("Opening browser for Amazon login...")
	if err := openBrowser(signinURL); err != nil {
		fmt.Println("Could not open browser. Please open this URL manually:")
		fmt.Println(signinURL)
	}

	fmt.Println()
	fmt.Print("After signing in, paste the redirect URL here: ")
	reader := bufio.NewReader(os.Stdin)
	redirectURL, _ := reader.ReadString('\n')
	redirectURL = strings.TrimSpace(redirectURL)

	if redirectURL == "" {
		return fmt.Errorf("no URL provided")
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

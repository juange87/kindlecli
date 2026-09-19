// cmd/root.go
package cmd

import (
	"fmt"
	"os"

	"github.com/spf13/cobra"
)

var cfgDir string
var verbose bool

var rootCmd = &cobra.Command{
	Use:           "kindlecli",
	SilenceUsage:  true,
	SilenceErrors: true,
	Short:         "Send files to your Kindle",
	Long:          "A CLI tool for sending EPUBs and PDFs to your Kindle devices via Amazon's Send-to-Kindle service.",
}

func Execute() {
	if err := rootCmd.Execute(); err != nil {
		fmt.Fprintln(os.Stderr, err)
		os.Exit(1)
	}
}

func init() {
	rootCmd.PersistentFlags().StringVar(&cfgDir, "config", "", "config directory (default: ~/.config/kindlecli)")
	rootCmd.PersistentFlags().BoolVarP(&verbose, "verbose", "v", false, "verbose output")
}

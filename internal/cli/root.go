package cli

import (
	"fmt"
	"os"

	"github.com/spf13/cobra"
)

var (
	Version     = "0.7.0"
	BuildCommit = "unknown"
	BuildDate   = "unknown"

	jsonOutput  bool
	verbose     bool
	projectRoot string
)

var rootCmd = &cobra.Command{
	Use:   "pm",
	Short: "Pommel - Semantic code search for AI agents",
	Long: `Pommel is a local-first semantic code search system designed to reduce
context window consumption for AI coding agents.

It maintains an always-current vector database of code embeddings,
enabling targeted semantic searches rather than reading numerous
files into context.`,
	Version:       Version,
	SilenceUsage:  true,
	SilenceErrors: true,
}

func Execute() error {
	return rootCmd.Execute()
}

// RegisterConfigCommand adds the config command to root for CLI usage
func RegisterConfigCommand() {
	rootCmd.AddCommand(configCmd)
}

func init() {
	rootCmd.PersistentFlags().BoolVar(&jsonOutput, "json", false, "Output in JSON format")
	rootCmd.PersistentFlags().BoolVarP(&verbose, "verbose", "v", false, "Enable verbose output")
	rootCmd.PersistentFlags().StringVarP(&projectRoot, "project", "p", "", "Project root directory (default: current directory)")
	cobra.OnInitialize(initProjectRoot)
}

func initProjectRoot() {
	if projectRoot == "" {
		var err error
		projectRoot, err = os.Getwd()
		if err != nil {
			fmt.Fprintf(os.Stderr, "Error: failed to get current directory: %v\n", err)
			os.Exit(1)
		}
	}
}

func GetProjectRoot() string {
	return projectRoot
}

func IsJSONOutput() bool {
	return jsonOutput
}

func IsVerbose() bool {
	return verbose
}

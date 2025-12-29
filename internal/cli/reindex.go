package cli

import (
	"fmt"

	"github.com/spf13/cobra"
)

var (
	reindexForce bool
)

var reindexCmd = &cobra.Command{
	Use:   "reindex",
	Short: "Trigger a full reindex of the codebase",
	Long: `Trigger a full reindex of the codebase.

This command tells the daemon to re-scan and re-index all files
in the project. Use --force to skip confirmation and force reindexing
even if an index operation is already in progress.

Examples:
  pm reindex
  pm reindex --force`,
	RunE: runReindex,
}

func init() {
	rootCmd.AddCommand(reindexCmd)
	reindexCmd.Flags().BoolVarP(&reindexForce, "force", "f", false, "Force reindex without confirmation")
}

func runReindex(cmd *cobra.Command, args []string) error {
	return fmt.Errorf("not implemented")
}

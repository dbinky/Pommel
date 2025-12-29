package cli

import (
	"fmt"

	"github.com/spf13/cobra"
)

var statusCmd = &cobra.Command{
	Use:   "status",
	Short: "Show daemon and index status",
	Long: `Display the current status of the Pommel daemon and index.

Shows information about:
  - Daemon running state and uptime
  - Index statistics (files, chunks)
  - Last indexing time
  - Dependency availability

Examples:
  pm status
  pm status --json`,
	RunE: runStatus,
}

func init() {
	rootCmd.AddCommand(statusCmd)
}

func runStatus(cmd *cobra.Command, args []string) error {
	return fmt.Errorf("not implemented")
}

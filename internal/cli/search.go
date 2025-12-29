package cli

import (
	"fmt"

	"github.com/spf13/cobra"
)

var (
	searchLimit  int
	searchLevels []string
	searchPath   string
)

var searchCmd = &cobra.Command{
	Use:   "search <query>",
	Short: "Search the codebase",
	Long: `Search the codebase using semantic search.

Performs a semantic search against the indexed codebase and returns
matching code chunks ranked by relevance.

Examples:
  pm search "authentication middleware"
  pm search "database connection" --limit 5
  pm search "error handling" --level function,method
  pm search "config parsing" --path internal/config`,
	Args: cobra.ExactArgs(1),
	RunE: runSearch,
}

func init() {
	rootCmd.AddCommand(searchCmd)
	searchCmd.Flags().IntVarP(&searchLimit, "limit", "n", 10, "Maximum results")
	searchCmd.Flags().StringSliceVarP(&searchLevels, "level", "l", nil, "Filter by level (file, class, function, method, block)")
	searchCmd.Flags().StringVar(&searchPath, "path", "", "Filter by path prefix")
}

func runSearch(cmd *cobra.Command, args []string) error {
	return fmt.Errorf("not implemented")
}

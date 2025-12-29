package cli

import (
	"fmt"
	"strings"

	"github.com/pommel-dev/pommel/internal/api"
	"github.com/spf13/cobra"
)

var (
	searchLimit      int
	searchLevels     []string
	searchPath       string
	searchAll        bool
	searchSubproject string
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
	searchCmd.Flags().BoolVar(&searchAll, "all", false, "Search entire index (no scope filtering)")
	searchCmd.Flags().StringVarP(&searchSubproject, "subproject", "s", "", "Filter by sub-project ID")
}

func runSearch(cmd *cobra.Command, args []string) error {
	query := args[0]

	client, err := NewClientFromProjectRoot(GetProjectRoot())
	if err != nil {
		return fmt.Errorf("failed to create client: %w", err)
	}

	req := api.SearchRequest{
		Query:      query,
		Limit:      searchLimit,
		Levels:     searchLevels,
		PathPrefix: searchPath,
	}

	resp, err := client.Search(req)
	if err != nil {
		return err
	}

	if IsJSONOutput() {
		return JSON(resp)
	}

	// Human-readable output
	if len(resp.Results) == 0 {
		Info("No results found for: %s", query)
		return nil
	}

	Info("Found %d results for: %s (%.0fms)\n", resp.TotalResults, resp.Query, float64(resp.SearchTimeMs))

	for i, result := range resp.Results {
		// Format: #1 [score] file:lines - name (level)
		fmt.Printf("\n#%d [%.3f] %s:%d-%d\n", i+1, result.Score, result.File, result.StartLine, result.EndLine)
		if result.Name != "" {
			fmt.Printf("   %s (%s)\n", result.Name, result.Level)
		} else {
			fmt.Printf("   (%s)\n", result.Level)
		}

		// Show truncated content preview
		content := strings.TrimSpace(result.Content)
		lines := strings.Split(content, "\n")
		maxLines := 5
		if len(lines) > maxLines {
			for _, line := range lines[:maxLines] {
				fmt.Printf("   | %s\n", line)
			}
			fmt.Printf("   | ... (%d more lines)\n", len(lines)-maxLines)
		} else {
			for _, line := range lines {
				fmt.Printf("   | %s\n", line)
			}
		}
	}

	return nil
}

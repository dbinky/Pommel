package cli

import (
	"fmt"

	"github.com/pommel-dev/pommel/internal/api"
	"github.com/spf13/cobra"
)

var subprojectsCmd = &cobra.Command{
	Use:     "subprojects",
	Aliases: []string{"sp"},
	Short:   "List detected sub-projects",
	Long: `List all detected sub-projects in the current monorepo.

Shows each sub-project's ID, path, marker file, and primary language.

Examples:
  pm subprojects
  pm subprojects --json`,
	RunE: runSubprojects,
}

func init() {
	rootCmd.AddCommand(subprojectsCmd)
}

func runSubprojects(cmd *cobra.Command, args []string) error {
	client, err := NewClientFromProjectRoot(GetProjectRoot())
	if err != nil {
		return fmt.Errorf("failed to create client: %w", err)
	}

	resp, err := client.ListSubprojects()
	if err != nil {
		return err
	}

	if IsJSONOutput() {
		return JSON(resp)
	}

	// Human-readable output
	if len(resp.Subprojects) == 0 {
		Info("No sub-projects found")
		Info("Use 'pm init --monorepo' to detect and configure sub-projects")
		return nil
	}

	fmt.Printf("Found %d sub-projects:\n\n", resp.Total)

	for _, sp := range resp.Subprojects {
		lang := sp.Language
		if lang == "" {
			lang = "unknown"
		}
		fmt.Printf("  %-15s %-30s %s (%s)\n", sp.ID, sp.Path, sp.MarkerFile, lang)
	}

	return nil
}

// ListSubprojects retrieves the list of sub-projects from the daemon
func (c *Client) ListSubprojects() (*api.SubprojectsResponse, error) {
	var resp api.SubprojectsResponse
	if err := c.get("/subprojects", &resp); err != nil {
		return nil, err
	}
	return &resp, nil
}

package cli

import (
	"fmt"
	"time"

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
	client, err := NewClientFromProjectRoot(GetProjectRoot())
	if err != nil {
		return fmt.Errorf("failed to create client: %w", err)
	}

	status, err := client.Status()
	if err != nil {
		return fmt.Errorf("daemon not running or unreachable: %w", err)
	}

	if IsJSONOutput() {
		return JSON(status)
	}

	// Human-readable output
	fmt.Println("Pommel Status")
	fmt.Println("=============")
	fmt.Println()

	// Daemon info
	fmt.Println("Daemon:")
	if status.Daemon != nil {
		fmt.Printf("  Running:  %v\n", status.Daemon.Running)
		fmt.Printf("  PID:      %d\n", status.Daemon.PID)
		fmt.Printf("  Uptime:   %s\n", formatDuration(status.Daemon.UptimeSeconds))
	} else {
		fmt.Println("  Not available")
	}
	fmt.Println()

	// Index info
	fmt.Println("Index:")
	if status.Index != nil {
		fmt.Printf("  Files:    %d\n", status.Index.TotalFiles)
		fmt.Printf("  Chunks:   %d\n", status.Index.TotalChunks)
		if !status.Index.LastIndexedAt.IsZero() {
			fmt.Printf("  Last indexed: %s\n", status.Index.LastIndexedAt.Format(time.RFC3339))
		}
		if status.Index.IndexingActive {
			if status.Index.Progress != nil {
				// Show detailed progress
				fmt.Printf("  Status:   Indexing... %.1f%% complete\n", status.Index.Progress.PercentComplete)
				fmt.Printf("  Progress: %d/%d files\n", status.Index.Progress.FilesProcessed, status.Index.Progress.FilesToProcess)
				if status.Index.Progress.ETASeconds > 0 {
					fmt.Printf("  ETA:      %s remaining\n", formatETA(status.Index.Progress.ETASeconds))
				}
			} else {
				fmt.Println("  Status:   Indexing in progress...")
			}
		} else {
			fmt.Println("  Status:   Ready")
		}
	} else {
		fmt.Println("  Not available")
	}
	fmt.Println()

	// Dependencies
	fmt.Println("Dependencies:")
	if status.Dependencies != nil {
		fmt.Printf("  Database: %s\n", boolToStatus(status.Dependencies.Database))
		fmt.Printf("  Embedder: %s\n", boolToStatus(status.Dependencies.Embedder))
	} else {
		fmt.Println("  Not available")
	}

	return nil
}

func formatDuration(seconds float64) string {
	d := time.Duration(seconds * float64(time.Second))
	if d < time.Minute {
		return fmt.Sprintf("%.0fs", seconds)
	}
	if d < time.Hour {
		return fmt.Sprintf("%.0fm %.0fs", d.Minutes(), d.Seconds()-d.Minutes()*60)
	}
	hours := int(d.Hours())
	minutes := int(d.Minutes()) % 60
	return fmt.Sprintf("%dh %dm", hours, minutes)
}

func boolToStatus(ok bool) string {
	if ok {
		return "OK"
	}
	return "Not available"
}

// formatETA formats estimated time remaining in a human-readable format
func formatETA(seconds float64) string {
	if seconds < 1 {
		return "< 1s"
	}
	d := time.Duration(seconds * float64(time.Second))
	if d < time.Minute {
		return fmt.Sprintf("%.0fs", seconds)
	}
	if d < time.Hour {
		mins := int(d.Minutes())
		secs := int(d.Seconds()) % 60
		return fmt.Sprintf("%dm %ds", mins, secs)
	}
	hours := int(d.Hours())
	mins := int(d.Minutes()) % 60
	return fmt.Sprintf("%dh %dm", hours, mins)
}

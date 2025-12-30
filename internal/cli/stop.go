package cli

import (
	"fmt"
	"time"

	"github.com/pommel-dev/pommel/internal/config"
	"github.com/pommel-dev/pommel/internal/daemon"
	"github.com/spf13/cobra"
)

var stopCmd = &cobra.Command{
	Use:   "stop",
	Short: "Stop the Pommel daemon",
	Long:  `Stop the running Pommel daemon process.`,
	RunE:  runStop,
}

func init() {
	rootCmd.AddCommand(stopCmd)
}

func runStop(cmd *cobra.Command, args []string) error {
	// Check if initialized
	loader := config.NewLoader(projectRoot)
	if !loader.Exists() {
		return ErrNotInitialized()
	}

	// Check if running
	stateManager := daemon.NewStateManager(projectRoot)
	running, pid := stateManager.IsRunning()

	if !running {
		// Check if there was a stale PID file that got cleaned up
		if pid > 0 {
			fmt.Printf("Daemon was not running (cleaned up stale PID file for PID %d)\n", pid)
		} else {
			fmt.Println("Daemon is not running")
		}
		return nil
	}

	// Terminate the process using cross-platform method
	if err := daemon.TerminateProcess(pid); err != nil {
		// Process may already be gone
		_ = stateManager.RemovePID()
		fmt.Printf("Daemon was not running (cleaned up stale PID file for PID %d)\n", pid)
		return nil
	}

	// Wait for process to exit with timeout
	timeout := time.After(5 * time.Second)
	ticker := time.NewTicker(100 * time.Millisecond)
	defer ticker.Stop()

	for {
		select {
		case <-timeout:
			// Timeout - process may be stuck, but we've already tried to kill it
			_ = stateManager.RemovePID()
			fmt.Printf("Pommel daemon terminated (PID %d)\n", pid)
			return nil
		case <-ticker.C:
			// Check if process is still running
			if !daemon.IsProcessRunning(pid) {
				// Process has exited
				_ = stateManager.RemovePID()
				fmt.Printf("Pommel daemon stopped (PID %d)\n", pid)
				return nil
			}
		}
	}
}

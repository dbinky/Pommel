package cli

import (
	"fmt"
	"net/http"
	"os/exec"
	"time"

	"github.com/pommel-dev/pommel/internal/config"
	"github.com/pommel-dev/pommel/internal/daemon"
	"github.com/spf13/cobra"
)

var startCmd = &cobra.Command{
	Use:   "start",
	Short: "Start the Pommel daemon",
	Long:  `Start the Pommel daemon process to enable file watching and semantic indexing.`,
	RunE:  runStart,
}

func init() {
	rootCmd.AddCommand(startCmd)
}

func runStart(cmd *cobra.Command, args []string) error {
	// Check if initialized
	loader := config.NewLoader(projectRoot)
	if !loader.Exists() {
		return ErrNotInitialized()
	}

	// Check if already running
	stateManager := daemon.NewStateManager(projectRoot)
	if running, pid := stateManager.IsRunning(); running {
		return ErrDaemonAlreadyRunning(pid)
	}

	// Load config
	cfg, err := loader.Load()
	if err != nil {
		return ErrConfigInvalid(err)
	}

	// Fork daemon process
	daemonCmd := exec.Command("pommeld", "--project", projectRoot)
	if err := daemonCmd.Start(); err != nil {
		return ErrDaemonStartFailed(err)
	}

	// Wait for health check
	address := cfg.Daemon.Address()
	healthURL := fmt.Sprintf("http://%s/health", address)

	timeout := time.After(10 * time.Second)
	ticker := time.NewTicker(100 * time.Millisecond)
	defer ticker.Stop()

	for {
		select {
		case <-timeout:
			// Kill the process if it didn't become healthy
			if daemonCmd.Process != nil {
				_ = daemonCmd.Process.Kill()
			}
			return ErrDaemonHealthTimeout()
		case <-ticker.C:
			resp, err := http.Get(healthURL)
			if err == nil {
				resp.Body.Close()
				if resp.StatusCode == http.StatusOK {
					fmt.Printf("Pommel daemon started (PID %d)\n", daemonCmd.Process.Pid)
					return nil
				}
			}
		}
	}
}

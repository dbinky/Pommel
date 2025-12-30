package cli

import (
	"context"
	"fmt"
	"log/slog"
	"net/http"
	"os"
	"os/exec"
	"os/signal"
	"syscall"
	"time"

	"github.com/pommel-dev/pommel/internal/config"
	"github.com/pommel-dev/pommel/internal/daemon"
	"github.com/spf13/cobra"
)

var (
	startForeground bool
)

var startCmd = &cobra.Command{
	Use:   "start",
	Short: "Start the Pommel daemon",
	Long: `Start the Pommel daemon process to enable file watching and semantic indexing.

Use --foreground to run in foreground mode for debugging.`,
	RunE: runStart,
}

func init() {
	rootCmd.AddCommand(startCmd)
	startCmd.Flags().BoolVarP(&startForeground, "foreground", "f", false, "Run daemon in foreground (for debugging)")
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

	// Handle foreground mode
	if startForeground {
		return runDaemonForeground(cfg, stateManager)
	}

	// Fork daemon process (background mode)
	daemonCmd := exec.Command("pommeld", "--project", projectRoot)
	if err := daemonCmd.Start(); err != nil {
		return ErrDaemonStartFailed(err)
	}

	// Determine port (handles nil port by calculating hash-based port)
	port, err := daemon.DeterminePort(projectRoot, cfg)
	if err != nil {
		return fmt.Errorf("failed to determine daemon port: %w", err)
	}

	// Wait for health check
	address := cfg.Daemon.AddressWithPort(port)
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

// runDaemonForeground runs the daemon in the foreground for debugging
func runDaemonForeground(cfg *config.Config, stateManager *daemon.StateManager) error {
	fmt.Println("Starting Pommel daemon in foreground mode...")
	fmt.Printf("Press Ctrl+C to stop\n\n")

	// Create logger for foreground mode (outputs to stderr)
	logger := slog.New(slog.NewTextHandler(os.Stderr, &slog.HandlerOptions{
		Level: slog.LevelInfo,
	}))

	// Create and start daemon
	d, err := daemon.New(projectRoot, cfg, logger)
	if err != nil {
		return ErrDaemonStartFailed(err)
	}

	// Handle graceful shutdown
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	sigChan := make(chan os.Signal, 1)
	signal.Notify(sigChan, syscall.SIGINT, syscall.SIGTERM)

	go func() {
		<-sigChan
		fmt.Println("\nShutting down...")
		cancel()
	}()

	// Run the daemon (blocks until context is cancelled)
	if err := d.Run(ctx); err != nil && err != context.Canceled {
		return err
	}

	fmt.Println("Daemon stopped")
	return nil
}

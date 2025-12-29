# Phase 6: CLI Commands

**Phase Goal:** Implement all CLI commands with complete functionality, JSON output, and dependency auto-setup.

**Prerequisites:** Phase 1-5 complete (all core functionality)

**Estimated Tasks:** 14 tasks across 7 commands

---

## Table of Contents

1. [Overview](#overview)
2. [Success Criteria](#success-criteria)
3. [Task 6.1: Init Command](#task-61-init-command)
4. [Task 6.2: Start Command](#task-62-start-command)
5. [Task 6.3: Stop Command](#task-63-stop-command)
6. [Task 6.4: Search Command](#task-64-search-command)
7. [Task 6.5: Status Command](#task-65-status-command)
8. [Task 6.6: Reindex Command](#task-66-reindex-command)
9. [Task 6.7: Config Command](#task-67-config-command)
10. [Testing Strategy](#testing-strategy)

---

## Overview

Phase 6 completes all CLI commands:
- `pm init` - Initialize project with dependency setup
- `pm start` - Start the daemon
- `pm stop` - Stop the daemon
- `pm search` - Query the index
- `pm status` - Show daemon and index status
- `pm reindex` - Force full reindex
- `pm config` - View/modify configuration

Each command supports `--json` for structured output.

---

## Success Criteria

| Criterion | Validation |
|-----------|------------|
| pm init | Creates .pommel/, detects deps, optionally injects CLAUDE.md |
| pm start | Forks daemon, waits for health |
| pm stop | Gracefully stops daemon |
| pm search | Returns results from daemon |
| pm status | Shows daemon and index info |
| pm reindex | Triggers full reindex |
| pm config | Shows and modifies config |
| All commands | Support --json flag |

---

## Task 6.1: Init Command

### 6.1.1 Implement Dependency Detection

**File Content (internal/setup/detector.go):**
```go
package setup

import (
	"context"
	"fmt"
	"os/exec"
	"runtime"
	"strings"
	"time"
)

// Dependency represents a required system dependency
type Dependency struct {
	Name        string
	Required    bool
	CheckCmd    string
	InstallCmd  string
	InstallHint string
}

// Status represents dependency status
type Status int

const (
	StatusMissing Status = iota
	StatusInstalled
	StatusRunning
	StatusReady
)

// DependencyStatus holds status for a dependency
type DependencyStatus struct {
	Dependency Dependency
	Status     Status
	Version    string
	Error      error
}

// Detector checks for required dependencies
type Detector struct {
	dependencies []Dependency
}

// NewDetector creates a new dependency detector
func NewDetector() *Detector {
	return &Detector{
		dependencies: []Dependency{
			{
				Name:        "ollama",
				Required:    true,
				CheckCmd:    "ollama --version",
				InstallCmd:  getOllamaInstallCmd(),
				InstallHint: "Install Ollama from https://ollama.ai",
			},
		},
	}
}

func getOllamaInstallCmd() string {
	switch runtime.GOOS {
	case "darwin":
		return "brew install ollama"
	case "linux":
		return "curl -fsSL https://ollama.ai/install.sh | sh"
	default:
		return ""
	}
}

// Check checks all dependencies
func (d *Detector) Check(ctx context.Context) []DependencyStatus {
	results := make([]DependencyStatus, len(d.dependencies))

	for i, dep := range d.dependencies {
		results[i] = d.checkDependency(ctx, dep)
	}

	return results
}

func (d *Detector) checkDependency(ctx context.Context, dep Dependency) DependencyStatus {
	status := DependencyStatus{Dependency: dep}

	// Check if installed
	parts := strings.Fields(dep.CheckCmd)
	cmd := exec.CommandContext(ctx, parts[0], parts[1:]...)
	output, err := cmd.Output()

	if err != nil {
		status.Status = StatusMissing
		status.Error = err
		return status
	}

	status.Version = strings.TrimSpace(string(output))
	status.Status = StatusInstalled

	// For Ollama, check if running
	if dep.Name == "ollama" {
		if d.isOllamaRunning(ctx) {
			status.Status = StatusRunning

			// Check if model is available
			if d.isModelAvailable(ctx, "unclemusclez/jina-embeddings-v2-base-code") {
				status.Status = StatusReady
			}
		}
	}

	return status
}

func (d *Detector) isOllamaRunning(ctx context.Context) bool {
	ctx, cancel := context.WithTimeout(ctx, 2*time.Second)
	defer cancel()

	cmd := exec.CommandContext(ctx, "curl", "-s", "http://localhost:11434")
	return cmd.Run() == nil
}

func (d *Detector) isModelAvailable(ctx context.Context, model string) bool {
	cmd := exec.CommandContext(ctx, "ollama", "list")
	output, err := cmd.Output()
	if err != nil {
		return false
	}
	return strings.Contains(string(output), model)
}

// StartOllama attempts to start the Ollama service
func (d *Detector) StartOllama(ctx context.Context) error {
	cmd := exec.CommandContext(ctx, "ollama", "serve")
	return cmd.Start()
}

// PullModel pulls the embedding model
func (d *Detector) PullModel(ctx context.Context, model string) error {
	cmd := exec.CommandContext(ctx, "ollama", "pull", model)
	cmd.Stdout = nil // Suppress output
	return cmd.Run()
}
```

---

### 6.1.2 Implement Init Command

**File Content (internal/cli/init.go):**
```go
package cli

import (
	"bufio"
	"context"
	"fmt"
	"os"
	"path/filepath"
	"strings"

	"github.com/spf13/cobra"
	"github.com/pommel-dev/pommel/internal/config"
	"github.com/pommel-dev/pommel/internal/db"
	"github.com/pommel-dev/pommel/internal/setup"
)

var (
	initStart  bool
	initClaude bool
)

var initCmd = &cobra.Command{
	Use:   "init",
	Short: "Initialize Pommel in the current project",
	Long: `Initialize Pommel by creating the .pommel directory,
configuration files, and setting up dependencies.`,
	RunE: runInit,
}

func init() {
	rootCmd.AddCommand(initCmd)
	initCmd.Flags().BoolVar(&initStart, "start", false, "Start the daemon after initialization")
	initCmd.Flags().BoolVar(&initClaude, "claude", false, "Add Pommel instructions to CLAUDE.md")
}

func runInit(cmd *cobra.Command, args []string) error {
	ctx := cmd.Context()
	out := NewOutput()

	projectRoot := GetProjectRoot()
	loader := config.NewLoader(projectRoot)

	// Check if already initialized
	if loader.Exists() {
		out.Warn("Project already initialized. Use 'pm reindex' to rebuild the index.")
		return nil
	}

	out.Info("Initializing Pommel in %s", projectRoot)

	// Check dependencies
	detector := setup.NewDetector()
	deps := detector.Check(ctx)

	for _, dep := range deps {
		switch dep.Status {
		case setup.StatusMissing:
			if dep.Dependency.Required {
				return handleMissingDependency(ctx, out, dep, detector)
			}
		case setup.StatusInstalled:
			out.Info("✓ %s installed (%s)", dep.Dependency.Name, dep.Version)
			// Try to start it
			if dep.Dependency.Name == "ollama" {
				out.Info("  Starting Ollama...")
				if err := detector.StartOllama(ctx); err != nil {
					out.Warn("  Failed to start Ollama: %v", err)
				}
			}
		case setup.StatusRunning:
			out.Info("✓ %s running", dep.Dependency.Name)
			// Check for model
			if dep.Dependency.Name == "ollama" {
				if err := ensureModel(ctx, out, detector); err != nil {
					return err
				}
			}
		case setup.StatusReady:
			out.Info("✓ %s ready", dep.Dependency.Name)
		}
	}

	// Create .pommel directory
	pommelDir := filepath.Join(projectRoot, ".pommel")
	if err := os.MkdirAll(pommelDir, 0755); err != nil {
		return fmt.Errorf("failed to create .pommel directory: %w", err)
	}

	// Create default config
	cfg := config.Default()
	if err := loader.Save(cfg); err != nil {
		return fmt.Errorf("failed to save config: %w", err)
	}
	out.Success("Created config.yaml")

	// Initialize database
	database, err := db.Open(projectRoot)
	if err != nil {
		return fmt.Errorf("failed to create database: %w", err)
	}
	if err := database.Migrate(ctx); err != nil {
		database.Close()
		return fmt.Errorf("failed to initialize database: %w", err)
	}
	database.Close()
	out.Success("Created index.db")

	// Create .pommelignore if not exists
	ignoreFile := filepath.Join(projectRoot, ".pommelignore")
	if _, err := os.Stat(ignoreFile); os.IsNotExist(err) {
		defaultIgnore := `# Pommel ignore patterns
node_modules/
vendor/
.git/
*.log
*.tmp
`
		os.WriteFile(ignoreFile, []byte(defaultIgnore), 0644)
		out.Success("Created .pommelignore")
	}

	// Handle --claude flag
	if initClaude {
		if err := injectClaudeMd(projectRoot); err != nil {
			out.Warn("Failed to update CLAUDE.md: %v", err)
		} else {
			out.Success("Updated CLAUDE.md with Pommel instructions")
		}
	}

	out.Success("Pommel initialized!")

	// Handle --start flag
	if initStart {
		out.Info("Starting daemon...")
		return runStart(cmd, args)
	}

	out.Info("Run 'pm start' to begin indexing")
	return nil
}

func handleMissingDependency(ctx context.Context, out *Output, dep setup.DependencyStatus, detector *setup.Detector) error {
	out.Error("%s is required but not installed", dep.Dependency.Name)

	if dep.Dependency.InstallCmd != "" {
		out.Info("Would you like to install it? [Y/n]")

		reader := bufio.NewReader(os.Stdin)
		response, _ := reader.ReadString('\n')
		response = strings.TrimSpace(strings.ToLower(response))

		if response == "" || response == "y" || response == "yes" {
			out.Info("Installing %s...", dep.Dependency.Name)
			// Execute install command
			// For now, just show the command
			out.Info("Run: %s", dep.Dependency.InstallCmd)
		}
	} else {
		out.Info(dep.Dependency.InstallHint)
	}

	return fmt.Errorf("%s is required", dep.Dependency.Name)
}

func ensureModel(ctx context.Context, out *Output, detector *setup.Detector) error {
	model := "unclemusclez/jina-embeddings-v2-base-code"

	if !detector.isModelAvailable(ctx, model) {
		out.Info("Pulling embedding model (this may take a few minutes)...")
		if err := detector.PullModel(ctx, model); err != nil {
			return fmt.Errorf("failed to pull model: %w", err)
		}
		out.Success("Model downloaded")
	}

	return nil
}

func injectClaudeMd(projectRoot string) error {
	claudeMdPath := filepath.Join(projectRoot, "CLAUDE.md")

	pommelSection := `
## Semantic Code Search

This project uses Pommel for semantic code search. Before reading multiple files to find relevant code, use the pm CLI:

` + "```bash" + `
# Find code related to a concept
pm search "authentication middleware" --json

# Find implementations of a pattern
pm search "retry with exponential backoff" --level method --json

# Search within a specific area
pm search "validation" --path "src/" --json
` + "```" + `

Use Pommel search results to identify specific files and line ranges, then read only those targeted sections into context.
`

	// Check if file exists
	content, err := os.ReadFile(claudeMdPath)
	if err != nil {
		if os.IsNotExist(err) {
			// Create new file
			return os.WriteFile(claudeMdPath, []byte("# CLAUDE.md\n"+pommelSection), 0644)
		}
		return err
	}

	// Check if already contains Pommel section
	if strings.Contains(string(content), "Pommel") {
		return nil // Already has it
	}

	// Append section
	return os.WriteFile(claudeMdPath, append(content, []byte("\n"+pommelSection)...), 0644)
}
```

---

## Task 6.2: Start Command

**File Content (internal/cli/start.go):**
```go
package cli

import (
	"context"
	"fmt"
	"net/http"
	"os"
	"os/exec"
	"time"

	"github.com/spf13/cobra"
	"github.com/pommel-dev/pommel/internal/config"
	"github.com/pommel-dev/pommel/internal/daemon"
)

var startCmd = &cobra.Command{
	Use:   "start",
	Short: "Start the Pommel daemon",
	Long:  "Start the Pommel daemon for the current project.",
	RunE:  runStart,
}

func init() {
	rootCmd.AddCommand(startCmd)
}

func runStart(cmd *cobra.Command, args []string) error {
	out := NewOutput()
	projectRoot := GetProjectRoot()

	// Check if initialized
	loader := config.NewLoader(projectRoot)
	if !loader.Exists() {
		return fmt.Errorf("project not initialized. Run 'pm init' first")
	}

	// Check if already running
	state := daemon.NewStateManager(projectRoot)
	if running, pid := state.IsRunning(); running {
		out.Info("Daemon already running (PID %d)", pid)
		return nil
	}

	// Load config
	cfg, err := loader.Load()
	if err != nil {
		return fmt.Errorf("failed to load config: %w", err)
	}

	// Fork daemon process
	out.Info("Starting daemon...")

	daemonPath, err := exec.LookPath("pommeld")
	if err != nil {
		// Try relative path
		daemonPath = "./bin/pommeld"
	}

	daemonCmd := exec.Command(daemonPath, "--project", projectRoot)
	daemonCmd.Stdout = nil
	daemonCmd.Stderr = nil

	if err := daemonCmd.Start(); err != nil {
		return fmt.Errorf("failed to start daemon: %w", err)
	}

	// Wait for daemon to be ready
	ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
	defer cancel()

	healthURL := fmt.Sprintf("http://%s/health", cfg.Daemon.Address())

	for {
		select {
		case <-ctx.Done():
			return fmt.Errorf("daemon failed to start (timeout)")
		default:
			resp, err := http.Get(healthURL)
			if err == nil && resp.StatusCode == http.StatusOK {
				resp.Body.Close()
				out.Success("Daemon started (PID %d)", daemonCmd.Process.Pid)
				out.Info("Listening on %s", cfg.Daemon.Address())
				return nil
			}
			time.Sleep(100 * time.Millisecond)
		}
	}
}
```

---

## Task 6.3: Stop Command

**File Content (internal/cli/stop.go):**
```go
package cli

import (
	"fmt"
	"os"
	"syscall"
	"time"

	"github.com/spf13/cobra"
	"github.com/pommel-dev/pommel/internal/daemon"
)

var stopCmd = &cobra.Command{
	Use:   "stop",
	Short: "Stop the Pommel daemon",
	Long:  "Stop the Pommel daemon for the current project.",
	RunE:  runStop,
}

func init() {
	rootCmd.AddCommand(stopCmd)
}

func runStop(cmd *cobra.Command, args []string) error {
	out := NewOutput()
	projectRoot := GetProjectRoot()

	state := daemon.NewStateManager(projectRoot)

	running, pid := state.IsRunning()
	if !running {
		out.Info("Daemon is not running")
		return nil
	}

	out.Info("Stopping daemon (PID %d)...", pid)

	// Send SIGTERM
	process, err := os.FindProcess(pid)
	if err != nil {
		return fmt.Errorf("failed to find process: %w", err)
	}

	if err := process.Signal(syscall.SIGTERM); err != nil {
		return fmt.Errorf("failed to send signal: %w", err)
	}

	// Wait for process to exit
	timeout := time.After(10 * time.Second)
	ticker := time.NewTicker(100 * time.Millisecond)
	defer ticker.Stop()

	for {
		select {
		case <-timeout:
			out.Warn("Daemon did not stop gracefully, sending SIGKILL")
			process.Kill()
			state.RemovePID()
			return nil
		case <-ticker.C:
			if err := process.Signal(syscall.Signal(0)); err != nil {
				// Process no longer exists
				state.RemovePID()
				out.Success("Daemon stopped")
				return nil
			}
		}
	}
}
```

---

## Task 6.4: Search Command

**File Content (internal/cli/search.go):**
```go
package cli

import (
	"bytes"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"strings"

	"github.com/spf13/cobra"
	"github.com/pommel-dev/pommel/internal/api"
	"github.com/pommel-dev/pommel/internal/config"
)

var (
	searchLimit  int
	searchLevels []string
	searchPath   string
)

var searchCmd = &cobra.Command{
	Use:   "search <query>",
	Short: "Search the codebase",
	Long:  "Perform a semantic search across the indexed codebase.",
	Args:  cobra.ExactArgs(1),
	RunE:  runSearch,
}

func init() {
	rootCmd.AddCommand(searchCmd)
	searchCmd.Flags().IntVarP(&searchLimit, "limit", "n", 10, "Maximum number of results")
	searchCmd.Flags().StringSliceVarP(&searchLevels, "level", "l", nil, "Filter by level: file, class, method")
	searchCmd.Flags().StringVarP(&searchPath, "path", "p", "", "Filter by path prefix")
}

func runSearch(cmd *cobra.Command, args []string) error {
	out := NewOutput()
	projectRoot := GetProjectRoot()
	query := args[0]

	// Load config
	loader := config.NewLoader(projectRoot)
	cfg, err := loader.Load()
	if err != nil {
		return fmt.Errorf("failed to load config: %w", err)
	}

	// Build request
	reqBody := api.SearchRequest{
		Query:      query,
		Limit:      searchLimit,
		Levels:     searchLevels,
		PathPrefix: searchPath,
	}

	jsonBody, err := json.Marshal(reqBody)
	if err != nil {
		return fmt.Errorf("failed to build request: %w", err)
	}

	// Send request to daemon
	url := fmt.Sprintf("http://%s/search", cfg.Daemon.Address())
	resp, err := http.Post(url, "application/json", bytes.NewReader(jsonBody))
	if err != nil {
		return fmt.Errorf("daemon not running. Start with 'pm start'")
	}
	defer resp.Body.Close()

	body, err := io.ReadAll(resp.Body)
	if err != nil {
		return fmt.Errorf("failed to read response: %w", err)
	}

	if resp.StatusCode != http.StatusOK {
		var errResp api.ErrorResponse
		json.Unmarshal(body, &errResp)
		return fmt.Errorf("search failed: %s", errResp.Error.Message)
	}

	var result api.SearchResponse
	if err := json.Unmarshal(body, &result); err != nil {
		return fmt.Errorf("failed to parse response: %w", err)
	}

	// Output results
	if jsonOutput {
		return out.JSON(result)
	}

	// Human-readable output
	out.Info("Query: %s", result.Query)
	out.Info("Found %d results in %dms\n", result.TotalResults, result.SearchTimeMs)

	for i, r := range result.Results {
		fmt.Printf("%d. %s:%d-%d (%.0f%% match)\n",
			i+1, r.File, r.StartLine, r.EndLine, r.Score*100)
		fmt.Printf("   %s: %s\n", r.Level, r.Name)

		if r.Parent != nil {
			fmt.Printf("   └─ in %s %s\n", r.Parent.Level, r.Parent.Name)
		}

		// Show snippet (first 2 lines)
		lines := strings.Split(r.Content, "\n")
		for j, line := range lines {
			if j >= 2 {
				fmt.Println("   ...")
				break
			}
			fmt.Printf("   │ %s\n", strings.TrimSpace(line))
		}
		fmt.Println()
	}

	return nil
}
```

---

## Task 6.5: Status Command

**File Content (internal/cli/status.go):**
```go
package cli

import (
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"time"

	"github.com/spf13/cobra"
	"github.com/pommel-dev/pommel/internal/api"
	"github.com/pommel-dev/pommel/internal/config"
	"github.com/pommel-dev/pommel/internal/daemon"
)

var statusCmd = &cobra.Command{
	Use:   "status",
	Short: "Show daemon and index status",
	Long:  "Display the current status of the Pommel daemon and index.",
	RunE:  runStatus,
}

func init() {
	rootCmd.AddCommand(statusCmd)
}

func runStatus(cmd *cobra.Command, args []string) error {
	out := NewOutput()
	projectRoot := GetProjectRoot()

	// Check if running
	state := daemon.NewStateManager(projectRoot)
	running, pid := state.IsRunning()

	if !running {
		if jsonOutput {
			return out.JSON(map[string]any{
				"daemon": map[string]any{"running": false},
			})
		}
		out.Info("Daemon is not running")
		out.Info("Start with: pm start")
		return nil
	}

	// Load config and fetch status
	loader := config.NewLoader(projectRoot)
	cfg, err := loader.Load()
	if err != nil {
		return fmt.Errorf("failed to load config: %w", err)
	}

	url := fmt.Sprintf("http://%s/status", cfg.Daemon.Address())
	resp, err := http.Get(url)
	if err != nil {
		return fmt.Errorf("failed to connect to daemon: %w", err)
	}
	defer resp.Body.Close()

	body, _ := io.ReadAll(resp.Body)
	var status api.StatusResponse
	json.Unmarshal(body, &status)

	if jsonOutput {
		return out.JSON(status)
	}

	// Human-readable output
	fmt.Println("Daemon Status")
	fmt.Println("─────────────")
	fmt.Printf("  Running:    yes (PID %d)\n", pid)
	fmt.Printf("  Uptime:     %s\n", formatDuration(time.Duration(status.Daemon.UptimeSeconds)*time.Second))
	fmt.Printf("  Version:    %s\n", status.Daemon.Version)
	fmt.Println()

	fmt.Println("Index Status")
	fmt.Println("────────────")
	fmt.Printf("  Files:      %d\n", status.Index.TotalFiles)
	fmt.Printf("  Chunks:     %d\n", status.Index.TotalChunks)
	fmt.Printf("  Last Index: %s\n", status.Index.LastIndexed.Format(time.RFC3339))
	fmt.Printf("  Indexing:   %v\n", status.Index.IndexingActive)
	fmt.Println()

	fmt.Println("Dependencies")
	fmt.Println("────────────")
	fmt.Printf("  Ollama:     %s\n", status.Dependencies.Ollama.Status)
	fmt.Printf("  Model:      %s\n", status.Dependencies.Ollama.Model)
	fmt.Printf("  Database:   %s\n", status.Dependencies.Database.Status)

	return nil
}

func formatDuration(d time.Duration) string {
	if d < time.Minute {
		return fmt.Sprintf("%ds", int(d.Seconds()))
	}
	if d < time.Hour {
		return fmt.Sprintf("%dm %ds", int(d.Minutes()), int(d.Seconds())%60)
	}
	return fmt.Sprintf("%dh %dm", int(d.Hours()), int(d.Minutes())%60)
}
```

---

## Task 6.6: Reindex Command

**File Content (internal/cli/reindex.go):**
```go
package cli

import (
	"bytes"
	"encoding/json"
	"fmt"
	"net/http"

	"github.com/spf13/cobra"
	"github.com/pommel-dev/pommel/internal/config"
)

var reindexForce bool

var reindexCmd = &cobra.Command{
	Use:   "reindex",
	Short: "Rebuild the search index",
	Long:  "Force a full reindex of all project files.",
	RunE:  runReindex,
}

func init() {
	rootCmd.AddCommand(reindexCmd)
	reindexCmd.Flags().BoolVar(&reindexForce, "force", false, "Force reindex even if no changes detected")
}

func runReindex(cmd *cobra.Command, args []string) error {
	out := NewOutput()
	projectRoot := GetProjectRoot()

	loader := config.NewLoader(projectRoot)
	cfg, err := loader.Load()
	if err != nil {
		return fmt.Errorf("failed to load config: %w", err)
	}

	reqBody := map[string]bool{"force": reindexForce}
	jsonBody, _ := json.Marshal(reqBody)

	url := fmt.Sprintf("http://%s/reindex", cfg.Daemon.Address())
	resp, err := http.Post(url, "application/json", bytes.NewReader(jsonBody))
	if err != nil {
		return fmt.Errorf("daemon not running. Start with 'pm start'")
	}
	defer resp.Body.Close()

	if resp.StatusCode == http.StatusAccepted {
		out.Success("Reindex started in background")
		out.Info("Use 'pm status' to monitor progress")
		return nil
	}

	return fmt.Errorf("unexpected status: %d", resp.StatusCode)
}
```

---

## Task 6.7: Config Command

**File Content (internal/cli/config.go):**
```go
package cli

import (
	"fmt"
	"strings"

	"github.com/spf13/cobra"
	"github.com/pommel-dev/pommel/internal/config"
	"gopkg.in/yaml.v3"
)

var configCmd = &cobra.Command{
	Use:   "config [get|set] [key] [value]",
	Short: "View or modify configuration",
	Long: `View or modify Pommel configuration.

Examples:
  pm config                     # Show all config
  pm config get daemon.port     # Get specific value
  pm config set daemon.port 8080 # Set value`,
	RunE: runConfig,
}

func init() {
	rootCmd.AddCommand(configCmd)
}

func runConfig(cmd *cobra.Command, args []string) error {
	out := NewOutput()
	projectRoot := GetProjectRoot()

	loader := config.NewLoader(projectRoot)
	cfg, err := loader.Load()
	if err != nil {
		return fmt.Errorf("failed to load config: %w", err)
	}

	// No args - show all
	if len(args) == 0 {
		if jsonOutput {
			return out.JSON(cfg)
		}
		data, _ := yaml.Marshal(cfg)
		fmt.Println(string(data))
		return nil
	}

	operation := args[0]

	switch operation {
	case "get":
		if len(args) < 2 {
			return fmt.Errorf("usage: pm config get <key>")
		}
		return configGet(cfg, args[1], out)

	case "set":
		if len(args) < 3 {
			return fmt.Errorf("usage: pm config set <key> <value>")
		}
		return configSet(loader, cfg, args[1], args[2], out)

	default:
		return fmt.Errorf("unknown operation: %s (use get or set)", operation)
	}
}

func configGet(cfg *config.Config, key string, out *Output) error {
	parts := strings.Split(key, ".")

	// Manual lookup for now (could use reflection)
	var value any
	switch parts[0] {
	case "daemon":
		if len(parts) > 1 {
			switch parts[1] {
			case "port":
				value = cfg.Daemon.Port
			case "host":
				value = cfg.Daemon.Host
			case "log_level":
				value = cfg.Daemon.LogLevel
			}
		} else {
			value = cfg.Daemon
		}
	case "embedding":
		if len(parts) > 1 && parts[1] == "model" {
			value = cfg.Embedding.Model
		} else {
			value = cfg.Embedding
		}
	case "search":
		value = cfg.Search
	default:
		return fmt.Errorf("unknown key: %s", key)
	}

	if jsonOutput {
		return out.JSON(map[string]any{key: value})
	}
	fmt.Printf("%s = %v\n", key, value)
	return nil
}

func configSet(loader *config.Loader, cfg *config.Config, key, value string, out *Output) error {
	parts := strings.Split(key, ".")

	// Manual set (could use reflection)
	switch parts[0] {
	case "daemon":
		if len(parts) > 1 {
			switch parts[1] {
			case "port":
				var port int
				fmt.Sscanf(value, "%d", &port)
				cfg.Daemon.Port = port
			case "host":
				cfg.Daemon.Host = value
			case "log_level":
				cfg.Daemon.LogLevel = value
			}
		}
	case "search":
		if len(parts) > 1 && parts[1] == "default_limit" {
			var limit int
			fmt.Sscanf(value, "%d", &limit)
			cfg.Search.DefaultLimit = limit
		}
	default:
		return fmt.Errorf("unknown or read-only key: %s", key)
	}

	// Validate
	if err := cfg.Validate(); err != nil {
		return fmt.Errorf("invalid value: %w", err)
	}

	// Save
	if err := loader.Save(cfg); err != nil {
		return fmt.Errorf("failed to save config: %w", err)
	}

	out.Success("Set %s = %s", key, value)
	return nil
}
```

---

## Testing Strategy

### Unit Tests for Each Command
- Test with mock daemon
- Test JSON output format
- Test error handling

### Integration Tests
```bash
# Test full workflow
pm init --start
pm status
pm search "test query"
pm config get daemon.port
pm reindex
pm stop
```

---

## Checklist

Before marking Phase 6 complete:

- [ ] pm init creates .pommel directory
- [ ] pm init detects and prompts for dependencies
- [ ] pm init --claude updates CLAUDE.md
- [ ] pm start forks daemon
- [ ] pm stop gracefully stops daemon
- [ ] pm search returns results
- [ ] pm status shows daemon and index info
- [ ] pm reindex triggers reindex
- [ ] pm config shows and modifies config
- [ ] All commands support --json
- [ ] All commands have --help

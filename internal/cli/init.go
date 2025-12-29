package cli

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"os"
	"os/exec"
	"path/filepath"
	"strings"

	"github.com/pommel-dev/pommel/internal/config"
	"github.com/pommel-dev/pommel/internal/db"
	"github.com/spf13/cobra"
)

// InitFlags holds the flags for the init command
type InitFlags struct {
	Auto   bool
	Claude bool
	Start  bool
}

var (
	initAutoFlag   bool
	initClaudeFlag bool
	initStartFlag  bool
)

var initCmd = &cobra.Command{
	Use:   "init",
	Short: "Initialize Pommel in the current project",
	Long: `Initialize Pommel by creating a .pommel directory with configuration
and database files. This sets up semantic code search for the project.

The init command will:
  - Create the .pommel directory
  - Generate a default config.yaml file
  - Initialize the SQLite database with the required schema
  - Check for required dependencies (Ollama)`,
	RunE: func(cmd *cobra.Command, args []string) error {
		flags := InitFlags{
			Auto:   initAutoFlag,
			Claude: initClaudeFlag,
			Start:  initStartFlag,
		}
		return runInitFull(GetProjectRoot(), nil, nil, IsJSONOutput(), flags)
	},
}

func init() {
	rootCmd.AddCommand(initCmd)
	initCmd.Flags().BoolVar(&initAutoFlag, "auto", false, "Auto-detect languages and configure include patterns")
	initCmd.Flags().BoolVar(&initClaudeFlag, "claude", false, "Add Pommel usage instructions to CLAUDE.md")
	initCmd.Flags().BoolVar(&initStartFlag, "start", false, "Start daemon immediately after initialization")
}

// runInit performs the initialization logic with default flags
// projectRoot is the directory to initialize Pommel in
// out and errOut are optional writers for output (nil uses default stdout/stderr)
// jsonOutput controls whether output is in JSON format
func runInit(projectRoot string, out, errOut *bytes.Buffer, jsonOutput bool) error {
	return runInitFull(projectRoot, out, errOut, jsonOutput, InitFlags{})
}

// runInitFull performs the initialization logic with all flags
// projectRoot is the directory to initialize Pommel in
// out and errOut are optional writers for output (nil uses default stdout/stderr)
// jsonOutput controls whether output is in JSON format
// flags contains the init command flags (auto, claude, start)
func runInitFull(projectRoot string, out, errOut *bytes.Buffer, jsonOutput bool, flags InitFlags) error {
	// Set up output writers
	var stdout io.Writer = os.Stdout
	var stderr io.Writer = os.Stderr
	if out != nil {
		stdout = out
	}
	if errOut != nil {
		stderr = errOut
	}

	// Check if directory exists
	info, err := os.Stat(projectRoot)
	if err != nil {
		return WrapError(err,
			fmt.Sprintf("Cannot access directory: %s", projectRoot),
			"Check that the directory exists and you have read permissions")
	}
	if !info.IsDir() {
		return ErrInvalidProjectRoot(projectRoot)
	}

	pommelDir := filepath.Join(projectRoot, config.PommelDir)
	configPath := filepath.Join(pommelDir, config.ConfigFileName+"."+config.ConfigFileExt)
	dbPath := filepath.Join(pommelDir, db.DatabaseFile)

	// Check if already initialized
	loader := config.NewLoader(projectRoot)
	if loader.Exists() {
		msg := fmt.Sprintf("Pommel already initialized at %s", pommelDir)
		if jsonOutput {
			result := InitResult{
				Success:     false,
				ProjectRoot: projectRoot,
				ConfigPath:  configPath,
				Message:     msg,
			}
			enc := json.NewEncoder(stdout)
			enc.SetIndent("", "  ")
			return enc.Encode(result)
		}
		fmt.Fprintf(stderr, "%s\n", msg)
		return nil
	}

	// Create .pommel directory
	if err := os.MkdirAll(pommelDir, 0755); err != nil {
		return WrapError(err,
			fmt.Sprintf("Cannot create .pommel directory at %s", pommelDir),
			"Check that you have write permissions in this directory")
	}

	// Create default config
	cfg := config.Default()
	if err := loader.Save(cfg); err != nil {
		return WrapError(err,
			"Failed to create configuration file",
			"Check disk space and write permissions for the .pommel directory")
	}

	// Initialize database
	ctx := context.Background()
	database, err := db.Open(projectRoot)
	if err != nil {
		return WrapError(err,
			"Failed to initialize database",
			"Check disk space and ensure SQLite is available on your system")
	}
	defer database.Close()

	if err := database.Migrate(ctx); err != nil {
		return WrapError(err,
			"Failed to set up database schema",
			"This may indicate a corrupted database. Try deleting .pommel/pommel.db and running init again")
	}

	// Create .pommelignore with default patterns
	pommelignorePath := filepath.Join(projectRoot, ".pommelignore")
	if _, err := os.Stat(pommelignorePath); os.IsNotExist(err) {
		defaultIgnore := `# Pommel ignore file
# Patterns to exclude from indexing (gitignore-style syntax)

# Dependencies
node_modules/
vendor/
.venv/

# Build outputs
bin/
obj/
dist/
build/

# IDE and editor files
.idea/
.vscode/
*.swp
*.swo

# OS files
.DS_Store
Thumbs.db

# Test coverage
coverage/
*.cover
`
		if err := os.WriteFile(pommelignorePath, []byte(defaultIgnore), 0644); err != nil {
			return WrapError(err,
				"Failed to create .pommelignore file",
				"Check write permissions in the project root directory")
		}
	}

	// Handle --auto flag: detect languages and update config
	if flags.Auto {
		detectedPatterns := detectLanguagePatterns(projectRoot)
		if len(detectedPatterns) > 0 {
			cfg.IncludePatterns = detectedPatterns
			if err := loader.Save(cfg); err != nil {
				return WrapError(err,
					"Failed to update configuration with detected languages",
					"Check disk space and write permissions for the .pommel directory")
			}
			if !jsonOutput {
				fmt.Fprintf(stdout, "Auto-detected languages: %s\n", strings.Join(detectedPatterns, ", "))
			}
		}
	}

	// Handle --claude flag: create/update CLAUDE.md
	if flags.Claude {
		if err := updateClaudeMD(projectRoot); err != nil {
			return WrapError(err,
				"Failed to update CLAUDE.md",
				"Check write permissions in the project root directory")
		}
		if !jsonOutput {
			fmt.Fprintf(stdout, "Updated CLAUDE.md with Pommel instructions\n")
		}
	}

	// Handle --start flag: start daemon after initialization
	var daemonStarted bool
	if flags.Start {
		if err := startDaemonProcess(projectRoot); err != nil {
			// Don't fail init, just warn
			fmt.Fprintf(stderr, "Warning: Failed to start daemon: %v\n", err)
		} else {
			daemonStarted = true
			if !jsonOutput {
				fmt.Fprintf(stdout, "Started Pommel daemon\n")
			}
		}
	}
	_ = daemonStarted // used for JSON output below

	// Output success
	if jsonOutput {
		result := InitResult{
			Success:      true,
			ProjectRoot:  projectRoot,
			ConfigPath:   configPath,
			DatabasePath: dbPath,
			Message:      "Initialized Pommel successfully",
		}
		enc := json.NewEncoder(stdout)
		enc.SetIndent("", "  ")
		return enc.Encode(result)
	}

	fmt.Fprintf(stdout, "Initialized Pommel in %s\n", pommelDir)
	return nil
}

// InitResult represents the result of an init operation for JSON output
type InitResult struct {
	Success      bool   `json:"success"`
	ProjectRoot  string `json:"project_root"`
	ConfigPath   string `json:"config_path"`
	DatabasePath string `json:"database_path"`
	Message      string `json:"message,omitempty"`
	Error        string `json:"error,omitempty"`
}

// Language extension mappings - maps file extension to glob pattern
var languageExtensions = map[string]string{
	".go":   "**/*.go",
	".py":   "**/*.py",
	".ts":   "**/*.ts",
	".tsx":  "**/*.tsx",
	".js":   "**/*.js",
	".jsx":  "**/*.jsx",
	".java": "**/*.java",
	".cs":   "**/*.cs",
	".rs":   "**/*.rs",
	".rb":   "**/*.rb",
	".php":  "**/*.php",
	".c":    "**/*.c",
	".cpp":  "**/*.cpp",
	".cc":   "**/*.cc",
	".cxx":  "**/*.cxx",
	".h":    "**/*.h",
	".hpp":  "**/*.hpp",
}

// detectLanguagePatterns scans the project directory for source files
// and returns appropriate include patterns for detected languages
func detectLanguagePatterns(projectRoot string) []string {
	detected := make(map[string]bool)

	filepath.Walk(projectRoot, func(path string, info os.FileInfo, err error) error {
		if err != nil {
			return nil // Skip files we can't access
		}

		// Skip hidden directories and common non-source directories
		if info.IsDir() {
			name := info.Name()
			if strings.HasPrefix(name, ".") || name == "node_modules" || name == "vendor" || name == "dist" || name == "build" {
				return filepath.SkipDir
			}
			return nil
		}

		ext := strings.ToLower(filepath.Ext(path))
		if ext == "" {
			return nil
		}

		// Map extension to glob pattern
		if pattern, ok := languageExtensions[ext]; ok {
			detected[pattern] = true
		}

		return nil
	})

	// Convert map to sorted slice
	patterns := make([]string, 0, len(detected))
	for pattern := range detected {
		patterns = append(patterns, pattern)
	}

	// Sort for consistent output
	for i := 0; i < len(patterns); i++ {
		for j := i + 1; j < len(patterns); j++ {
			if patterns[i] > patterns[j] {
				patterns[i], patterns[j] = patterns[j], patterns[i]
			}
		}
	}

	return patterns
}

// pommelClaudeInstructions contains the instructions to add to CLAUDE.md
const pommelClaudeInstructions = `
## Pommel - Semantic Code Search

This project uses Pommel for semantic code search. Use the following commands to search the codebase efficiently:

### Quick Search Examples
` + "```" + `bash
# Find code by semantic meaning (not just keywords)
pm search "authentication logic"
pm search "error handling patterns"
pm search "database connection setup"

# Search with JSON output for programmatic use
pm search "user validation" --json

# Limit results
pm search "API endpoints" --limit 5

# Search specific chunk levels
pm search "class definitions" --level class
pm search "function implementations" --level function
` + "```" + `

### Available Commands
- ` + "`pm search <query>`" + ` - Semantic search across the codebase
- ` + "`pm status`" + ` - Check daemon status and index statistics
- ` + "`pm reindex`" + ` - Force a full reindex of the codebase

### Tips
- Use natural language queries - Pommel understands semantic meaning
- Keep the daemon running (` + "`pm start`" + `) for always-current search results
- Use ` + "`--json`" + ` flag when you need structured output for processing
`

// pommelClaudeMarker is used to identify Pommel sections in CLAUDE.md
const pommelClaudeMarker = "## Pommel - Semantic Code Search"

// updateClaudeMD creates or updates CLAUDE.md with Pommel usage instructions
func updateClaudeMD(projectRoot string) error {
	claudePath := filepath.Join(projectRoot, "CLAUDE.md")

	// Check if file exists
	existingContent, err := os.ReadFile(claudePath)
	if err != nil && !os.IsNotExist(err) {
		return err
	}

	// Check if Pommel instructions already exist
	if strings.Contains(string(existingContent), pommelClaudeMarker) {
		// Already has Pommel instructions, don't duplicate
		return nil
	}

	var newContent string
	if len(existingContent) > 0 {
		// Append to existing file
		newContent = string(existingContent)
		if !strings.HasSuffix(newContent, "\n") {
			newContent += "\n"
		}
		newContent += pommelClaudeInstructions
	} else {
		// Create new file with header
		newContent = "# CLAUDE.md\n" + pommelClaudeInstructions
	}

	return os.WriteFile(claudePath, []byte(newContent), 0644)
}

// startDaemonProcess starts the pommeld daemon in the background
func startDaemonProcess(projectRoot string) error {
	// Find pommeld executable
	pommeldPath, err := exec.LookPath("pommeld")
	if err != nil {
		// Try relative to current executable
		exePath, err := os.Executable()
		if err != nil {
			return fmt.Errorf("cannot find pommeld executable: %w", err)
		}
		pommeldPath = filepath.Join(filepath.Dir(exePath), "pommeld")
		if _, err := os.Stat(pommeldPath); err != nil {
			return fmt.Errorf("cannot find pommeld executable")
		}
	}

	// Start daemon with project root
	cmd := exec.Command(pommeldPath, "-p", projectRoot)
	cmd.Dir = projectRoot

	// Detach from parent process
	if err := cmd.Start(); err != nil {
		return fmt.Errorf("failed to start daemon: %w", err)
	}

	// Don't wait for it - let it run in background
	go func() {
		cmd.Wait()
	}()

	return nil
}

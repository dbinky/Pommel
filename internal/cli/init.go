package cli

import (
	"bufio"
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"os"
	"os/exec"
	"path/filepath"
	"sort"
	"strings"

	"github.com/pommel-dev/pommel/internal/config"
	"github.com/pommel-dev/pommel/internal/db"
	"github.com/pommel-dev/pommel/internal/subproject"
	"github.com/spf13/cobra"
)

// InitFlags holds the flags for the init command
type InitFlags struct {
	Auto       bool
	Claude     bool
	Start      bool
	Monorepo   bool
	NoMonorepo bool
}

var (
	initAutoFlag     bool
	initClaudeFlag   bool
	initStartFlag    bool
	initMonorepoFlag bool
	initNoMonorepo   bool
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
			Auto:       initAutoFlag,
			Claude:     initClaudeFlag,
			Start:      initStartFlag,
			Monorepo:   initMonorepoFlag,
			NoMonorepo: initNoMonorepo,
		}
		return runInitFull(GetProjectRoot(), nil, nil, IsJSONOutput(), flags)
	},
}

func init() {
	rootCmd.AddCommand(initCmd)
	initCmd.Flags().BoolVar(&initAutoFlag, "auto", false, "Auto-detect languages and configure include patterns")
	initCmd.Flags().BoolVar(&initClaudeFlag, "claude", false, "Add Pommel usage instructions to CLAUDE.md")
	initCmd.Flags().BoolVar(&initStartFlag, "start", false, "Start daemon immediately after initialization")
	initCmd.Flags().BoolVar(&initMonorepoFlag, "monorepo", false, "Initialize as monorepo without prompting")
	initCmd.Flags().BoolVar(&initNoMonorepo, "no-monorepo", false, "Skip monorepo/sub-project detection")
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
	alreadyInitialized := loader.Exists()

	// If already initialized and no flags specified, just inform and return
	if alreadyInitialized && !flags.Auto && !flags.Claude && !flags.Start {
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

	// Load existing config if already initialized (needed for flag processing)
	var cfg *config.Config
	if alreadyInitialized {
		var err error
		cfg, err = loader.Load()
		if err != nil {
			return WrapError(err,
				"Failed to load existing configuration",
				"Check that the .pommel/config.yaml file is valid")
		}
		if !jsonOutput {
			fmt.Fprintf(stdout, "Pommel already initialized, processing flags...\n")
		}
	}

	// Only do initial setup if not already initialized
	if !alreadyInitialized {
		// Create .pommel directory
		if err := os.MkdirAll(pommelDir, 0755); err != nil {
			return WrapError(err,
				fmt.Sprintf("Cannot create .pommel directory at %s", pommelDir),
				"Check that you have write permissions in this directory")
		}

		// Create default config
		cfg = config.Default()
		if err := loader.Save(cfg); err != nil {
			return WrapError(err,
				"Failed to create configuration file",
				"Check disk space and write permissions for the .pommel directory")
		}
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

	// Add .pommel/ to .gitignore if it exists and doesn't already contain it
	if err := addToGitignore(projectRoot); err != nil {
		// Non-fatal - just log warning
		fmt.Fprintf(stderr, "Warning: Could not update .gitignore: %v\n", err)
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

	// Handle monorepo detection (unless --no-monorepo)
	var detectedSubprojects []*subproject.DetectedSubproject
	if !flags.NoMonorepo {
		detector := subproject.NewDetector(projectRoot, nil, nil)
		detected, err := detector.Scan()
		if err != nil {
			fmt.Fprintf(stderr, "Warning: Failed to scan for sub-projects: %v\n", err)
		} else {
			detectedSubprojects = detected
		}
	}

	// Handle detected subprojects
	if len(detectedSubprojects) > 0 {
		if err := handleMonorepoDetection(projectRoot, detectedSubprojects, flags, cfg, loader, stdout, stderr, jsonOutput); err != nil {
			return err
		}
	}

	// Handle --claude flag: create/update CLAUDE.md
	if flags.Claude {
		if err := updateClaudeMDFiles(projectRoot, detectedSubprojects, stdout, jsonOutput); err != nil {
			return WrapError(err,
				"Failed to update CLAUDE.md",
				"Check write permissions in the project root directory")
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

	// Output success
	if jsonOutput {
		result := InitResult{
			Success:       true,
			ProjectRoot:   projectRoot,
			ConfigPath:    configPath,
			DatabasePath:  dbPath,
			DaemonStarted: daemonStarted,
			Message:       "Initialized Pommel successfully",
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
	Success       bool   `json:"success"`
	ProjectRoot   string `json:"project_root"`
	ConfigPath    string `json:"config_path"`
	DatabasePath  string `json:"database_path"`
	DaemonStarted bool   `json:"daemon_started,omitempty"`
	Message       string `json:"message,omitempty"`
	Error         string `json:"error,omitempty"`
}

// Language extension mappings - maps file extension to glob pattern
var languageExtensions = map[string]string{
	// Supported languages (full AST-aware chunking)
	".cs":    "**/*.cs",
	".dart":  "**/*.dart",
	".ex":    "**/*.ex",
	".exs":   "**/*.exs",
	".go":    "**/*.go",
	".java":  "**/*.java",
	".js":    "**/*.js",
	".jsx":   "**/*.jsx",
	".mjs":   "**/*.mjs",
	".cjs":   "**/*.cjs",
	".kt":    "**/*.kt",
	".kts":   "**/*.kts",
	".php":   "**/*.php",
	".py":    "**/*.py",
	".pyi":   "**/*.pyi",
	".rs":    "**/*.rs",
	".sol":   "**/*.sol",
	".swift": "**/*.swift",
	".ts":    "**/*.ts",
	".tsx":   "**/*.tsx",
	".mts":   "**/*.mts",
	".cts":   "**/*.cts",
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
	sort.Strings(patterns)

	return patterns
}

// pommelClaudeInstructions contains the instructions to add to CLAUDE.md
const pommelClaudeInstructions = `
## Pommel - Semantic Code Search

This project uses Pommel (v0.5.2) for semantic code search. Pommel indexes your codebase into semantic chunks and enables natural language search with hybrid vector + keyword matching.

**Supported platforms:** macOS, Linux, Windows
**Supported languages:** C#, Dart, Elixir, Go, Java, JavaScript, Kotlin, PHP, Python, Rust, Solidity, Swift, TypeScript

### Code Search Decision Tree

**Use ` + "`pm search`" + ` FIRST for:**
- Finding specific implementations ("where is X implemented")
- Quick code lookups when you know what you're looking for
- Iterative exploration (multiple related searches)
- Cost/time-sensitive tasks (~18x fewer tokens, 1000x+ faster)

**Fall back to Explorer/Grep when:**
- Verifying something does NOT exist (Pommel may return false positives)
- Understanding architecture or code flow relationships
- Need full context around matches (not just snippets)
- Searching for exact string literals (specific error messages, identifiers)

**Decision rule:** Start with ` + "`pm search`" + `. If results seem off-topic or you need to confirm absence, use Explorer.

### When to Use Which Tool

| Use Case                         | Recommended Tool          |
|----------------------------------|---------------------------|
| Quick code lookup                | Pommel                    |
| Understanding architecture       | Explorer                  |
| Finding specific implementations | Pommel                    |
| Verifying if feature exists      | Explorer                  |
| Iterative exploration            | Pommel                    |
| Comprehensive documentation      | Explorer                  |
| Cost-sensitive workflows         | Pommel (18x fewer tokens) |
| Time-sensitive tasks             | Pommel (1000x+ faster)    |

### Quick Search Examples
` + "```" + `bash
# Find code by semantic meaning (not just keywords)
pm search "authentication logic"
pm search "error handling patterns"

# Search with JSON output for programmatic use
pm search "user validation" --json

# Limit results and filter by chunk level
pm search "API endpoints" --limit 5
pm search "class definitions" --level class

# Show detailed match reasons and score breakdown
pm search "rate limiting" --verbose
` + "```" + `

### Available Commands
- ` + "`pm search <query>`" + ` - Hybrid semantic + keyword search (~18x fewer tokens than grep)
- ` + "`pm status`" + ` - Check daemon status and index statistics
- ` + "`pm reindex`" + ` - Force a full reindex of the codebase
- ` + "`pm start`" + ` / ` + "`pm stop`" + ` - Control the background daemon

### Tips
- **Low scores (< 0.5) suggest weak matches** - consider using Explorer to confirm
- Use natural language queries - Pommel understands semantic meaning
- Keep the daemon running (` + "`pm start`" + `) for always-current search results
- Use ` + "`--verbose`" + ` to see why results matched (helpful for tuning queries)
`

// pommelClaudeMarker is used to identify Pommel sections in CLAUDE.md
const pommelClaudeMarker = "## Pommel - Semantic Code Search"

// updateClaudeMD creates or updates CLAUDE.md with Pommel usage instructions
// If an existing Pommel section is found, it is removed and replaced with the new instructions
func updateClaudeMD(projectRoot string) error {
	claudePath := filepath.Join(projectRoot, "CLAUDE.md")

	// Check if file exists
	existingContent, err := os.ReadFile(claudePath)
	if err != nil && !os.IsNotExist(err) {
		return err
	}

	var newContent string
	if len(existingContent) > 0 {
		// Remove existing Pommel section if present
		contentStr := removePommelSection(string(existingContent))

		// Append new Pommel instructions
		if !strings.HasSuffix(contentStr, "\n") {
			contentStr += "\n"
		}
		newContent = contentStr + pommelClaudeInstructions
	} else {
		// Create new file with header
		newContent = "# CLAUDE.md\n" + pommelClaudeInstructions
	}

	return os.WriteFile(claudePath, []byte(newContent), 0644)
}

// removePommelSection removes the existing Pommel section from content
// The section starts with "## Pommel - Semantic Code Search" and ends at the next "## " heading or EOF
func removePommelSection(content string) string {
	// Find the start of the Pommel section
	startIdx := strings.Index(content, pommelClaudeMarker)
	if startIdx == -1 {
		// No Pommel section found
		return content
	}

	// Find the end of the Pommel section (next ## heading or EOF)
	afterStart := content[startIdx+len(pommelClaudeMarker):]
	endIdx := -1

	// Look for the next ## heading (but not ### or deeper)
	lines := strings.Split(afterStart, "\n")
	charCount := 0
	for _, line := range lines {
		charCount += len(line) + 1 // +1 for newline
		if strings.HasPrefix(line, "## ") && !strings.HasPrefix(line, "### ") {
			// Found the next section
			endIdx = startIdx + len(pommelClaudeMarker) + charCount - len(line) - 1
			break
		}
	}

	// Build the new content
	before := content[:startIdx]
	var after string
	if endIdx != -1 {
		after = content[endIdx:]
	}

	// Clean up: remove trailing whitespace from before, but keep structure
	before = strings.TrimRight(before, " \t")
	// Ensure proper spacing
	if len(before) > 0 && !strings.HasSuffix(before, "\n\n") {
		if strings.HasSuffix(before, "\n") {
			// Already has one newline
		} else {
			before += "\n"
		}
	}

	return before + after
}

// handleMonorepoDetection handles detected subprojects during init
func handleMonorepoDetection(projectRoot string, detected []*subproject.DetectedSubproject, flags InitFlags, cfg *config.Config, loader *config.Loader, stdout, stderr io.Writer, jsonOutput bool) error {
	if !jsonOutput {
		fmt.Fprintf(stdout, "\nScanning for project markers...\n\n")
		fmt.Fprintf(stdout, "Found %d sub-projects:\n", len(detected))

		for _, sp := range detected {
			fmt.Fprintf(stdout, "  • %-15s (%s)\t%s\n", sp.ID, sp.Path, sp.MarkerFile)
		}
		fmt.Fprintln(stdout)
	}

	// Determine whether to configure as monorepo
	initAsMonorepo := flags.Monorepo

	if !flags.Monorepo && !flags.NoMonorepo && !jsonOutput {
		// Prompt user (only in interactive mode)
		initAsMonorepo = promptYesNo(stdout, nil, "Initialize as monorepo with these sub-projects?", true)
	}

	if initAsMonorepo {
		// Update config to enable subproject auto-detection
		cfg.Subprojects.AutoDetect = true
		if err := loader.Save(cfg); err != nil {
			return WrapError(err,
				"Failed to update configuration for monorepo",
				"Check disk space and write permissions for the .pommel directory")
		}
		if !jsonOutput {
			fmt.Fprintf(stdout, "Configured as monorepo with %d sub-projects\n", len(detected))
		}
	}

	return nil
}

// promptYesNo prompts for yes/no input with a default.
// stdin can be nil to use os.Stdin.
func promptYesNo(stdout io.Writer, stdin io.Reader, question string, defaultYes bool) bool {
	if stdin == nil {
		stdin = os.Stdin
	}
	reader := bufio.NewReader(stdin)
	defaultStr := "Y/n"
	if !defaultYes {
		defaultStr = "y/N"
	}

	fmt.Fprintf(stdout, "%s [%s] ", question, defaultStr)

	response, err := reader.ReadString('\n')
	if err != nil {
		return defaultYes
	}

	response = strings.TrimSpace(strings.ToLower(response))
	if response == "" {
		return defaultYes
	}

	return response == "y" || response == "yes"
}

// updateClaudeMDFiles updates CLAUDE.md in root and all subprojects
func updateClaudeMDFiles(projectRoot string, subprojects []*subproject.DetectedSubproject, stdout io.Writer, jsonOutput bool) error {
	// Update root CLAUDE.md
	if err := updateClaudeMD(projectRoot); err != nil {
		return err
	}
	if !jsonOutput {
		fmt.Fprintf(stdout, "Updated CLAUDE.md with Pommel instructions\n")
	}

	// Update each sub-project CLAUDE.md if we have subprojects
	for _, sp := range subprojects {
		spPath := filepath.Join(projectRoot, sp.Path)
		if err := updateClaudeMDForSubproject(spPath, sp); err != nil {
			// Don't fail, just report
			if !jsonOutput {
				fmt.Fprintf(stdout, "Warning: Failed to update %s/CLAUDE.md: %v\n", sp.Path, err)
			}
		} else if !jsonOutput {
			fmt.Fprintf(stdout, "Updated %s/CLAUDE.md\n", sp.Path)
		}
	}

	return nil
}

// updateClaudeMDForSubproject creates or updates CLAUDE.md for a subproject
// If an existing Pommel section is found, it is removed and replaced with the new instructions
func updateClaudeMDForSubproject(spPath string, sp *subproject.DetectedSubproject) error {
	claudePath := filepath.Join(spPath, "CLAUDE.md")

	var existingContent []byte
	existingContent, _ = os.ReadFile(claudePath)

	var newContent string
	if len(existingContent) > 0 {
		// Remove existing Pommel section if present
		contentStr := removePommelSection(string(existingContent))

		// Append new Pommel instructions
		if !strings.HasSuffix(contentStr, "\n") {
			contentStr += "\n"
		}
		newContent = contentStr + pommelSubprojectInstructions(sp)
	} else {
		newContent = "# CLAUDE.md\n" + pommelSubprojectInstructions(sp)
	}

	return os.WriteFile(claudePath, []byte(newContent), 0644)
}

// pommelSubprojectInstructions returns Pommel instructions for a subproject CLAUDE.md
func pommelSubprojectInstructions(sp *subproject.DetectedSubproject) string {
	return fmt.Sprintf(`
## Pommel - Semantic Code Search

This sub-project (%s) uses Pommel (v0.5.2) for semantic code search with hybrid vector + keyword matching.

**Supported languages:** C#, Dart, Elixir, Go, Java, JavaScript, Kotlin, PHP, Python, Rust, Solidity, Swift, TypeScript

### Code Search Decision Tree

**Use `+"`pm search`"+` FIRST for:**
- Finding specific implementations ("where is X implemented")
- Quick code lookups when you know what you're looking for
- Iterative exploration (multiple related searches)
- Cost/time-sensitive tasks (~18x fewer tokens, 1000x+ faster)

**Fall back to Explorer/Grep when:**
- Verifying something does NOT exist (Pommel may return false positives)
- Understanding architecture or code flow relationships
- Need full context around matches (not just snippets)
- Searching for exact string literals (specific error messages, identifiers)

**Decision rule:** Start with `+"`pm search`"+`. If results seem off-topic or you need to confirm absence, use Explorer.

### When to Use Which Tool

| Use Case                         | Recommended Tool          |
|----------------------------------|---------------------------|
| Quick code lookup                | Pommel                    |
| Understanding architecture       | Explorer                  |
| Finding specific implementations | Pommel                    |
| Verifying if feature exists      | Explorer                  |
| Iterative exploration            | Pommel                    |
| Cost-sensitive workflows         | Pommel (18x fewer tokens) |
| Time-sensitive tasks             | Pommel (1000x+ faster)    |

### Quick Search Examples
`+"```bash"+`
# Search within this sub-project (default when running from here)
pm search "authentication logic"

# Search with JSON output
pm search "error handling" --json

# Search across entire monorepo
pm search "shared utilities" --all

# Show detailed match reasons
pm search "rate limiting" --verbose
`+"```"+`

### Available Commands
- `+"`pm search <query>`"+` - Hybrid search (~18x fewer tokens than grep)
- `+"`pm status`"+` - Check daemon status and index statistics
- `+"`pm subprojects`"+` - List all sub-projects
- `+"`pm start`"+` / `+"`pm stop`"+` - Control the background daemon

### Tips
- **Low scores (< 0.5) suggest weak matches** - consider using Explorer to confirm
- Searches default to this sub-project when you're in this directory
- Use `+"`--all`"+` to search across the entire monorepo
- Use `+"`--verbose`"+` to see why results matched
`, sp.ID)
}

// addToGitignore adds .pommel/ to .gitignore if it doesn't already contain it
func addToGitignore(projectRoot string) error {
	gitignorePath := filepath.Join(projectRoot, ".gitignore")

	// Check if .gitignore exists
	content, err := os.ReadFile(gitignorePath)
	if os.IsNotExist(err) {
		// No .gitignore file - create one with .pommel/
		return os.WriteFile(gitignorePath, []byte(".pommel/\n"), 0644)
	}
	if err != nil {
		return err
	}

	// Check if .pommel is already in the file
	lines := strings.Split(string(content), "\n")
	for _, line := range lines {
		trimmed := strings.TrimSpace(line)
		// Check various forms that would ignore .pommel
		if trimmed == ".pommel" || trimmed == ".pommel/" || trimmed == "/.pommel" || trimmed == "/.pommel/" {
			return nil // Already present
		}
	}

	// Append .pommel/ to the file
	newContent := string(content)
	if !strings.HasSuffix(newContent, "\n") {
		newContent += "\n"
	}
	newContent += ".pommel/\n"

	return os.WriteFile(gitignorePath, []byte(newContent), 0644)
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

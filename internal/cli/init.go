package cli

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"os"
	"path/filepath"

	"github.com/pommel-dev/pommel/internal/config"
	"github.com/pommel-dev/pommel/internal/db"
	"github.com/spf13/cobra"
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
		return runInit(GetProjectRoot(), nil, nil, IsJSONOutput())
	},
}

func init() {
	rootCmd.AddCommand(initCmd)
}

// runInit performs the initialization logic
// projectRoot is the directory to initialize Pommel in
// out and errOut are optional writers for output (nil uses default stdout/stderr)
// jsonOutput controls whether output is in JSON format
func runInit(projectRoot string, out, errOut *bytes.Buffer, jsonOutput bool) error {
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
		return fmt.Errorf("failed to access directory: %w", err)
	}
	if !info.IsDir() {
		return fmt.Errorf("path is not a directory: %s", projectRoot)
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
		return fmt.Errorf("failed to create .pommel directory: %w", err)
	}

	// Create default config
	cfg := config.Default()
	if err := loader.Save(cfg); err != nil {
		return fmt.Errorf("failed to create config: %w", err)
	}

	// Initialize database
	ctx := context.Background()
	database, err := db.Open(projectRoot)
	if err != nil {
		return fmt.Errorf("failed to initialize database: %w", err)
	}
	defer database.Close()

	if err := database.Migrate(ctx); err != nil {
		return fmt.Errorf("failed to migrate database: %w", err)
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
			return fmt.Errorf("failed to create .pommelignore: %w", err)
		}
	}

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
	Success     bool   `json:"success"`
	ProjectRoot string `json:"project_root"`
	ConfigPath  string `json:"config_path"`
	DatabasePath string `json:"database_path"`
	Message     string `json:"message,omitempty"`
	Error       string `json:"error,omitempty"`
}

# Phase 13: Init Changes

**Status:** Not Started
**Effort:** Medium
**Dependencies:** Phase 12 (Runtime Detection)

---

## Objective

Enhance `pm init` to detect monorepos, prompt the user appropriately, and support new flags (`--monorepo`, `--no-monorepo`). Also update `--claude` to create/update CLAUDE.md in both the monorepo root and each sub-project.

---

## Requirements

1. Detect monorepo by scanning for marker files
2. Prompt user when sub-projects found: "Initialize as monorepo?"
3. `--monorepo` flag skips prompt, assumes yes
4. `--no-monorepo` flag skips detection entirely
5. `--claude` updates CLAUDE.md in root and all sub-project directories
6. Preserve existing `--auto`, `--start` flags

---

## Implementation Tasks

### 13.1 Add New Init Flags

**File:** `internal/cli/init.go`

```go
var (
    initAutoFlag     bool
    initClaudeFlag   bool
    initStartFlag    bool
    initMonorepoFlag bool
    initNoMonorepo   bool
)

func init() {
    rootCmd.AddCommand(initCmd)
    initCmd.Flags().BoolVar(&initAutoFlag, "auto", false, "Auto-detect languages and configure include patterns")
    initCmd.Flags().BoolVar(&initClaudeFlag, "claude", false, "Add Pommel usage instructions to CLAUDE.md")
    initCmd.Flags().BoolVar(&initStartFlag, "start", false, "Start daemon immediately after initialization")
    initCmd.Flags().BoolVar(&initMonorepoFlag, "monorepo", false, "Initialize as monorepo without prompting")
    initCmd.Flags().BoolVar(&initNoMonorepo, "no-monorepo", false, "Skip monorepo/sub-project detection")
}

type InitFlags struct {
    Auto       bool
    Claude     bool
    Start      bool
    Monorepo   bool
    NoMonorepo bool
}
```

### 13.2 Detect Monorepo During Init

**File:** `internal/cli/init.go`

```go
func runInitFull(projectRoot string, out, errOut io.Writer, jsonOutput bool, flags InitFlags) error {
    if out == nil {
        out = os.Stdout
    }
    if errOut == nil {
        errOut = os.Stderr
    }

    // Check if already initialized
    pommelDir := filepath.Join(projectRoot, ".pommel")
    alreadyInitialized := dirExists(pommelDir)

    // ... existing initialization logic ...

    // Monorepo detection (unless --no-monorepo)
    var detectedSubprojects []*subproject.DetectedSubproject
    if !flags.NoMonorepo {
        detector := subproject.NewDetector(projectRoot, nil, nil)
        detected, err := detector.Scan()
        if err != nil {
            fmt.Fprintf(errOut, "Warning: Failed to scan for sub-projects: %v\n", err)
        } else {
            detectedSubprojects = detected
        }
    }

    // Handle monorepo detection
    if len(detectedSubprojects) > 0 {
        if err := handleMonorepoDetection(projectRoot, detectedSubprojects, flags, out, errOut); err != nil {
            return err
        }
    }

    // Handle --claude flag
    if flags.Claude {
        if err := updateClaudeMDFiles(projectRoot, detectedSubprojects, out); err != nil {
            return err
        }
    }

    // ... rest of init logic ...

    return nil
}
```

### 13.3 Monorepo Detection and Prompting

**File:** `internal/cli/init.go`

```go
func handleMonorepoDetection(projectRoot string, detected []*subproject.DetectedSubproject, flags InitFlags, out, errOut io.Writer) error {
    fmt.Fprintf(out, "\nScanning for project markers...\n\n")
    fmt.Fprintf(out, "Found %d sub-projects:\n", len(detected))

    for _, sp := range detected {
        fmt.Fprintf(out, "  • %-15s (%s)\t%s\n", sp.ID, sp.Path, sp.MarkerFile)
    }

    fmt.Fprintln(out)

    // Determine whether to configure as monorepo
    initAsMonorepo := flags.Monorepo

    if !flags.Monorepo && !flags.NoMonorepo {
        // Prompt user
        initAsMonorepo = promptYesNo("Initialize as monorepo with these sub-projects?", true)
    }

    if initAsMonorepo {
        // Write subproject config
        if err := writeMonorepoConfig(projectRoot, detected); err != nil {
            return err
        }
        fmt.Fprintf(out, "✓ Configured as monorepo with %d sub-projects\n", len(detected))
    }

    return nil
}

func promptYesNo(question string, defaultYes bool) bool {
    reader := bufio.NewReader(os.Stdin)
    defaultStr := "Y/n"
    if !defaultYes {
        defaultStr = "y/N"
    }

    fmt.Printf("%s [%s] ", question, defaultStr)

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

func writeMonorepoConfig(projectRoot string, detected []*subproject.DetectedSubproject) error {
    configPath := filepath.Join(projectRoot, ".pommel", "config.yaml")

    cfg, err := config.LoadFromFile(configPath)
    if err != nil {
        cfg = config.DefaultConfig()
    }

    cfg.Subprojects.AutoDetect = true

    return config.SaveToFile(configPath, cfg)
}
```

### 13.4 Update Multiple CLAUDE.md Files

**File:** `internal/cli/init.go`

```go
func updateClaudeMDFiles(projectRoot string, subprojects []*subproject.DetectedSubproject, out io.Writer) error {
    // Update root CLAUDE.md
    if err := updateClaudeMD(projectRoot); err != nil {
        return err
    }
    fmt.Fprintf(out, "✓ Updated CLAUDE.md (monorepo root)\n")

    // Update each sub-project CLAUDE.md
    for _, sp := range subprojects {
        spPath := filepath.Join(projectRoot, sp.Path)
        if err := updateClaudeMDForSubproject(spPath, sp); err != nil {
            fmt.Fprintf(out, "✓ Created %s/CLAUDE.md\n", sp.Path)
        } else {
            fmt.Fprintf(out, "✓ Updated %s/CLAUDE.md\n", sp.Path)
        }
    }

    return nil
}

func updateClaudeMDForSubproject(spPath string, sp *subproject.DetectedSubproject) error {
    claudePath := filepath.Join(spPath, "CLAUDE.md")

    var existingContent []byte
    existingContent, _ = os.ReadFile(claudePath)

    // Check if already has Pommel section
    if strings.Contains(string(existingContent), pommelClaudeMarker) {
        return nil // Already updated
    }

    var newContent string
    if len(existingContent) > 0 {
        newContent = string(existingContent) + "\n" + pommelSubprojectInstructions(sp)
    } else {
        newContent = "# CLAUDE.md\n" + pommelSubprojectInstructions(sp)
    }

    return os.WriteFile(claudePath, []byte(newContent), 0644)
}

func pommelSubprojectInstructions(sp *subproject.DetectedSubproject) string {
    return fmt.Sprintf(`
## Pommel - Semantic Code Search

This sub-project (%s) uses Pommel for semantic code search.

### Quick Search Examples
`+"```bash"+`
# Search within this sub-project (default when running from here)
pm search "authentication logic"

# Search with JSON output
pm search "error handling" --json

# Search across entire monorepo
pm search "shared utilities" --all
`+"```"+`

### Available Commands
- `+"`pm search <query>`"+` - Search this sub-project (or use --all for everything)
- `+"`pm status`"+` - Check daemon status
- `+"`pm subprojects`"+` - List all sub-projects

### Tips
- Searches default to this sub-project when you're in this directory
- Use `+"`--all`"+` to search across the entire monorepo
- Use `+"`--path`"+` to search specific paths
`, sp.ID)
}
```

### 13.5 Update Default Config Generation

**File:** `internal/config/config.go`

```go
func DefaultConfig() *Config {
    return &Config{
        Version:     2,
        ChunkLevels: []string{"method", "class", "file"},
        IncludePatterns: []string{
            "**/*.go", "**/*.py", "**/*.js", "**/*.ts",
            "**/*.jsx", "**/*.tsx", "**/*.cs", "**/*.java",
            "**/*.rs", "**/*.c", "**/*.cpp", "**/*.h",
        },
        ExcludePatterns: []string{
            "**/node_modules/**", "**/vendor/**", "**/bin/**",
            "**/obj/**", "**/__pycache__/**", "**/target/**",
            "**/.git/**", "**/.pommel/**",
        },
        Watcher: WatcherConfig{
            DebounceMs:  500,
            MaxFileSize: 1048576,
        },
        Daemon: DaemonConfig{
            Host: "127.0.0.1",
            Port: nil, // Use hash-based port
        },
        Embedding: EmbeddingConfig{
            Model:     "unclemusclez/jina-embeddings-v2-base-code",
            OllamaURL: "http://localhost:11434",
            BatchSize: 32,
            CacheSize: 1000,
        },
        Search: SearchConfig{
            DefaultLimit:  10,
            DefaultLevels: []string{"method", "class"},
        },
        Subprojects: SubprojectsConfig{
            AutoDetect: true,
            Markers: []string{
                "*.sln", "*.csproj", "go.mod", "package.json",
                "pyproject.toml", "setup.py", "Cargo.toml",
                "pom.xml", "build.gradle",
            },
            Projects: nil,
            Exclude:  nil,
        },
    }
}
```

### 13.6 Update Init Output for JSON Mode

**File:** `internal/cli/init.go`

```go
type InitResult struct {
    ProjectRoot  string                           `json:"project_root"`
    Initialized  bool                             `json:"initialized"`
    Subprojects  []*subproject.DetectedSubproject `json:"subprojects,omitempty"`
    ClaudeMDPath []string                         `json:"claude_md_paths,omitempty"`
    DaemonPort   int                              `json:"daemon_port,omitempty"`
}

func outputInitResult(result *InitResult, jsonOutput bool, out io.Writer) error {
    if jsonOutput {
        return json.NewEncoder(out).Encode(result)
    }

    // Human-readable output already printed during execution
    return nil
}
```

---

## Testing

### Unit Tests

| Test | Description |
|------|-------------|
| `TestInit_DetectsMonorepo` | Finds sub-projects during init |
| `TestInit_MonorepoFlag` | --monorepo skips prompt |
| `TestInit_NoMonorepoFlag` | --no-monorepo skips detection |
| `TestInit_ClaudeMonorepo` | --claude updates multiple files |
| `TestPromptYesNo_DefaultYes` | Empty input returns default |
| `TestWriteMonorepoConfig` | Config includes subprojects block |

### Integration Tests

| Test | Description |
|------|-------------|
| `TestInit_FullMonorepoFlow` | Complete init with monorepo |
| `TestInit_PreservesExistingClaudeMD` | Appends to existing CLAUDE.md |
| `TestInit_AllFlagsCombined` | --auto --claude --monorepo --start |

---

## Acceptance Criteria

- [ ] `pm init` detects sub-projects and prompts user
- [ ] `pm init --monorepo` assumes yes, no prompt
- [ ] `pm init --no-monorepo` skips detection entirely
- [ ] `pm init --claude` updates CLAUDE.md at root
- [ ] `pm init --claude` on monorepo updates all sub-project CLAUDE.md files
- [ ] Existing `--auto` and `--start` flags still work
- [ ] JSON output includes detected sub-projects
- [ ] Config file includes subprojects section

---

## Files Modified

| File | Change |
|------|--------|
| `internal/cli/init.go` | New flags, monorepo detection, multi-CLAUDE.md |
| `internal/config/config.go` | Updated DefaultConfig with subprojects |
| `internal/subproject/detector.go` | Ensure exported for CLI use |

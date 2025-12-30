package config

import (
	"fmt"
	"os"
	"path/filepath"

	"github.com/spf13/viper"
)

const (
	// ConfigFileName is the name of the config file without extension
	ConfigFileName = "config"
	// ConfigFileExt is the config file extension
	ConfigFileExt = "yaml"
	// PommelDir is the name of the Pommel directory
	PommelDir = ".pommel"
)

// Loader handles configuration loading and saving
type Loader struct {
	projectRoot string
	v           *viper.Viper
}

// NewLoader creates a new config loader for the given project root
func NewLoader(projectRoot string) *Loader {
	return &Loader{
		projectRoot: projectRoot,
		v:           viper.New(),
	}
}

// ConfigPath returns the full path to the config file
func (l *Loader) ConfigPath() string {
	return filepath.Join(l.projectRoot, PommelDir, ConfigFileName+"."+ConfigFileExt)
}

// PommelDirPath returns the full path to the .pommel directory
func (l *Loader) PommelDirPath() string {
	return filepath.Join(l.projectRoot, PommelDir)
}

// Exists returns true if a config file exists at the expected location
func (l *Loader) Exists() bool {
	_, err := os.Stat(l.ConfigPath())
	return err == nil
}

// Load reads the configuration from disk
// If the config file doesn't exist, it returns an error
func (l *Loader) Load() (*Config, error) {
	if !l.Exists() {
		return nil, fmt.Errorf("config file not found at %s", l.ConfigPath())
	}

	// Create a fresh viper instance for each load to avoid stale state
	l.v = viper.New()
	l.v.SetConfigFile(l.ConfigPath())
	l.v.SetConfigType(ConfigFileExt)

	if err := l.v.ReadInConfig(); err != nil {
		return nil, fmt.Errorf("failed to read config file: %w", err)
	}

	// Use empty struct so viper values are used directly without merging with defaults
	cfg := &Config{}
	if err := l.v.Unmarshal(cfg); err != nil {
		return nil, fmt.Errorf("failed to unmarshal config: %w", err)
	}

	return cfg, nil
}

// LoadOrDefault loads the configuration from disk, or returns default config if not found
func (l *Loader) LoadOrDefault() (*Config, error) {
	if !l.Exists() {
		return Default(), nil
	}
	return l.Load()
}

// Save writes the configuration to disk
// It creates the .pommel directory if it doesn't exist
func (l *Loader) Save(cfg *Config) error {
	pommelDir := l.PommelDirPath()
	if err := os.MkdirAll(pommelDir, 0755); err != nil {
		return fmt.Errorf("failed to create .pommel directory: %w", err)
	}

	// Set all config values in viper
	l.v.Set("version", cfg.Version)
	l.v.Set("chunk_levels", cfg.ChunkLevels)
	l.v.Set("include_patterns", cfg.IncludePatterns)
	l.v.Set("exclude_patterns", cfg.ExcludePatterns)
	l.v.Set("watcher", cfg.Watcher)
	l.v.Set("daemon", cfg.Daemon)
	l.v.Set("embedding", cfg.Embedding)
	l.v.Set("search", cfg.Search)
	l.v.Set("subprojects", cfg.Subprojects)

	if err := l.v.WriteConfigAs(l.ConfigPath()); err != nil {
		return fmt.Errorf("failed to write config file: %w", err)
	}

	return nil
}

// Init initializes a new Pommel configuration in the project
// It creates the .pommel directory and writes a default config file
func (l *Loader) Init() (*Config, error) {
	if l.Exists() {
		return nil, fmt.Errorf("config already exists at %s", l.ConfigPath())
	}

	cfg := Default()
	if err := l.Save(cfg); err != nil {
		return nil, err
	}

	return cfg, nil
}

// Merge loads the existing config and merges the provided overrides
func (l *Loader) Merge(overrides map[string]interface{}) (*Config, error) {
	cfg, err := l.LoadOrDefault()
	if err != nil {
		return nil, err
	}

	// Apply overrides using viper's merge capability
	for key, value := range overrides {
		l.v.Set(key, value)
	}

	if err := l.v.Unmarshal(cfg); err != nil {
		return nil, fmt.Errorf("failed to unmarshal merged config: %w", err)
	}

	return cfg, nil
}

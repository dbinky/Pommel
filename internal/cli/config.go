package cli

import (
	"encoding/json"
	"fmt"
	"strconv"
	"strings"

	"github.com/pommel-dev/pommel/internal/config"
	"github.com/spf13/cobra"
	"gopkg.in/yaml.v3"
)

var configCmd = &cobra.Command{
	Use:   "config [get|set] [key] [value]",
	Short: "View or modify configuration",
	Long: `View or modify Pommel configuration.

Without arguments, displays the full configuration.
Use 'get <key>' to view a specific setting.
Use 'set <key> <value>' to modify a setting.

Keys use dot notation (e.g., daemon.port, embedding.batch_size).`,
	RunE: runConfig,
}

// Note: configCmd is NOT added to rootCmd here to allow standalone testing.
// The CLI main package should call RegisterConfigCommand() to enable `pm config`.

func runConfig(cmd *cobra.Command, args []string) error {
	loader := config.NewLoader(projectRoot)

	// No args - show full config
	if len(args) == 0 {
		return showFullConfig(cmd, loader)
	}

	subcommand := args[0]

	switch subcommand {
	case "get":
		if len(args) < 2 {
			return fmt.Errorf("get requires a key argument")
		}
		return getConfigValue(cmd, loader, args[1])
	case "set":
		if len(args) < 2 {
			return fmt.Errorf("set requires a key argument")
		}
		if len(args) < 3 {
			return fmt.Errorf("set requires a value argument")
		}
		return setConfigValue(cmd, loader, args[1], args[2])
	default:
		return fmt.Errorf("unknown subcommand: %s", subcommand)
	}
}

func showFullConfig(cmd *cobra.Command, loader *config.Loader) error {
	cfg, err := loader.Load()
	if err != nil {
		return fmt.Errorf("failed to load config: %w", err)
	}

	if jsonOutput {
		data, err := json.MarshalIndent(cfg, "", "  ")
		if err != nil {
			return fmt.Errorf("failed to marshal config to JSON: %w", err)
		}
		fmt.Fprintln(cmd.OutOrStdout(), string(data))
	} else {
		data, err := yaml.Marshal(cfg)
		if err != nil {
			return fmt.Errorf("failed to marshal config to YAML: %w", err)
		}
		fmt.Fprint(cmd.OutOrStdout(), string(data))
	}

	return nil
}

func getConfigValue(cmd *cobra.Command, loader *config.Loader, key string) error {
	cfg, err := loader.Load()
	if err != nil {
		return fmt.Errorf("failed to load config: %w", err)
	}

	value, err := getValueByKey(cfg, key)
	if err != nil {
		return err
	}

	if jsonOutput {
		result := map[string]interface{}{
			"key":   key,
			"value": value,
		}
		data, err := json.MarshalIndent(result, "", "  ")
		if err != nil {
			return fmt.Errorf("failed to marshal value to JSON: %w", err)
		}
		fmt.Fprintln(cmd.OutOrStdout(), string(data))
	} else {
		// For nested structures, output as YAML
		switch v := value.(type) {
		case config.DaemonConfig, config.WatcherConfig, config.EmbeddingConfig, config.SearchConfig:
			data, err := yaml.Marshal(v)
			if err != nil {
				return fmt.Errorf("failed to marshal value to YAML: %w", err)
			}
			fmt.Fprint(cmd.OutOrStdout(), string(data))
		default:
			fmt.Fprintln(cmd.OutOrStdout(), value)
		}
	}

	return nil
}

func setConfigValue(cmd *cobra.Command, loader *config.Loader, key, value string) error {
	cfg, err := loader.Load()
	if err != nil {
		return fmt.Errorf("failed to load config: %w", err)
	}

	if err := setValueByKey(cfg, key, value); err != nil {
		return err
	}

	// Validate the updated config
	if validationErrs := config.Validate(cfg); validationErrs.HasErrors() {
		return validationErrs
	}

	// Save the config
	if err := loader.Save(cfg); err != nil {
		return fmt.Errorf("failed to save config: %w", err)
	}

	fmt.Fprintf(cmd.OutOrStdout(), "Set %s = %s\n", key, value)
	return nil
}

func getValueByKey(cfg *config.Config, key string) (interface{}, error) {
	parts := strings.Split(key, ".")

	switch parts[0] {
	case "version":
		if len(parts) > 1 {
			return nil, fmt.Errorf("unknown key: %s", key)
		}
		return cfg.Version, nil
	case "daemon":
		return getDaemonValue(cfg, parts)
	case "watcher":
		return getWatcherValue(cfg, parts)
	case "embedding":
		return getEmbeddingValue(cfg, parts)
	case "search":
		return getSearchValue(cfg, parts)
	default:
		return nil, fmt.Errorf("unknown key: %s", key)
	}
}

func getDaemonValue(cfg *config.Config, parts []string) (interface{}, error) {
	if len(parts) == 1 {
		return cfg.Daemon, nil
	}
	switch parts[1] {
	case "host":
		return cfg.Daemon.Host, nil
	case "port":
		return cfg.Daemon.Port, nil
	case "log_level":
		return cfg.Daemon.LogLevel, nil
	default:
		return nil, fmt.Errorf("unknown key: %s", strings.Join(parts, "."))
	}
}

func getWatcherValue(cfg *config.Config, parts []string) (interface{}, error) {
	if len(parts) == 1 {
		return cfg.Watcher, nil
	}
	switch parts[1] {
	case "debounce_ms":
		return cfg.Watcher.DebounceMs, nil
	case "max_file_size":
		return cfg.Watcher.MaxFileSize, nil
	default:
		return nil, fmt.Errorf("unknown key: %s", strings.Join(parts, "."))
	}
}

func getEmbeddingValue(cfg *config.Config, parts []string) (interface{}, error) {
	if len(parts) == 1 {
		return cfg.Embedding, nil
	}
	switch parts[1] {
	case "model":
		return cfg.Embedding.Model, nil
	case "batch_size":
		return cfg.Embedding.BatchSize, nil
	case "cache_size":
		return cfg.Embedding.CacheSize, nil
	default:
		return nil, fmt.Errorf("unknown key: %s", strings.Join(parts, "."))
	}
}

func getSearchValue(cfg *config.Config, parts []string) (interface{}, error) {
	if len(parts) == 1 {
		return cfg.Search, nil
	}
	switch parts[1] {
	case "default_limit":
		return cfg.Search.DefaultLimit, nil
	case "default_levels":
		return cfg.Search.DefaultLevels, nil
	default:
		return nil, fmt.Errorf("unknown key: %s", strings.Join(parts, "."))
	}
}

func setValueByKey(cfg *config.Config, key, value string) error {
	parts := strings.Split(key, ".")

	switch parts[0] {
	case "daemon":
		return setDaemonValue(cfg, parts, value)
	case "watcher":
		return setWatcherValue(cfg, parts, value)
	case "embedding":
		return setEmbeddingValue(cfg, parts, value)
	case "search":
		return setSearchValue(cfg, parts, value)
	default:
		return fmt.Errorf("unknown key: %s", key)
	}
}

func setDaemonValue(cfg *config.Config, parts []string, value string) error {
	if len(parts) < 2 {
		return fmt.Errorf("unknown key: %s", strings.Join(parts, "."))
	}
	switch parts[1] {
	case "host":
		cfg.Daemon.Host = value
	case "port":
		port, err := strconv.Atoi(value)
		if err != nil {
			return fmt.Errorf("invalid integer value for daemon.port: %s", value)
		}
		cfg.Daemon.Port = port
	case "log_level":
		cfg.Daemon.LogLevel = value
	default:
		return fmt.Errorf("unknown key: %s", strings.Join(parts, "."))
	}
	return nil
}

func setWatcherValue(cfg *config.Config, parts []string, value string) error {
	if len(parts) < 2 {
		return fmt.Errorf("unknown key: %s", strings.Join(parts, "."))
	}
	switch parts[1] {
	case "debounce_ms":
		debounce, err := strconv.Atoi(value)
		if err != nil {
			return fmt.Errorf("invalid integer value for watcher.debounce_ms: %s", value)
		}
		cfg.Watcher.DebounceMs = debounce
	case "max_file_size":
		maxFileSize, err := strconv.ParseInt(value, 10, 64)
		if err != nil {
			return fmt.Errorf("invalid integer value for watcher.max_file_size: %s", value)
		}
		cfg.Watcher.MaxFileSize = maxFileSize
	default:
		return fmt.Errorf("unknown key: %s", strings.Join(parts, "."))
	}
	return nil
}

func setEmbeddingValue(cfg *config.Config, parts []string, value string) error {
	if len(parts) < 2 {
		return fmt.Errorf("unknown key: %s", strings.Join(parts, "."))
	}
	switch parts[1] {
	case "model":
		cfg.Embedding.Model = value
	case "batch_size":
		batchSize, err := strconv.Atoi(value)
		if err != nil {
			return fmt.Errorf("invalid integer value for embedding.batch_size: %s", value)
		}
		cfg.Embedding.BatchSize = batchSize
	case "cache_size":
		cacheSize, err := strconv.Atoi(value)
		if err != nil {
			return fmt.Errorf("invalid integer value for embedding.cache_size: %s", value)
		}
		cfg.Embedding.CacheSize = cacheSize
	default:
		return fmt.Errorf("unknown key: %s", strings.Join(parts, "."))
	}
	return nil
}

func setSearchValue(cfg *config.Config, parts []string, value string) error {
	if len(parts) < 2 {
		return fmt.Errorf("unknown key: %s", strings.Join(parts, "."))
	}
	switch parts[1] {
	case "default_limit":
		limit, err := strconv.Atoi(value)
		if err != nil {
			return fmt.Errorf("invalid integer value for search.default_limit: %s", value)
		}
		cfg.Search.DefaultLimit = limit
	default:
		return fmt.Errorf("unknown key: %s", strings.Join(parts, "."))
	}
	return nil
}

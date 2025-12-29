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
			return NewCLIError(
				"Missing key argument for 'config get'",
				"Usage: pm config get <key>. Example: pm config get daemon.port")
		}
		return getConfigValue(cmd, loader, args[1])
	case "set":
		if len(args) < 2 {
			return NewCLIError(
				"Missing key argument for 'config set'",
				"Usage: pm config set <key> <value>. Example: pm config set daemon.port 7420")
		}
		if len(args) < 3 {
			return NewCLIError(
				"Missing value argument for 'config set'",
				"Usage: pm config set <key> <value>. Example: pm config set daemon.port 7420")
		}
		return setConfigValue(cmd, loader, args[1], args[2])
	default:
		return NewCLIError(
			fmt.Sprintf("Unknown subcommand: %s", subcommand),
			"Valid subcommands are: get, set. Run 'pm config --help' for usage")
	}
}

func showFullConfig(cmd *cobra.Command, loader *config.Loader) error {
	cfg, err := loader.Load()
	if err != nil {
		return ErrConfigInvalid(err)
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
		return ErrConfigInvalid(err)
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
		return ErrConfigInvalid(err)
	}

	if err := setValueByKey(cfg, key, value); err != nil {
		return err
	}

	// Validate the updated config
	if validationErrs := config.Validate(cfg); validationErrs.HasErrors() {
		return WrapError(validationErrs,
			"Configuration validation failed",
			"Check the value you're trying to set is valid for this key")
	}

	// Save the config
	if err := loader.Save(cfg); err != nil {
		return WrapError(err,
			"Failed to save configuration",
			"Check write permissions for the .pommel directory")
	}

	fmt.Fprintf(cmd.OutOrStdout(), "Set %s = %s\n", key, value)
	return nil
}

func getValueByKey(cfg *config.Config, key string) (interface{}, error) {
	parts := strings.Split(key, ".")

	switch parts[0] {
	case "version":
		if len(parts) > 1 {
			return nil, unknownKeyError(key)
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
		return nil, unknownKeyError(key)
	}
}

// unknownKeyError creates a helpful error for unknown config keys.
func unknownKeyError(key string) *CLIError {
	return NewCLIError(
		fmt.Sprintf("Unknown configuration key: %s", key),
		"Valid top-level keys are: version, daemon, watcher, embedding, search. Use 'pm config' to see all available settings")
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
		return nil, NewCLIError(
			fmt.Sprintf("Unknown daemon config key: %s", parts[1]),
			"Valid daemon keys are: host, port, log_level")
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
		return nil, NewCLIError(
			fmt.Sprintf("Unknown watcher config key: %s", parts[1]),
			"Valid watcher keys are: debounce_ms, max_file_size")
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
		return nil, NewCLIError(
			fmt.Sprintf("Unknown embedding config key: %s", parts[1]),
			"Valid embedding keys are: model, batch_size, cache_size")
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
		return nil, NewCLIError(
			fmt.Sprintf("Unknown search config key: %s", parts[1]),
			"Valid search keys are: default_limit, default_levels")
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
		return unknownKeyError(key)
	}
}

// invalidIntError creates a helpful error for invalid integer values.
func invalidIntError(key, value string) *CLIError {
	return NewCLIError(
		fmt.Sprintf("Invalid value '%s' for %s: expected an integer", value, key),
		"Provide a valid integer, e.g., 'pm config set "+key+" 100'")
}

func setDaemonValue(cfg *config.Config, parts []string, value string) error {
	if len(parts) < 2 {
		return NewCLIError(
			"Cannot set daemon section directly",
			"Specify a key within daemon, e.g., 'pm config set daemon.port 7420'")
	}
	switch parts[1] {
	case "host":
		cfg.Daemon.Host = value
	case "port":
		port, err := strconv.Atoi(value)
		if err != nil {
			return invalidIntError("daemon.port", value)
		}
		cfg.Daemon.Port = port
	case "log_level":
		cfg.Daemon.LogLevel = value
	default:
		return NewCLIError(
			fmt.Sprintf("Unknown daemon config key: %s", parts[1]),
			"Valid daemon keys are: host, port, log_level")
	}
	return nil
}

func setWatcherValue(cfg *config.Config, parts []string, value string) error {
	if len(parts) < 2 {
		return NewCLIError(
			"Cannot set watcher section directly",
			"Specify a key within watcher, e.g., 'pm config set watcher.debounce_ms 100'")
	}
	switch parts[1] {
	case "debounce_ms":
		debounce, err := strconv.Atoi(value)
		if err != nil {
			return invalidIntError("watcher.debounce_ms", value)
		}
		cfg.Watcher.DebounceMs = debounce
	case "max_file_size":
		maxFileSize, err := strconv.ParseInt(value, 10, 64)
		if err != nil {
			return invalidIntError("watcher.max_file_size", value)
		}
		cfg.Watcher.MaxFileSize = maxFileSize
	default:
		return NewCLIError(
			fmt.Sprintf("Unknown watcher config key: %s", parts[1]),
			"Valid watcher keys are: debounce_ms, max_file_size")
	}
	return nil
}

func setEmbeddingValue(cfg *config.Config, parts []string, value string) error {
	if len(parts) < 2 {
		return NewCLIError(
			"Cannot set embedding section directly",
			"Specify a key within embedding, e.g., 'pm config set embedding.model mymodel'")
	}
	switch parts[1] {
	case "model":
		cfg.Embedding.Model = value
	case "batch_size":
		batchSize, err := strconv.Atoi(value)
		if err != nil {
			return invalidIntError("embedding.batch_size", value)
		}
		cfg.Embedding.BatchSize = batchSize
	case "cache_size":
		cacheSize, err := strconv.Atoi(value)
		if err != nil {
			return invalidIntError("embedding.cache_size", value)
		}
		cfg.Embedding.CacheSize = cacheSize
	default:
		return NewCLIError(
			fmt.Sprintf("Unknown embedding config key: %s", parts[1]),
			"Valid embedding keys are: model, batch_size, cache_size")
	}
	return nil
}

func setSearchValue(cfg *config.Config, parts []string, value string) error {
	if len(parts) < 2 {
		return NewCLIError(
			"Cannot set search section directly",
			"Specify a key within search, e.g., 'pm config set search.default_limit 10'")
	}
	switch parts[1] {
	case "default_limit":
		limit, err := strconv.Atoi(value)
		if err != nil {
			return invalidIntError("search.default_limit", value)
		}
		cfg.Search.DefaultLimit = limit
	default:
		return NewCLIError(
			fmt.Sprintf("Unknown search config key: %s", parts[1]),
			"Valid search keys are: default_limit")
	}
	return nil
}

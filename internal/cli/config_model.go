package cli

import (
	"fmt"
	"os"
	"path/filepath"

	"github.com/pommel-dev/pommel/internal/config"
	"github.com/pommel-dev/pommel/internal/db"
	"github.com/pommel-dev/pommel/internal/embedder"
	"github.com/spf13/cobra"
)

func init() {
	configCmd.AddCommand(newConfigModelCmd())
}

func newConfigModelCmd() *cobra.Command {
	return &cobra.Command{
		Use:   "model [v2|v4]",
		Short: "View or change the embedding model",
		Long: `View or change the embedding model used for code search.

Without arguments, shows the current model.
With an argument, switches to the specified model.

Warning: Switching models deletes the existing index and requires reindexing.

Available models:
  v2  - Jina v2 Code (~300MB, 768 dims) - lightweight, fast
  v4  - Jina v4 Code (~8GB, 1024 dims) - best quality, larger`,
		RunE: runConfigModel,
	}
}

func runConfigModel(cmd *cobra.Command, args []string) error {
	loader := config.NewLoader(projectRoot)
	cfg, err := loader.Load()
	if err != nil {
		return fmt.Errorf("failed to load config: %w", err)
	}

	// No args - show current model
	if len(args) == 0 {
		return showCurrentModel(cmd, cfg)
	}

	// Switch to requested model
	return switchModel(cmd, loader, cfg, args[0])
}

func showCurrentModel(cmd *cobra.Command, cfg *config.Config) error {
	modelName := cfg.Embedding.Ollama.Model
	if modelName == "" {
		modelName = embedder.EmbeddingModels[embedder.DefaultModel].Name
	}

	shortName := embedder.GetShortNameForModel(modelName)
	if shortName == "" {
		shortName = "custom"
	}

	fmt.Fprintf(cmd.OutOrStdout(), "Current model: %s (%s)\n", shortName, modelName)
	return nil
}

func switchModel(cmd *cobra.Command, loader *config.Loader, cfg *config.Config, target string) error {
	// Validate target model
	targetInfo, err := embedder.GetModelInfo(target)
	if err != nil {
		return err
	}

	// Check if already using this model
	currentModel := cfg.Embedding.Ollama.Model
	if currentModel == targetInfo.Name {
		shortName := embedder.GetShortNameForModel(currentModel)
		fmt.Fprintf(cmd.OutOrStdout(), "Already using model %s\n", shortName)
		return nil
	}

	// Check for existing database and delete if present
	dbPath := filepath.Join(projectRoot, ".pommel", db.DatabaseFile)
	if _, err := os.Stat(dbPath); err == nil {
		if err := os.Remove(dbPath); err != nil {
			return fmt.Errorf("failed to delete existing database: %w", err)
		}
		// Clean up WAL/SHM files (ignore errors - they may not exist)
		os.Remove(dbPath + "-wal")
		os.Remove(dbPath + "-shm")
	}

	// Update config
	cfg.Embedding.Ollama.Model = targetInfo.Name
	if err := loader.Save(cfg); err != nil {
		return fmt.Errorf("failed to save configuration: %w", err)
	}

	shortName := embedder.GetShortNameForModel(targetInfo.Name)
	fmt.Fprintf(cmd.OutOrStdout(), "Switched to model %s (%s)\n", shortName, targetInfo.Name)
	fmt.Fprintf(cmd.OutOrStdout(), "Run 'pm start' to reindex with the new model.\n")
	return nil
}

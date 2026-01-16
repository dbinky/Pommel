package cli

import (
	"fmt"

	"github.com/pommel-dev/pommel/internal/config"
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
With an argument, switches to the specified model (requires reindex).

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

	// TODO: implement model switching in Task 3.2
	return fmt.Errorf("model switching not yet implemented")
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

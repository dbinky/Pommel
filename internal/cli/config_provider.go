package cli

import (
	"bufio"
	"fmt"
	"io"
	"net/http"
	"os"
	"strconv"
	"strings"
	"time"

	"github.com/pommel-dev/pommel/internal/config"
	"github.com/spf13/cobra"
)

// APIValidator validates API keys by making a test request
type APIValidator interface {
	ValidateOpenAI(apiKey string) bool
	ValidateVoyage(apiKey string) bool
}

// RealAPIValidator makes actual API calls to validate keys
type RealAPIValidator struct {
	client *http.Client
}

func NewRealAPIValidator() *RealAPIValidator {
	return &RealAPIValidator{
		client: &http.Client{Timeout: 10 * time.Second},
	}
}

func (v *RealAPIValidator) ValidateOpenAI(apiKey string) bool {
	req, err := http.NewRequest("GET", "https://api.openai.com/v1/models", nil)
	if err != nil {
		return false
	}
	req.Header.Set("Authorization", "Bearer "+apiKey)

	resp, err := v.client.Do(req)
	if err != nil {
		return false
	}
	defer resp.Body.Close()

	return resp.StatusCode == http.StatusOK
}

func (v *RealAPIValidator) ValidateVoyage(apiKey string) bool {
	req, err := http.NewRequest("GET", "https://api.voyageai.com/v1/models", nil)
	if err != nil {
		return false
	}
	req.Header.Set("Authorization", "Bearer "+apiKey)

	resp, err := v.client.Do(req)
	if err != nil {
		return false
	}
	defer resp.Body.Close()

	return resp.StatusCode == http.StatusOK
}

// configProviderState holds the state for the config provider command
type configProviderState struct {
	validator APIValidator
	input     io.Reader
	output    io.Writer
}

var (
	configProviderAPIKey string
	configProviderURL    string
	configProviderModel  string
)

var configProviderCmd = &cobra.Command{
	Use:   "provider [name]",
	Short: "Configure embedding provider",
	Long: `Configure the embedding provider for Pommel.

Without arguments, runs an interactive setup wizard.
With a provider name, sets that provider directly.

Available providers:
  ollama         Local Ollama instance (default: localhost:11434)
  ollama-remote  Remote Ollama instance (requires --url)
  openai         OpenAI API (requires --api-key or OPENAI_API_KEY env)
  voyage         Voyage AI API (requires --api-key or VOYAGE_API_KEY env)

Examples:
  pm config provider                          # Interactive setup
  pm config provider ollama                   # Use local Ollama
  pm config provider openai --api-key sk-... # Use OpenAI with key
  pm config provider ollama-remote --url http://192.168.1.100:11434`,
	RunE: func(cmd *cobra.Command, args []string) error {
		state := &configProviderState{
			validator: NewRealAPIValidator(),
			input:     cmd.InOrStdin(),
			output:    cmd.OutOrStdout(),
		}
		return runConfigProvider(state, args)
	},
}

func init() {
	configCmd.AddCommand(configProviderCmd)
	configProviderCmd.Flags().StringVar(&configProviderAPIKey, "api-key", "", "API key for OpenAI or Voyage")
	configProviderCmd.Flags().StringVar(&configProviderURL, "url", "", "URL for remote Ollama instance")
	configProviderCmd.Flags().StringVar(&configProviderModel, "model", "", "Embedding model name")
}

// NewConfigProviderCmd creates a new config provider command for testing
func NewConfigProviderCmd() *cobra.Command {
	cmd := &cobra.Command{
		Use:   "provider [name]",
		Short: "Configure embedding provider",
		RunE: func(cmd *cobra.Command, args []string) error {
			state := &configProviderState{
				validator: NewRealAPIValidator(),
				input:     cmd.InOrStdin(),
				output:    cmd.OutOrStdout(),
			}
			return runConfigProvider(state, args)
		},
	}
	cmd.Flags().StringVar(&configProviderAPIKey, "api-key", "", "API key for OpenAI or Voyage")
	cmd.Flags().StringVar(&configProviderURL, "url", "", "URL for remote Ollama instance")
	cmd.Flags().StringVar(&configProviderModel, "model", "", "Embedding model name")
	return cmd
}

// NewConfigProviderCmdWithValidator creates a config provider command with a custom validator
func NewConfigProviderCmdWithValidator(validator APIValidator) *cobra.Command {
	cmd := &cobra.Command{
		Use:   "provider [name]",
		Short: "Configure embedding provider",
		RunE: func(cmd *cobra.Command, args []string) error {
			state := &configProviderState{
				validator: validator,
				input:     cmd.InOrStdin(),
				output:    cmd.OutOrStdout(),
			}
			return runConfigProvider(state, args)
		},
	}
	cmd.Flags().StringVar(&configProviderAPIKey, "api-key", "", "API key for OpenAI or Voyage")
	cmd.Flags().StringVar(&configProviderURL, "url", "", "URL for remote Ollama instance")
	cmd.Flags().StringVar(&configProviderModel, "model", "", "Embedding model name")
	return cmd
}

func runConfigProvider(state *configProviderState, args []string) error {
	// Load existing global config
	existingCfg, err := config.LoadGlobalConfig()
	if err != nil {
		return fmt.Errorf("failed to load global config: %w", err)
	}

	var currentProvider string
	if existingCfg != nil && existingCfg.Embedding.Provider != "" {
		currentProvider = existingCfg.Embedding.Provider
	}

	// Direct mode: provider name specified
	if len(args) > 0 {
		return runConfigProviderDirect(state, args[0], currentProvider)
	}

	// Interactive mode
	return runConfigProviderInteractive(state, currentProvider)
}

func runConfigProviderDirect(state *configProviderState, providerName, currentProvider string) error {
	// Validate provider name
	validProviders := map[string]bool{
		"ollama":        true,
		"ollama-remote": true,
		"openai":        true,
		"voyage":        true,
	}

	if !validProviders[providerName] {
		return fmt.Errorf("unknown provider '%s'; valid providers are: ollama, ollama-remote, openai, voyage", providerName)
	}

	// Build config
	cfg := &config.Config{
		Embedding: config.EmbeddingConfig{
			Provider: providerName,
		},
	}

	// Handle provider-specific settings
	switch providerName {
	case "ollama-remote":
		if configProviderURL == "" {
			return fmt.Errorf("--url is required for ollama-remote provider")
		}
		cfg.Embedding.Ollama.URL = configProviderURL

	case "openai":
		// Check for API key from flag or environment
		apiKey := configProviderAPIKey
		if apiKey == "" {
			apiKey = os.Getenv("OPENAI_API_KEY")
		}
		if apiKey == "" {
			return fmt.Errorf("API key required for OpenAI provider. Use --api-key or set OPENAI_API_KEY environment variable")
		}
		// Only store in config if from flag (not env)
		if configProviderAPIKey != "" {
			cfg.Embedding.OpenAI.APIKey = configProviderAPIKey
		}

	case "voyage":
		// Check for API key from flag or environment
		apiKey := configProviderAPIKey
		if apiKey == "" {
			apiKey = os.Getenv("VOYAGE_API_KEY")
		}
		if apiKey == "" {
			return fmt.Errorf("API key required for Voyage provider. Use --api-key or set VOYAGE_API_KEY environment variable")
		}
		// Only store in config if from flag (not env)
		if configProviderAPIKey != "" {
			cfg.Embedding.Voyage.APIKey = configProviderAPIKey
		}
	}

	// Set model if provided
	if configProviderModel != "" {
		switch providerName {
		case "ollama", "ollama-remote":
			cfg.Embedding.Ollama.Model = configProviderModel
		case "openai":
			cfg.Embedding.OpenAI.Model = configProviderModel
		case "voyage":
			cfg.Embedding.Voyage.Model = configProviderModel
		}
	}

	// Save config
	if err := config.SaveGlobalConfig(cfg); err != nil {
		return fmt.Errorf("failed to save config: %w", err)
	}

	// Show reindex warning if provider changed
	if currentProvider != "" && currentProvider != providerName {
		fmt.Fprintf(state.output, "Provider changed from %s to %s\n", currentProvider, providerName)
		fmt.Fprintln(state.output, "Note: You'll need to reindex your projects for the change to take effect.")
	}

	fmt.Fprintf(state.output, "Configured %s provider.\n", providerName)
	return nil
}

func runConfigProviderInteractive(state *configProviderState, currentProvider string) error {
	reader := bufio.NewReader(state.input)

	// Show current provider if set
	if currentProvider != "" {
		fmt.Fprintf(state.output, "Current provider: %s\n\n", currentProvider)
	}

	// Show menu
	fmt.Fprintln(state.output, "Select embedding provider:")
	fmt.Fprintln(state.output, "  1. Local Ollama (recommended for privacy)")
	fmt.Fprintln(state.output, "  2. Remote Ollama (self-hosted server)")
	fmt.Fprintln(state.output, "  3. OpenAI (best quality, requires API key)")
	fmt.Fprintln(state.output, "  4. Voyage AI (optimized for code, requires API key)")
	fmt.Fprintln(state.output)
	fmt.Fprint(state.output, "Enter choice (1-4): ")

	choice, err := reader.ReadString('\n')
	if err != nil {
		return fmt.Errorf("failed to read input: %w", err)
	}
	choice = strings.TrimSpace(choice)

	choiceNum, err := strconv.Atoi(choice)
	if err != nil || choiceNum < 1 || choiceNum > 4 {
		fmt.Fprintln(state.output, "Invalid choice. Please enter 1-4.")
		// Read next line for retry
		fmt.Fprint(state.output, "Enter choice (1-4): ")
		choice, err = reader.ReadString('\n')
		if err != nil {
			return fmt.Errorf("failed to read input: %w", err)
		}
		choice = strings.TrimSpace(choice)
		choiceNum, err = strconv.Atoi(choice)
		if err != nil || choiceNum < 1 || choiceNum > 4 {
			return fmt.Errorf("invalid choice")
		}
	}

	// Build config based on choice
	cfg := &config.Config{}
	var providerName string

	switch choiceNum {
	case 1:
		providerName = "ollama"
		cfg.Embedding.Provider = providerName
		fmt.Fprintln(state.output)
		fmt.Fprintln(state.output, "Using Local Ollama at http://localhost:11434")

	case 2:
		providerName = "ollama-remote"
		cfg.Embedding.Provider = providerName
		fmt.Fprintln(state.output)
		fmt.Fprint(state.output, "Enter Ollama URL: ")
		url, err := reader.ReadString('\n')
		if err != nil {
			return fmt.Errorf("failed to read URL: %w", err)
		}
		url = strings.TrimSpace(url)
		if url == "" {
			return fmt.Errorf("URL is required for remote Ollama")
		}
		cfg.Embedding.Ollama.URL = url

	case 3:
		providerName = "openai"
		cfg.Embedding.Provider = providerName
		fmt.Fprintln(state.output)
		fmt.Fprint(state.output, "Enter OpenAI API key (or press Enter to configure later): ")
		apiKey, err := reader.ReadString('\n')
		if err != nil {
			return fmt.Errorf("failed to read API key: %w", err)
		}
		apiKey = strings.TrimSpace(apiKey)

		if apiKey == "" {
			fmt.Fprintln(state.output, "No API key entered. You can configure later via OPENAI_API_KEY environment variable.")
		} else {
			// Validate API key
			if state.validator.ValidateOpenAI(apiKey) {
				fmt.Fprintln(state.output, "API key validated successfully!")
				cfg.Embedding.OpenAI.APIKey = apiKey
			} else {
				fmt.Fprintln(state.output, "Invalid API key. You can configure later via OPENAI_API_KEY environment variable.")
			}
		}

	case 4:
		providerName = "voyage"
		cfg.Embedding.Provider = providerName
		fmt.Fprintln(state.output)
		fmt.Fprint(state.output, "Enter Voyage API key (or press Enter to configure later): ")
		apiKey, err := reader.ReadString('\n')
		if err != nil {
			return fmt.Errorf("failed to read API key: %w", err)
		}
		apiKey = strings.TrimSpace(apiKey)

		if apiKey == "" {
			fmt.Fprintln(state.output, "No API key entered. You can configure later via VOYAGE_API_KEY environment variable.")
		} else {
			// Validate API key
			if state.validator.ValidateVoyage(apiKey) {
				fmt.Fprintln(state.output, "API key validated successfully!")
				cfg.Embedding.Voyage.APIKey = apiKey
			} else {
				fmt.Fprintln(state.output, "Invalid API key. You can configure later via VOYAGE_API_KEY environment variable.")
			}
		}
	}

	// Save config
	if err := config.SaveGlobalConfig(cfg); err != nil {
		return fmt.Errorf("failed to save config: %w", err)
	}

	// Show reindex warning if provider changed
	if currentProvider != "" && currentProvider != providerName {
		fmt.Fprintln(state.output)
		fmt.Fprintf(state.output, "Provider changed from %s to %s\n", currentProvider, providerName)
		fmt.Fprintln(state.output, "Note: You'll need to reindex your projects for the change to take effect.")
	}

	fmt.Fprintln(state.output)
	fmt.Fprintln(state.output, "Ready! Run 'pm start' in your project to begin indexing.")
	return nil
}

package embedder

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"strconv"
	"time"
)

// OpenAIConfig holds configuration for the OpenAI embedding client.
type OpenAIConfig struct {
	APIKey  string
	Model   string
	BaseURL string
	Timeout time.Duration
}

// DefaultOpenAIConfig returns the default configuration for OpenAI.
func DefaultOpenAIConfig() OpenAIConfig {
	return OpenAIConfig{
		Model:   "text-embedding-3-small",
		BaseURL: "https://api.openai.com",
		Timeout: 30 * time.Second,
	}
}

// OpenAIClient provides embedding generation via OpenAI's API.
type OpenAIClient struct {
	apiKey     string
	model      string
	baseURL    string
	httpClient *http.Client
}

// openAIEmbedRequest represents the request to OpenAI's embeddings endpoint.
type openAIEmbedRequest struct {
	Model string `json:"model"`
	Input any    `json:"input"`
}

// openAIEmbedResponse represents the response from OpenAI's embeddings endpoint.
type openAIEmbedResponse struct {
	Object string `json:"object"`
	Data   []struct {
		Object    string    `json:"object"`
		Index     int       `json:"index"`
		Embedding []float64 `json:"embedding"`
	} `json:"data"`
	Model string `json:"model"`
	Usage struct {
		PromptTokens int `json:"prompt_tokens"`
		TotalTokens  int `json:"total_tokens"`
	} `json:"usage"`
}

// openAIErrorResponse represents an error response from OpenAI.
type openAIErrorResponse struct {
	Error struct {
		Message string `json:"message"`
		Type    string `json:"type"`
		Code    string `json:"code"`
	} `json:"error"`
}

// NewOpenAIClient creates a new OpenAI embedding client.
func NewOpenAIClient(cfg OpenAIConfig) *OpenAIClient {
	defaults := DefaultOpenAIConfig()

	if cfg.Model == "" {
		cfg.Model = defaults.Model
	}
	if cfg.BaseURL == "" {
		cfg.BaseURL = defaults.BaseURL
	}
	if cfg.Timeout == 0 {
		cfg.Timeout = defaults.Timeout
	}

	return &OpenAIClient{
		apiKey:  cfg.APIKey,
		model:   cfg.Model,
		baseURL: cfg.BaseURL,
		httpClient: &http.Client{
			Timeout: cfg.Timeout,
		},
	}
}

// Health checks if the OpenAI API is accessible with valid credentials.
func (c *OpenAIClient) Health(ctx context.Context) error {
	// Do a minimal embedding request to verify credentials
	_, err := c.EmbedSingle(ctx, "health check")
	return err
}

// EmbedSingle generates an embedding for a single text input.
func (c *OpenAIClient) EmbedSingle(ctx context.Context, text string) ([]float32, error) {
	embeddings, err := c.embed(ctx, text)
	if err != nil {
		return nil, err
	}
	if len(embeddings) == 0 {
		return nil, &EmbeddingError{
			Code:    "EMBEDDING_EMPTY",
			Message: "OpenAI returned no embeddings for the input",
		}
	}
	return embeddings[0], nil
}

// Embed generates embeddings for multiple texts in a single request.
func (c *OpenAIClient) Embed(ctx context.Context, texts []string) ([][]float32, error) {
	if len(texts) == 0 {
		return [][]float32{}, nil
	}
	return c.embed(ctx, texts)
}

// embed is the internal method that handles the actual embedding request.
func (c *OpenAIClient) embed(ctx context.Context, input any) ([][]float32, error) {
	reqBody := openAIEmbedRequest{
		Model: c.model,
		Input: input,
	}

	jsonBody, err := json.Marshal(reqBody)
	if err != nil {
		return nil, fmt.Errorf("failed to marshal request: %w", err)
	}

	req, err := http.NewRequestWithContext(ctx, http.MethodPost, c.baseURL+"/v1/embeddings", bytes.NewReader(jsonBody))
	if err != nil {
		return nil, fmt.Errorf("failed to create request: %w", err)
	}

	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("Authorization", "Bearer "+c.apiKey)

	resp, err := c.httpClient.Do(req)
	if err != nil {
		// Check if context was cancelled
		if ctx.Err() != nil {
			return nil, ctx.Err()
		}
		return nil, ErrProviderUnavailable.WithCause(err)
	}
	defer resp.Body.Close()

	body, err := io.ReadAll(resp.Body)
	if err != nil {
		return nil, fmt.Errorf("failed to read response body: %w", err)
	}

	// Handle error responses
	if resp.StatusCode != http.StatusOK {
		return nil, c.handleErrorResponse(resp, body)
	}

	var embedResp openAIEmbedResponse
	if err := json.Unmarshal(body, &embedResp); err != nil {
		return nil, &EmbeddingError{
			Code:    "INVALID_RESPONSE",
			Message: "received invalid response from OpenAI",
			Cause:   err,
		}
	}

	// Convert to float32 and ensure correct order
	result := make([][]float32, len(embedResp.Data))
	for _, item := range embedResp.Data {
		embedding := make([]float32, len(item.Embedding))
		for i, v := range item.Embedding {
			embedding[i] = float32(v)
		}
		if item.Index < len(result) {
			result[item.Index] = embedding
		}
	}

	return result, nil
}

// handleErrorResponse processes error responses from OpenAI.
func (c *OpenAIClient) handleErrorResponse(resp *http.Response, body []byte) error {
	var errResp openAIErrorResponse
	json.Unmarshal(body, &errResp) // Ignore unmarshal errors, use status code

	switch resp.StatusCode {
	case http.StatusUnauthorized:
		return &EmbeddingError{
			Code:       "AUTH_FAILED",
			Message:    "Invalid OpenAI API key",
			Suggestion: "Run 'pm config provider' to update your API key",
			Retryable:  false,
		}

	case http.StatusTooManyRequests:
		retryAfter := parseRetryAfter(resp.Header.Get("Retry-After"))
		return &EmbeddingError{
			Code:       "RATE_LIMITED",
			Message:    "OpenAI rate limit exceeded",
			Suggestion: "Waiting to retry automatically...",
			Retryable:  true,
			RetryAfter: retryAfter,
		}

	case http.StatusPaymentRequired:
		return &EmbeddingError{
			Code:       "QUOTA_EXCEEDED",
			Message:    "OpenAI quota exhausted",
			Suggestion: "Check your billing at platform.openai.com",
			Retryable:  false,
		}

	case http.StatusBadRequest:
		return &EmbeddingError{
			Code:      "INVALID_REQUEST",
			Message:   fmt.Sprintf("Invalid request: %s", errResp.Error.Message),
			Retryable: false,
		}

	default:
		// 5xx errors are retryable
		retryable := resp.StatusCode >= 500
		return &EmbeddingError{
			Code:      "REQUEST_FAILED",
			Message:   fmt.Sprintf("OpenAI request failed with status %d: %s", resp.StatusCode, errResp.Error.Message),
			Retryable: retryable,
		}
	}
}

// parseRetryAfter parses the Retry-After header value.
func parseRetryAfter(value string) time.Duration {
	if value == "" {
		return 0
	}
	seconds, err := strconv.Atoi(value)
	if err != nil {
		return 0
	}
	return time.Duration(seconds) * time.Second
}

// ModelName returns the configured model name.
func (c *OpenAIClient) ModelName() string {
	return c.model
}

// Dimensions returns the embedding dimension size.
func (c *OpenAIClient) Dimensions() int {
	return 1536 // text-embedding-3-small
}

// Compile-time check that OpenAIClient implements Embedder
var _ Embedder = (*OpenAIClient)(nil)

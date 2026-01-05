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

// VoyageConfig holds configuration for the Voyage AI embedding client.
type VoyageConfig struct {
	APIKey  string
	Model   string
	BaseURL string
	Timeout time.Duration
}

// DefaultVoyageConfig returns the default configuration for Voyage AI.
func DefaultVoyageConfig() VoyageConfig {
	return VoyageConfig{
		Model:   "voyage-code-3",
		BaseURL: "https://api.voyageai.com",
		Timeout: 30 * time.Second,
	}
}

// VoyageClient provides embedding generation via Voyage AI's API.
type VoyageClient struct {
	apiKey     string
	model      string
	baseURL    string
	httpClient *http.Client
}

// voyageEmbedRequest represents the request to Voyage's embeddings endpoint.
type voyageEmbedRequest struct {
	Model     string `json:"model"`
	Input     any    `json:"input"`
	InputType string `json:"input_type"`
}

// voyageEmbedResponse represents the response from Voyage's embeddings endpoint.
type voyageEmbedResponse struct {
	Object string `json:"object"`
	Data   []struct {
		Object    string    `json:"object"`
		Index     int       `json:"index"`
		Embedding []float64 `json:"embedding"`
	} `json:"data"`
	Model string `json:"model"`
	Usage struct {
		TotalTokens int `json:"total_tokens"`
	} `json:"usage"`
}

// voyageErrorResponse represents an error response from Voyage.
type voyageErrorResponse struct {
	Detail string `json:"detail"`
}

// NewVoyageClient creates a new Voyage AI embedding client.
func NewVoyageClient(cfg VoyageConfig) *VoyageClient {
	defaults := DefaultVoyageConfig()

	if cfg.Model == "" {
		cfg.Model = defaults.Model
	}
	if cfg.BaseURL == "" {
		cfg.BaseURL = defaults.BaseURL
	}
	if cfg.Timeout == 0 {
		cfg.Timeout = defaults.Timeout
	}

	return &VoyageClient{
		apiKey:  cfg.APIKey,
		model:   cfg.Model,
		baseURL: cfg.BaseURL,
		httpClient: &http.Client{
			Timeout: cfg.Timeout,
		},
	}
}

// Health checks if the Voyage API is accessible with valid credentials.
func (c *VoyageClient) Health(ctx context.Context) error {
	_, err := c.EmbedSingle(ctx, "health check")
	return err
}

// EmbedSingle generates an embedding for a single text input.
func (c *VoyageClient) EmbedSingle(ctx context.Context, text string) ([]float32, error) {
	embeddings, err := c.embed(ctx, text)
	if err != nil {
		return nil, err
	}
	if len(embeddings) == 0 {
		return nil, &EmbeddingError{
			Code:    "EMBEDDING_EMPTY",
			Message: "Voyage returned no embeddings for the input",
		}
	}
	return embeddings[0], nil
}

// Embed generates embeddings for multiple texts in a single request.
func (c *VoyageClient) Embed(ctx context.Context, texts []string) ([][]float32, error) {
	if len(texts) == 0 {
		return [][]float32{}, nil
	}
	return c.embed(ctx, texts)
}

// embed is the internal method that handles the actual embedding request.
func (c *VoyageClient) embed(ctx context.Context, input any) ([][]float32, error) {
	reqBody := voyageEmbedRequest{
		Model:     c.model,
		Input:     input,
		InputType: "document", // Use document for code indexing
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

	if resp.StatusCode != http.StatusOK {
		return nil, c.handleErrorResponse(resp, body)
	}

	var embedResp voyageEmbedResponse
	if err := json.Unmarshal(body, &embedResp); err != nil {
		return nil, &EmbeddingError{
			Code:    "INVALID_RESPONSE",
			Message: "received invalid response from Voyage",
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

// handleErrorResponse processes error responses from Voyage.
func (c *VoyageClient) handleErrorResponse(resp *http.Response, body []byte) error {
	var errResp voyageErrorResponse
	json.Unmarshal(body, &errResp)

	switch resp.StatusCode {
	case http.StatusUnauthorized:
		return &EmbeddingError{
			Code:       "AUTH_FAILED",
			Message:    "Invalid Voyage API key",
			Suggestion: "Run 'pm config provider' to update your API key",
			Retryable:  false,
		}

	case http.StatusTooManyRequests:
		retryAfter := parseVoyageRetryAfter(resp.Header.Get("Retry-After"))
		return &EmbeddingError{
			Code:       "RATE_LIMITED",
			Message:    "Voyage rate limit exceeded",
			Suggestion: "Waiting to retry automatically...",
			Retryable:  true,
			RetryAfter: retryAfter,
		}

	case http.StatusPaymentRequired:
		return &EmbeddingError{
			Code:       "QUOTA_EXCEEDED",
			Message:    "Voyage quota exhausted",
			Suggestion: "Check your billing at dash.voyageai.com",
			Retryable:  false,
		}

	case http.StatusBadRequest:
		return &EmbeddingError{
			Code:      "INVALID_REQUEST",
			Message:   fmt.Sprintf("Invalid request: %s", errResp.Detail),
			Retryable: false,
		}

	default:
		retryable := resp.StatusCode >= 500
		return &EmbeddingError{
			Code:      "REQUEST_FAILED",
			Message:   fmt.Sprintf("Voyage request failed with status %d: %s", resp.StatusCode, errResp.Detail),
			Retryable: retryable,
		}
	}
}

// parseVoyageRetryAfter parses the Retry-After header value.
func parseVoyageRetryAfter(value string) time.Duration {
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
func (c *VoyageClient) ModelName() string {
	return c.model
}

// Dimensions returns the embedding dimension size.
func (c *VoyageClient) Dimensions() int {
	return 1024 // voyage-code-3
}

// Compile-time check that VoyageClient implements Embedder
var _ Embedder = (*VoyageClient)(nil)

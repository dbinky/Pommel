package embedder

import (
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// === Happy Path Tests ===

func TestOpenAIClient_EmbedSingle_Success(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		assert.Equal(t, "POST", r.Method)
		assert.Equal(t, "/v1/embeddings", r.URL.Path)
		assert.Equal(t, "application/json", r.Header.Get("Content-Type"))
		assert.Equal(t, "Bearer sk-test", r.Header.Get("Authorization"))

		// Generate 1536 floats (OpenAI text-embedding-3-small dimension)
		embedding := make([]float64, 1536)
		for i := range embedding {
			embedding[i] = float64(i) * 0.001
		}

		resp := map[string]any{
			"object": "list",
			"data": []map[string]any{
				{
					"object":    "embedding",
					"index":     0,
					"embedding": embedding,
				},
			},
			"model": "text-embedding-3-small",
			"usage": map[string]int{"prompt_tokens": 5, "total_tokens": 5},
		}
		json.NewEncoder(w).Encode(resp)
	}))
	defer server.Close()

	client := NewOpenAIClient(OpenAIConfig{
		APIKey:  "sk-test",
		Model:   "text-embedding-3-small",
		BaseURL: server.URL,
	})

	embedding, err := client.EmbedSingle(context.Background(), "test text")
	require.NoError(t, err)
	assert.Len(t, embedding, 1536)
}

func TestOpenAIClient_Embed_MultipleTexts(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		var req map[string]any
		json.NewDecoder(r.Body).Decode(&req)
		inputs := req["input"].([]any)

		data := make([]map[string]any, len(inputs))
		for i := range inputs {
			embedding := make([]float64, 1536)
			data[i] = map[string]any{
				"object":    "embedding",
				"index":     i,
				"embedding": embedding,
			}
		}

		resp := map[string]any{
			"object": "list",
			"data":   data,
			"model":  "text-embedding-3-small",
		}
		json.NewEncoder(w).Encode(resp)
	}))
	defer server.Close()

	client := NewOpenAIClient(OpenAIConfig{
		APIKey:  "sk-test",
		BaseURL: server.URL,
	})

	embeddings, err := client.Embed(context.Background(), []string{"text1", "text2", "text3"})
	require.NoError(t, err)
	assert.Len(t, embeddings, 3)
	for _, emb := range embeddings {
		assert.Len(t, emb, 1536)
	}
}

func TestOpenAIClient_Health_Success(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		// OpenAI doesn't have a health endpoint, we just do a minimal embedding request
		resp := map[string]any{
			"object": "list",
			"data": []map[string]any{
				{"embedding": make([]float64, 1536)},
			},
		}
		json.NewEncoder(w).Encode(resp)
	}))
	defer server.Close()

	client := NewOpenAIClient(OpenAIConfig{APIKey: "sk-test", BaseURL: server.URL})
	err := client.Health(context.Background())
	assert.NoError(t, err)
}

// === Failure Scenario Tests ===

func TestOpenAIClient_EmbedSingle_InvalidAPIKey(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusUnauthorized)
		json.NewEncoder(w).Encode(map[string]any{
			"error": map[string]any{
				"message": "Incorrect API key provided",
				"type":    "invalid_request_error",
				"code":    "invalid_api_key",
			},
		})
	}))
	defer server.Close()

	client := NewOpenAIClient(OpenAIConfig{APIKey: "invalid", BaseURL: server.URL})
	_, err := client.EmbedSingle(context.Background(), "test")

	require.Error(t, err)
	var embErr *EmbeddingError
	require.ErrorAs(t, err, &embErr)
	assert.Equal(t, "AUTH_FAILED", embErr.Code)
	assert.False(t, embErr.Retryable)
}

func TestOpenAIClient_EmbedSingle_RateLimited(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Retry-After", "2")
		w.WriteHeader(http.StatusTooManyRequests)
		json.NewEncoder(w).Encode(map[string]any{
			"error": map[string]any{
				"message": "Rate limit exceeded",
				"type":    "rate_limit_error",
			},
		})
	}))
	defer server.Close()

	client := NewOpenAIClient(OpenAIConfig{APIKey: "sk-test", BaseURL: server.URL})
	_, err := client.EmbedSingle(context.Background(), "test")

	require.Error(t, err)
	var embErr *EmbeddingError
	require.ErrorAs(t, err, &embErr)
	assert.Equal(t, "RATE_LIMITED", embErr.Code)
	assert.True(t, embErr.Retryable)
	assert.Equal(t, 2*time.Second, embErr.RetryAfter)
}

func TestOpenAIClient_EmbedSingle_QuotaExceeded(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusPaymentRequired)
		json.NewEncoder(w).Encode(map[string]any{
			"error": map[string]any{
				"message": "You exceeded your current quota",
				"type":    "insufficient_quota",
			},
		})
	}))
	defer server.Close()

	client := NewOpenAIClient(OpenAIConfig{APIKey: "sk-test", BaseURL: server.URL})
	_, err := client.EmbedSingle(context.Background(), "test")

	require.Error(t, err)
	var embErr *EmbeddingError
	require.ErrorAs(t, err, &embErr)
	assert.Equal(t, "QUOTA_EXCEEDED", embErr.Code)
	assert.False(t, embErr.Retryable)
}

// === Error Scenario Tests ===

func TestOpenAIClient_EmbedSingle_NetworkError(t *testing.T) {
	client := NewOpenAIClient(OpenAIConfig{
		APIKey:  "sk-test",
		BaseURL: "http://localhost:99999", // Invalid port
	})

	_, err := client.EmbedSingle(context.Background(), "test")
	require.Error(t, err)

	var embErr *EmbeddingError
	require.ErrorAs(t, err, &embErr)
	assert.True(t, embErr.Retryable)
}

func TestOpenAIClient_EmbedSingle_InvalidResponse(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Write([]byte("not json"))
	}))
	defer server.Close()

	client := NewOpenAIClient(OpenAIConfig{APIKey: "sk-test", BaseURL: server.URL})
	_, err := client.EmbedSingle(context.Background(), "test")
	require.Error(t, err)
}

func TestOpenAIClient_EmbedSingle_ContextCanceled(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		time.Sleep(5 * time.Second) // Slow response
	}))
	defer server.Close()

	client := NewOpenAIClient(OpenAIConfig{APIKey: "sk-test", BaseURL: server.URL})

	ctx, cancel := context.WithCancel(context.Background())
	cancel() // Cancel immediately

	_, err := client.EmbedSingle(ctx, "test")
	require.ErrorIs(t, err, context.Canceled)
}

func TestOpenAIClient_EmbedSingle_ServerError(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusInternalServerError)
		json.NewEncoder(w).Encode(map[string]any{
			"error": map[string]any{
				"message": "Internal server error",
			},
		})
	}))
	defer server.Close()

	client := NewOpenAIClient(OpenAIConfig{APIKey: "sk-test", BaseURL: server.URL})
	_, err := client.EmbedSingle(context.Background(), "test")

	require.Error(t, err)
	var embErr *EmbeddingError
	require.ErrorAs(t, err, &embErr)
	assert.True(t, embErr.Retryable) // 5xx errors are retryable
}

// === Edge Case Tests ===

func TestOpenAIClient_Embed_EmptyInput(t *testing.T) {
	client := NewOpenAIClient(OpenAIConfig{APIKey: "sk-test"})

	embeddings, err := client.Embed(context.Background(), []string{})
	require.NoError(t, err)
	assert.Empty(t, embeddings)
}

func TestOpenAIClient_EmbedSingle_EmptyText(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		resp := map[string]any{
			"data": []map[string]any{
				{"embedding": make([]float64, 1536)},
			},
		}
		json.NewEncoder(w).Encode(resp)
	}))
	defer server.Close()

	client := NewOpenAIClient(OpenAIConfig{APIKey: "sk-test", BaseURL: server.URL})
	embedding, err := client.EmbedSingle(context.Background(), "")
	require.NoError(t, err) // OpenAI accepts empty strings
	assert.Len(t, embedding, 1536)
}

func TestOpenAIClient_ModelName(t *testing.T) {
	client := NewOpenAIClient(OpenAIConfig{Model: "text-embedding-3-small"})
	assert.Equal(t, "text-embedding-3-small", client.ModelName())
}

func TestOpenAIClient_Dimensions(t *testing.T) {
	client := NewOpenAIClient(OpenAIConfig{})
	assert.Equal(t, 1536, client.Dimensions())
}

func TestOpenAIClient_DefaultConfig(t *testing.T) {
	client := NewOpenAIClient(OpenAIConfig{APIKey: "sk-test"})
	assert.Equal(t, "text-embedding-3-small", client.ModelName())
	assert.Equal(t, "https://api.openai.com", client.baseURL)
}

func TestOpenAIClient_ImplementsEmbedder(t *testing.T) {
	var _ Embedder = (*OpenAIClient)(nil)
}

func TestOpenAIClient_Embed_OutOfOrderResponse(t *testing.T) {
	// OpenAI may return embeddings out of order
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		resp := map[string]any{
			"data": []map[string]any{
				{"index": 2, "embedding": make([]float64, 1536)},
				{"index": 0, "embedding": make([]float64, 1536)},
				{"index": 1, "embedding": make([]float64, 1536)},
			},
		}
		json.NewEncoder(w).Encode(resp)
	}))
	defer server.Close()

	client := NewOpenAIClient(OpenAIConfig{APIKey: "sk-test", BaseURL: server.URL})
	embeddings, err := client.Embed(context.Background(), []string{"a", "b", "c"})
	require.NoError(t, err)
	assert.Len(t, embeddings, 3)
	// All embeddings should be in correct order
}

func TestOpenAIClient_EmbedSingle_EmptyData(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		resp := map[string]any{
			"data": []map[string]any{},
		}
		json.NewEncoder(w).Encode(resp)
	}))
	defer server.Close()

	client := NewOpenAIClient(OpenAIConfig{APIKey: "sk-test", BaseURL: server.URL})
	_, err := client.EmbedSingle(context.Background(), "test")
	require.Error(t, err) // Should error on empty data
}

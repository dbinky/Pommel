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

func TestVoyageClient_EmbedSingle_Success(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		assert.Equal(t, "POST", r.Method)
		assert.Equal(t, "/v1/embeddings", r.URL.Path)
		assert.Equal(t, "application/json", r.Header.Get("Content-Type"))
		assert.Equal(t, "Bearer pa-test", r.Header.Get("Authorization"))

		// Generate 1024 floats (voyage-code-3 dimension)
		embedding := make([]float64, 1024)
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
			"model": "voyage-code-3",
			"usage": map[string]int{"total_tokens": 5},
		}
		json.NewEncoder(w).Encode(resp)
	}))
	defer server.Close()

	client := NewVoyageClient(VoyageConfig{
		APIKey:  "pa-test",
		Model:   "voyage-code-3",
		BaseURL: server.URL,
	})

	embedding, err := client.EmbedSingle(context.Background(), "test text")
	require.NoError(t, err)
	assert.Len(t, embedding, 1024)
}

func TestVoyageClient_Embed_MultipleTexts(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		var req map[string]any
		json.NewDecoder(r.Body).Decode(&req)
		inputs := req["input"].([]any)

		data := make([]map[string]any, len(inputs))
		for i := range inputs {
			data[i] = map[string]any{
				"object":    "embedding",
				"index":     i,
				"embedding": make([]float64, 1024),
			}
		}

		resp := map[string]any{
			"object": "list",
			"data":   data,
			"model":  "voyage-code-3",
		}
		json.NewEncoder(w).Encode(resp)
	}))
	defer server.Close()

	client := NewVoyageClient(VoyageConfig{
		APIKey:  "pa-test",
		BaseURL: server.URL,
	})

	embeddings, err := client.Embed(context.Background(), []string{"text1", "text2", "text3"})
	require.NoError(t, err)
	assert.Len(t, embeddings, 3)
	for _, emb := range embeddings {
		assert.Len(t, emb, 1024)
	}
}

func TestVoyageClient_Embed_WithInputType(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		var req map[string]any
		json.NewDecoder(r.Body).Decode(&req)

		// Voyage uses input_type for query vs document
		assert.Equal(t, "document", req["input_type"])

		resp := map[string]any{
			"data": []map[string]any{
				{"index": 0, "embedding": make([]float64, 1024)},
			},
		}
		json.NewEncoder(w).Encode(resp)
	}))
	defer server.Close()

	client := NewVoyageClient(VoyageConfig{APIKey: "pa-test", BaseURL: server.URL})
	_, err := client.EmbedSingle(context.Background(), "test")
	require.NoError(t, err)
}

func TestVoyageClient_Health_Success(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		resp := map[string]any{
			"data": []map[string]any{
				{"embedding": make([]float64, 1024)},
			},
		}
		json.NewEncoder(w).Encode(resp)
	}))
	defer server.Close()

	client := NewVoyageClient(VoyageConfig{APIKey: "pa-test", BaseURL: server.URL})
	err := client.Health(context.Background())
	assert.NoError(t, err)
}

// === Failure Scenario Tests ===

func TestVoyageClient_EmbedSingle_InvalidAPIKey(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusUnauthorized)
		json.NewEncoder(w).Encode(map[string]any{
			"detail": "Invalid API key",
		})
	}))
	defer server.Close()

	client := NewVoyageClient(VoyageConfig{APIKey: "invalid", BaseURL: server.URL})
	_, err := client.EmbedSingle(context.Background(), "test")

	require.Error(t, err)
	var embErr *EmbeddingError
	require.ErrorAs(t, err, &embErr)
	assert.Equal(t, "AUTH_FAILED", embErr.Code)
	assert.False(t, embErr.Retryable)
}

func TestVoyageClient_EmbedSingle_RateLimited(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Retry-After", "5")
		w.WriteHeader(http.StatusTooManyRequests)
		json.NewEncoder(w).Encode(map[string]any{
			"detail": "Rate limit exceeded",
		})
	}))
	defer server.Close()

	client := NewVoyageClient(VoyageConfig{APIKey: "pa-test", BaseURL: server.URL})
	_, err := client.EmbedSingle(context.Background(), "test")

	require.Error(t, err)
	var embErr *EmbeddingError
	require.ErrorAs(t, err, &embErr)
	assert.Equal(t, "RATE_LIMITED", embErr.Code)
	assert.True(t, embErr.Retryable)
	assert.Equal(t, 5*time.Second, embErr.RetryAfter)
}

func TestVoyageClient_EmbedSingle_QuotaExceeded(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusPaymentRequired)
		json.NewEncoder(w).Encode(map[string]any{
			"detail": "Quota exceeded",
		})
	}))
	defer server.Close()

	client := NewVoyageClient(VoyageConfig{APIKey: "pa-test", BaseURL: server.URL})
	_, err := client.EmbedSingle(context.Background(), "test")

	require.Error(t, err)
	var embErr *EmbeddingError
	require.ErrorAs(t, err, &embErr)
	assert.Equal(t, "QUOTA_EXCEEDED", embErr.Code)
	assert.False(t, embErr.Retryable)
}

// === Error Scenario Tests ===

func TestVoyageClient_EmbedSingle_NetworkError(t *testing.T) {
	client := NewVoyageClient(VoyageConfig{
		APIKey:  "pa-test",
		BaseURL: "http://localhost:99999",
	})

	_, err := client.EmbedSingle(context.Background(), "test")
	require.Error(t, err)

	var embErr *EmbeddingError
	require.ErrorAs(t, err, &embErr)
	assert.True(t, embErr.Retryable)
}

func TestVoyageClient_EmbedSingle_InvalidResponse(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Write([]byte("not json"))
	}))
	defer server.Close()

	client := NewVoyageClient(VoyageConfig{APIKey: "pa-test", BaseURL: server.URL})
	_, err := client.EmbedSingle(context.Background(), "test")
	require.Error(t, err)
}

func TestVoyageClient_EmbedSingle_ContextCanceled(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		time.Sleep(5 * time.Second)
	}))
	defer server.Close()

	client := NewVoyageClient(VoyageConfig{APIKey: "pa-test", BaseURL: server.URL})

	ctx, cancel := context.WithCancel(context.Background())
	cancel()

	_, err := client.EmbedSingle(ctx, "test")
	require.ErrorIs(t, err, context.Canceled)
}

func TestVoyageClient_EmbedSingle_ServerError(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusInternalServerError)
		json.NewEncoder(w).Encode(map[string]any{
			"detail": "Internal server error",
		})
	}))
	defer server.Close()

	client := NewVoyageClient(VoyageConfig{APIKey: "pa-test", BaseURL: server.URL})
	_, err := client.EmbedSingle(context.Background(), "test")

	require.Error(t, err)
	var embErr *EmbeddingError
	require.ErrorAs(t, err, &embErr)
	assert.True(t, embErr.Retryable)
}

// === Edge Case Tests ===

func TestVoyageClient_Embed_EmptyInput(t *testing.T) {
	client := NewVoyageClient(VoyageConfig{APIKey: "pa-test"})

	embeddings, err := client.Embed(context.Background(), []string{})
	require.NoError(t, err)
	assert.Empty(t, embeddings)
}

func TestVoyageClient_ModelName(t *testing.T) {
	client := NewVoyageClient(VoyageConfig{Model: "voyage-code-3"})
	assert.Equal(t, "voyage-code-3", client.ModelName())
}

func TestVoyageClient_Dimensions(t *testing.T) {
	client := NewVoyageClient(VoyageConfig{})
	assert.Equal(t, 1024, client.Dimensions())
}

func TestVoyageClient_DefaultConfig(t *testing.T) {
	client := NewVoyageClient(VoyageConfig{APIKey: "pa-test"})
	assert.Equal(t, "voyage-code-3", client.ModelName())
	assert.Equal(t, "https://api.voyageai.com", client.baseURL)
}

func TestVoyageClient_ImplementsEmbedder(t *testing.T) {
	var _ Embedder = (*VoyageClient)(nil)
}

func TestVoyageClient_EmbedSingle_EmptyData(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		resp := map[string]any{
			"data": []map[string]any{},
		}
		json.NewEncoder(w).Encode(resp)
	}))
	defer server.Close()

	client := NewVoyageClient(VoyageConfig{APIKey: "pa-test", BaseURL: server.URL})
	_, err := client.EmbedSingle(context.Background(), "test")
	require.Error(t, err)
}

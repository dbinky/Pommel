package embedder

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"net/http"
	"net/http/httptest"
	"sync/atomic"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// ============================================================================
// Test Helpers
// ============================================================================

// mockOllamaResponse represents the response from Ollama's /api/embed endpoint.
type mockOllamaResponse struct {
	Embeddings [][]float32 `json:"embeddings"`
}

// mockOllamaRequest represents the request to Ollama's /api/embed endpoint.
type mockOllamaRequest struct {
	Model   string                 `json:"model"`
	Input   any                    `json:"input"`
	Options map[string]interface{} `json:"options,omitempty"`
}

// generate768DimEmbedding creates a mock 768-dimensional embedding.
func generate768DimEmbedding() []float32 {
	embedding := make([]float32, 768)
	for i := range embedding {
		embedding[i] = float32(i) * 0.001
	}
	return embedding
}

// createMockOllamaServer creates a mock Ollama server with configurable behavior.
func createMockOllamaServer(handler http.HandlerFunc) *httptest.Server {
	return httptest.NewServer(handler)
}

// ============================================================================
// Happy Path / Success Cases
// ============================================================================

func TestOllamaClient_Health(t *testing.T) {
	// Create mock server that returns 200 OK
	server := createMockOllamaServer(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path == "/" || r.URL.Path == "/api/version" {
			w.WriteHeader(http.StatusOK)
			w.Write([]byte(`{"version": "0.5.0"}`))
			return
		}
		w.WriteHeader(http.StatusNotFound)
	})
	defer server.Close()

	client := NewOllamaClient(OllamaConfig{
		BaseURL: server.URL,
		Model:   "test-model",
		Timeout: 5 * time.Second,
	})
	require.NotNil(t, client, "NewOllamaClient should return non-nil client")

	err := client.Health(context.Background())
	assert.NoError(t, err, "Health check should succeed when Ollama responds with 200")
}

func TestOllamaClient_EmbedSingle(t *testing.T) {
	expectedEmbedding := generate768DimEmbedding()

	server := createMockOllamaServer(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path != "/api/embed" {
			w.WriteHeader(http.StatusNotFound)
			return
		}

		// Verify request method
		assert.Equal(t, http.MethodPost, r.Method)

		// Decode and verify request
		var req mockOllamaRequest
		err := json.NewDecoder(r.Body).Decode(&req)
		require.NoError(t, err)
		assert.Equal(t, "test-model", req.Model)
		assert.NotEmpty(t, req.Input)

		// Return mock embedding
		resp := mockOllamaResponse{
			Embeddings: [][]float32{expectedEmbedding},
		}
		w.Header().Set("Content-Type", "application/json")
		json.NewEncoder(w).Encode(resp)
	})
	defer server.Close()

	client := NewOllamaClient(OllamaConfig{
		BaseURL: server.URL,
		Model:   "test-model",
		Timeout: 5 * time.Second,
	})
	require.NotNil(t, client, "NewOllamaClient should return non-nil client")

	embedding, err := client.EmbedSingle(context.Background(), "func main() { fmt.Println(\"hello\") }")

	require.NoError(t, err, "EmbedSingle should succeed")
	require.NotNil(t, embedding, "Embedding should not be nil")
	assert.Len(t, embedding, 768, "Embedding should have 768 dimensions")
	assert.Equal(t, expectedEmbedding, embedding, "Embedding should match expected values")
}

func TestOllamaClient_Embed_Multiple(t *testing.T) {
	texts := []string{
		"func hello() {}",
		"func world() {}",
		"func test() {}",
	}

	expectedEmbeddings := make([][]float32, len(texts))
	for i := range texts {
		expectedEmbeddings[i] = generate768DimEmbedding()
		// Vary the first element to make each embedding unique
		expectedEmbeddings[i][0] = float32(i) * 0.1
	}

	server := createMockOllamaServer(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path != "/api/embed" {
			w.WriteHeader(http.StatusNotFound)
			return
		}

		var req mockOllamaRequest
		err := json.NewDecoder(r.Body).Decode(&req)
		require.NoError(t, err)

		// Return embeddings for all inputs
		resp := mockOllamaResponse{
			Embeddings: expectedEmbeddings,
		}
		w.Header().Set("Content-Type", "application/json")
		json.NewEncoder(w).Encode(resp)
	})
	defer server.Close()

	client := NewOllamaClient(OllamaConfig{
		BaseURL: server.URL,
		Model:   "test-model",
		Timeout: 5 * time.Second,
	})
	require.NotNil(t, client, "NewOllamaClient should return non-nil client")

	embeddings, err := client.Embed(context.Background(), texts)

	require.NoError(t, err, "Embed should succeed")
	require.NotNil(t, embeddings, "Embeddings should not be nil")
	assert.Len(t, embeddings, len(texts), "Should return embedding for each input text")

	for i, emb := range embeddings {
		assert.Len(t, emb, 768, "Each embedding should have 768 dimensions")
		assert.Equal(t, expectedEmbeddings[i][0], emb[0], "Embedding %d should match expected", i)
	}
}

func TestOllamaClient_EmbedBatch_Concurrent(t *testing.T) {
	// Track concurrent requests
	var maxConcurrent int32
	var currentConcurrent int32

	texts := make([]string, 10)
	for i := range texts {
		texts[i] = fmt.Sprintf("func test%d() {}", i)
	}

	server := createMockOllamaServer(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path != "/api/embed" {
			w.WriteHeader(http.StatusNotFound)
			return
		}

		// Track concurrency
		current := atomic.AddInt32(&currentConcurrent, 1)
		for {
			max := atomic.LoadInt32(&maxConcurrent)
			if current <= max {
				break
			}
			if atomic.CompareAndSwapInt32(&maxConcurrent, max, current) {
				break
			}
		}

		// Simulate processing time to allow concurrent requests
		time.Sleep(10 * time.Millisecond)

		atomic.AddInt32(&currentConcurrent, -1)

		var req mockOllamaRequest
		json.NewDecoder(r.Body).Decode(&req)

		resp := mockOllamaResponse{
			Embeddings: [][]float32{generate768DimEmbedding()},
		}
		w.Header().Set("Content-Type", "application/json")
		json.NewEncoder(w).Encode(resp)
	})
	defer server.Close()

	client := NewOllamaClient(OllamaConfig{
		BaseURL: server.URL,
		Model:   "test-model",
		Timeout: 30 * time.Second,
	})
	require.NotNil(t, client, "NewOllamaClient should return non-nil client")

	concurrency := 3
	embeddings, err := client.EmbedBatch(context.Background(), texts, concurrency)

	require.NoError(t, err, "EmbedBatch should succeed")
	require.NotNil(t, embeddings, "Embeddings should not be nil")
	assert.Len(t, embeddings, len(texts), "Should return embedding for each input text")

	// Verify concurrent execution occurred
	assert.GreaterOrEqual(t, atomic.LoadInt32(&maxConcurrent), int32(2),
		"Should have made concurrent requests (max concurrent: %d)", maxConcurrent)
}

func TestOllamaClient_Dimensions(t *testing.T) {
	client := NewOllamaClient(OllamaConfig{
		BaseURL: "http://localhost:11434",
		Model:   "test-model",
		Timeout: 5 * time.Second,
	})
	require.NotNil(t, client, "NewOllamaClient should return non-nil client")

	dims := client.Dimensions()
	assert.Equal(t, 768, dims, "Dimensions should return 768 for Jina Code embeddings")
}

func TestOllamaClient_Dimensions_V2Model(t *testing.T) {
	cfg := OllamaConfig{
		Model: "unclemusclez/jina-embeddings-v2-base-code",
	}
	client := NewOllamaClient(cfg)
	assert.Equal(t, 768, client.Dimensions())
}

func TestOllamaClient_Dimensions_V4Model(t *testing.T) {
	cfg := OllamaConfig{
		Model: "sellerscrisp/jina-embeddings-v4-text-code-q4",
	}
	client := NewOllamaClient(cfg)
	assert.Equal(t, 1024, client.Dimensions())
}

func TestOllamaClient_Dimensions_UnknownModel(t *testing.T) {
	cfg := OllamaConfig{
		Model: "some-random-model",
	}
	client := NewOllamaClient(cfg)
	assert.Equal(t, 768, client.Dimensions(), "unknown models default to 768")
}

func TestOllamaClient_ContextSize_V2Model(t *testing.T) {
	cfg := OllamaConfig{
		Model: "unclemusclez/jina-embeddings-v2-base-code",
	}
	client := NewOllamaClient(cfg)
	assert.Equal(t, 8192, client.ContextSize())
}

func TestOllamaClient_ContextSize_V4Model(t *testing.T) {
	cfg := OllamaConfig{
		Model: "sellerscrisp/jina-embeddings-v4-text-code-q4",
	}
	client := NewOllamaClient(cfg)
	assert.Equal(t, 32768, client.ContextSize())
}

func TestOllamaClient_ContextSize_UnknownModel(t *testing.T) {
	cfg := OllamaConfig{
		Model: "some-random-model",
	}
	client := NewOllamaClient(cfg)
	assert.Equal(t, 8192, client.ContextSize(), "unknown models default to 8192")
}

func TestOllamaClient_ModelName(t *testing.T) {
	modelName := "unclemusclez/jina-embeddings-v2-base-code"

	client := NewOllamaClient(OllamaConfig{
		BaseURL: "http://localhost:11434",
		Model:   modelName,
		Timeout: 5 * time.Second,
	})
	require.NotNil(t, client, "NewOllamaClient should return non-nil client")

	result := client.ModelName()
	assert.Equal(t, modelName, result, "ModelName should return the configured model name")
}

func TestOllamaClient_DefaultsApplied(t *testing.T) {
	// Create client with empty/zero config values
	client := NewOllamaClient(OllamaConfig{})

	require.NotNil(t, client, "Client should not be nil even with empty config")

	// Verify defaults are applied
	assert.Equal(t, "unclemusclez/jina-embeddings-v2-base-code", client.ModelName(),
		"Default model should be applied")
	assert.Equal(t, 768, client.Dimensions(),
		"Default dimensions should be 768")
}

// ============================================================================
// Failure / Error Cases
// ============================================================================

func TestOllamaClient_Health_NotRunning(t *testing.T) {
	// Use a port that should not have anything listening
	client := NewOllamaClient(OllamaConfig{
		BaseURL: "http://localhost:59999",
		Model:   "test-model",
		Timeout: 1 * time.Second,
	})
	require.NotNil(t, client, "NewOllamaClient should return non-nil client")

	err := client.Health(context.Background())
	assert.Error(t, err, "Health should return error when Ollama is not reachable")
	// Check that the error is an OllamaError with helpful message
	var ollamaErr *OllamaError
	if errors.As(err, &ollamaErr) {
		assert.Equal(t, "OLLAMA_UNAVAILABLE", ollamaErr.Code, "Expected OLLAMA_UNAVAILABLE error code")
		assert.Contains(t, ollamaErr.Message, "cannot connect", "Error should indicate connection failed")
		assert.Contains(t, ollamaErr.Suggestion, "Is Ollama running?", "Error should suggest checking if Ollama is running")
	} else {
		t.Errorf("Expected OllamaError, got %T: %v", err, err)
	}
}

func TestOllamaClient_Health_ServerError(t *testing.T) {
	server := createMockOllamaServer(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusInternalServerError)
		w.Write([]byte(`{"error": "internal server error"}`))
	})
	defer server.Close()

	client := NewOllamaClient(OllamaConfig{
		BaseURL: server.URL,
		Model:   "test-model",
		Timeout: 5 * time.Second,
	})
	require.NotNil(t, client, "NewOllamaClient should return non-nil client")

	err := client.Health(context.Background())
	assert.Error(t, err, "Health should return error on 500 response")
	assert.Contains(t, err.Error(), "500",
		"Error should include status code")
}

func TestOllamaClient_EmbedSingle_ModelNotFound(t *testing.T) {
	server := createMockOllamaServer(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path == "/api/embed" {
			w.WriteHeader(http.StatusNotFound)
			w.Write([]byte(`{"error": "model 'nonexistent-model' not found"}`))
			return
		}
		w.WriteHeader(http.StatusOK)
	})
	defer server.Close()

	client := NewOllamaClient(OllamaConfig{
		BaseURL: server.URL,
		Model:   "nonexistent-model",
		Timeout: 5 * time.Second,
	})
	require.NotNil(t, client, "NewOllamaClient should return non-nil client")

	embedding, err := client.EmbedSingle(context.Background(), "test code")

	assert.Error(t, err, "EmbedSingle should return error when model is not found")
	assert.Nil(t, embedding, "Embedding should be nil on error")
	assert.Contains(t, err.Error(), "not found",
		"Error should indicate model was not found")
}

func TestOllamaClient_EmbedSingle_InvalidJSON(t *testing.T) {
	server := createMockOllamaServer(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path == "/api/embed" {
			w.Header().Set("Content-Type", "application/json")
			w.WriteHeader(http.StatusOK)
			// Return malformed JSON
			w.Write([]byte(`{"embeddings": [not valid json`))
			return
		}
		w.WriteHeader(http.StatusOK)
	})
	defer server.Close()

	client := NewOllamaClient(OllamaConfig{
		BaseURL: server.URL,
		Model:   "test-model",
		Timeout: 5 * time.Second,
	})
	require.NotNil(t, client, "NewOllamaClient should return non-nil client")

	embedding, err := client.EmbedSingle(context.Background(), "test code")

	assert.Error(t, err, "EmbedSingle should return error on malformed JSON response")
	assert.Nil(t, embedding, "Embedding should be nil on error")
}

func TestOllamaClient_EmbedSingle_ContextCancelled(t *testing.T) {
	server := createMockOllamaServer(func(w http.ResponseWriter, r *http.Request) {
		// Simulate slow response
		time.Sleep(5 * time.Second)
		resp := mockOllamaResponse{
			Embeddings: [][]float32{generate768DimEmbedding()},
		}
		json.NewEncoder(w).Encode(resp)
	})
	defer server.Close()

	client := NewOllamaClient(OllamaConfig{
		BaseURL: server.URL,
		Model:   "test-model",
		Timeout: 30 * time.Second,
	})
	require.NotNil(t, client, "NewOllamaClient should return non-nil client")

	ctx, cancel := context.WithCancel(context.Background())

	// Cancel context after a short delay
	go func() {
		time.Sleep(50 * time.Millisecond)
		cancel()
	}()

	embedding, err := client.EmbedSingle(ctx, "test code")

	assert.Error(t, err, "EmbedSingle should return error when context is cancelled")
	assert.Nil(t, embedding, "Embedding should be nil on cancellation")
	assert.ErrorIs(t, err, context.Canceled,
		"Error should be context.Canceled")
}

func TestOllamaClient_EmbedBatch_PartialFailure(t *testing.T) {
	var requestCount int32

	server := createMockOllamaServer(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path != "/api/embed" {
			w.WriteHeader(http.StatusNotFound)
			return
		}

		count := atomic.AddInt32(&requestCount, 1)

		// Fail every other request
		if count%2 == 0 {
			w.WriteHeader(http.StatusInternalServerError)
			w.Write([]byte(`{"error": "simulated failure"}`))
			return
		}

		resp := mockOllamaResponse{
			Embeddings: [][]float32{generate768DimEmbedding()},
		}
		w.Header().Set("Content-Type", "application/json")
		json.NewEncoder(w).Encode(resp)
	})
	defer server.Close()

	client := NewOllamaClient(OllamaConfig{
		BaseURL: server.URL,
		Model:   "test-model",
		Timeout: 5 * time.Second,
	})
	require.NotNil(t, client, "NewOllamaClient should return non-nil client")

	texts := []string{"code1", "code2", "code3", "code4"}
	embeddings, err := client.EmbedBatch(context.Background(), texts, 2)

	// Batch with partial failure should return error
	assert.Error(t, err, "EmbedBatch should return error on partial failure")
	assert.Nil(t, embeddings, "Embeddings should be nil when any request fails")
}

func TestOllamaClient_EmbedBatch_EmptyInput(t *testing.T) {
	requestMade := false

	server := createMockOllamaServer(func(w http.ResponseWriter, r *http.Request) {
		requestMade = true
		w.WriteHeader(http.StatusOK)
	})
	defer server.Close()

	client := NewOllamaClient(OllamaConfig{
		BaseURL: server.URL,
		Model:   "test-model",
		Timeout: 5 * time.Second,
	})
	require.NotNil(t, client, "NewOllamaClient should return non-nil client")

	// Test with empty slice
	embeddings, err := client.EmbedBatch(context.Background(), []string{}, 2)

	assert.NoError(t, err, "EmbedBatch should not return error for empty input")
	assert.Empty(t, embeddings, "Embeddings should be empty for empty input")
	assert.False(t, requestMade, "No HTTP request should be made for empty input")

	// Test with nil slice
	embeddings, err = client.EmbedBatch(context.Background(), nil, 2)

	assert.NoError(t, err, "EmbedBatch should not return error for nil input")
	assert.Empty(t, embeddings, "Embeddings should be empty for nil input")
}

// ============================================================================
// Additional Edge Cases
// ============================================================================

func TestOllamaClient_EmbedSingle_EmptyText(t *testing.T) {
	server := createMockOllamaServer(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path != "/api/embed" {
			w.WriteHeader(http.StatusNotFound)
			return
		}

		var req mockOllamaRequest
		json.NewDecoder(r.Body).Decode(&req)

		// Ollama still returns an embedding for empty string
		resp := mockOllamaResponse{
			Embeddings: [][]float32{generate768DimEmbedding()},
		}
		w.Header().Set("Content-Type", "application/json")
		json.NewEncoder(w).Encode(resp)
	})
	defer server.Close()

	client := NewOllamaClient(OllamaConfig{
		BaseURL: server.URL,
		Model:   "test-model",
		Timeout: 5 * time.Second,
	})
	require.NotNil(t, client, "NewOllamaClient should return non-nil client")

	embedding, err := client.EmbedSingle(context.Background(), "")

	require.NoError(t, err, "EmbedSingle should handle empty string")
	assert.Len(t, embedding, 768, "Should return 768-dim embedding for empty string")
}

func TestOllamaClient_Embed_SingleText(t *testing.T) {
	expectedEmbedding := generate768DimEmbedding()

	server := createMockOllamaServer(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path != "/api/embed" {
			w.WriteHeader(http.StatusNotFound)
			return
		}

		resp := mockOllamaResponse{
			Embeddings: [][]float32{expectedEmbedding},
		}
		w.Header().Set("Content-Type", "application/json")
		json.NewEncoder(w).Encode(resp)
	})
	defer server.Close()

	client := NewOllamaClient(OllamaConfig{
		BaseURL: server.URL,
		Model:   "test-model",
		Timeout: 5 * time.Second,
	})
	require.NotNil(t, client, "NewOllamaClient should return non-nil client")

	// Embed with single text should work same as EmbedSingle
	embeddings, err := client.Embed(context.Background(), []string{"single text"})

	require.NoError(t, err, "Embed should succeed with single text")
	require.Len(t, embeddings, 1, "Should return one embedding")
	assert.Len(t, embeddings[0], 768, "Embedding should have 768 dimensions")
}

func TestOllamaClient_Timeout(t *testing.T) {
	server := createMockOllamaServer(func(w http.ResponseWriter, r *http.Request) {
		// Simulate slow response that exceeds timeout
		time.Sleep(2 * time.Second)
		resp := mockOllamaResponse{
			Embeddings: [][]float32{generate768DimEmbedding()},
		}
		json.NewEncoder(w).Encode(resp)
	})
	defer server.Close()

	client := NewOllamaClient(OllamaConfig{
		BaseURL: server.URL,
		Model:   "test-model",
		Timeout: 100 * time.Millisecond, // Very short timeout
	})
	require.NotNil(t, client, "NewOllamaClient should return non-nil client")

	embedding, err := client.EmbedSingle(context.Background(), "test code")

	assert.Error(t, err, "EmbedSingle should return error when request times out")
	assert.Nil(t, embedding, "Embedding should be nil on timeout")
}

func TestDefaultOllamaConfig(t *testing.T) {
	cfg := DefaultOllamaConfig()

	assert.Equal(t, "http://localhost:11434", cfg.BaseURL,
		"Default BaseURL should be localhost:11434")
	assert.Equal(t, "unclemusclez/jina-embeddings-v2-base-code", cfg.Model,
		"Default Model should be Jina Code embeddings")
	assert.Equal(t, 30*time.Second, cfg.Timeout,
		"Default Timeout should be 30 seconds")
}

func TestOllamaClient_SetsNumCtxOption(t *testing.T) {
	var receivedOptions map[string]interface{}

	server := createMockOllamaServer(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path != "/api/embed" {
			w.WriteHeader(http.StatusNotFound)
			return
		}

		var req mockOllamaRequest
		err := json.NewDecoder(r.Body).Decode(&req)
		require.NoError(t, err)

		// Capture the options for verification
		receivedOptions = req.Options

		resp := mockOllamaResponse{
			Embeddings: [][]float32{generate768DimEmbedding()},
		}
		w.Header().Set("Content-Type", "application/json")
		json.NewEncoder(w).Encode(resp)
	})
	defer server.Close()

	client := NewOllamaClient(OllamaConfig{
		BaseURL: server.URL,
		Model:   "test-model",
		Timeout: 5 * time.Second,
	})

	_, err := client.EmbedSingle(context.Background(), "test code")
	require.NoError(t, err)

	// Verify num_ctx was sent
	require.NotNil(t, receivedOptions, "Options should be sent in request")
	numCtx, ok := receivedOptions["num_ctx"]
	require.True(t, ok, "num_ctx should be present in options")

	// JSON numbers decode as float64
	numCtxFloat, ok := numCtx.(float64)
	require.True(t, ok, "num_ctx should be a number")
	// Unknown model defaults to 8192
	assert.Equal(t, float64(8192), numCtxFloat, "num_ctx should use model's context size (8192 default)")
}

func TestOllamaClient_SetsNumCtxOption_V4Model(t *testing.T) {
	var receivedOptions map[string]interface{}

	server := createMockOllamaServer(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path != "/api/embed" {
			w.WriteHeader(http.StatusNotFound)
			return
		}

		var req mockOllamaRequest
		err := json.NewDecoder(r.Body).Decode(&req)
		require.NoError(t, err)

		// Capture the options for verification
		receivedOptions = req.Options

		resp := mockOllamaResponse{
			Embeddings: [][]float32{generate768DimEmbedding()},
		}
		w.Header().Set("Content-Type", "application/json")
		json.NewEncoder(w).Encode(resp)
	})
	defer server.Close()

	client := NewOllamaClient(OllamaConfig{
		BaseURL: server.URL,
		Model:   "sellerscrisp/jina-embeddings-v4-text-code-q4",
		Timeout: 5 * time.Second,
	})

	_, err := client.EmbedSingle(context.Background(), "test code")
	require.NoError(t, err)

	// Verify num_ctx was sent with v4 context size
	require.NotNil(t, receivedOptions, "Options should be sent in request")
	numCtx, ok := receivedOptions["num_ctx"]
	require.True(t, ok, "num_ctx should be present in options")

	numCtxFloat, ok := numCtx.(float64)
	require.True(t, ok, "num_ctx should be a number")
	assert.Equal(t, float64(32768), numCtxFloat, "num_ctx should be 32768 for v4 model")
}

package embedder

import (
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestGetModelInfo_V2(t *testing.T) {
	info, err := GetModelInfo("v2")
	require.NoError(t, err)
	assert.Equal(t, "unclemusclez/jina-embeddings-v2-base-code", info.Name)
	assert.Equal(t, 768, info.Dimensions)
	assert.Equal(t, 8192, info.ContextSize)
}

func TestGetModelInfo_V4(t *testing.T) {
	info, err := GetModelInfo("v4")
	require.NoError(t, err)
	assert.Equal(t, "sellerscrisp/jina-embeddings-v4-text-code-q4", info.Name)
	assert.Equal(t, 1024, info.Dimensions)
	assert.Equal(t, 32768, info.ContextSize)
}

func TestGetModelInfo_UnknownShortName(t *testing.T) {
	info, err := GetModelInfo("v5")
	assert.Nil(t, info)
	require.Error(t, err)
	assert.Contains(t, err.Error(), "unknown model 'v5'")
}

func TestGetModelInfo_EmptyName(t *testing.T) {
	info, err := GetModelInfo("")
	assert.Nil(t, info)
	require.Error(t, err)
	assert.Contains(t, err.Error(), "cannot be empty")
}

func TestGetModelInfo_CaseInsensitive(t *testing.T) {
	// Test uppercase "V4" works the same as lowercase "v4"
	info, err := GetModelInfo("V4")
	require.NoError(t, err)
	assert.Equal(t, "sellerscrisp/jina-embeddings-v4-text-code-q4", info.Name)
	assert.Equal(t, 1024, info.Dimensions)
	assert.Equal(t, 32768, info.ContextSize)
}

func TestGetModelByFullName_V2(t *testing.T) {
	info := GetModelByFullName("unclemusclez/jina-embeddings-v2-base-code")
	require.NotNil(t, info)
	assert.Equal(t, 768, info.Dimensions)
}

func TestGetModelByFullName_V4(t *testing.T) {
	info := GetModelByFullName("sellerscrisp/jina-embeddings-v4-text-code-q4")
	require.NotNil(t, info)
	assert.Equal(t, 1024, info.Dimensions)
}

func TestGetModelByFullName_Unknown(t *testing.T) {
	info := GetModelByFullName("some-random-model")
	assert.Nil(t, info)
}

func TestGetDimensionsForModel_V2(t *testing.T) {
	dims := GetDimensionsForModel("unclemusclez/jina-embeddings-v2-base-code")
	assert.Equal(t, 768, dims)
}

func TestGetDimensionsForModel_V4(t *testing.T) {
	dims := GetDimensionsForModel("sellerscrisp/jina-embeddings-v4-text-code-q4")
	assert.Equal(t, 1024, dims)
}

func TestGetDimensionsForModel_Unknown_DefaultsTo768(t *testing.T) {
	dims := GetDimensionsForModel("some-unknown-model")
	assert.Equal(t, 768, dims, "unknown models should default to 768")
}

func TestGetDimensionsForModel_Empty_DefaultsTo768(t *testing.T) {
	dims := GetDimensionsForModel("")
	assert.Equal(t, 768, dims, "empty model should default to 768")
}

func TestGetShortNameForModel_V2(t *testing.T) {
	shortName := GetShortNameForModel("unclemusclez/jina-embeddings-v2-base-code")
	assert.Equal(t, "v2", shortName)
}

func TestGetShortNameForModel_V4(t *testing.T) {
	shortName := GetShortNameForModel("sellerscrisp/jina-embeddings-v4-text-code-q4")
	assert.Equal(t, "v4", shortName)
}

func TestGetShortNameForModel_Unknown(t *testing.T) {
	shortName := GetShortNameForModel("some-random-model")
	assert.Equal(t, "", shortName, "unknown models should return empty string")
}

func TestGetContextSizeForModel_V2(t *testing.T) {
	size := GetContextSizeForModel("unclemusclez/jina-embeddings-v2-base-code")
	assert.Equal(t, 8192, size)
}

func TestGetContextSizeForModel_V4(t *testing.T) {
	size := GetContextSizeForModel("sellerscrisp/jina-embeddings-v4-text-code-q4")
	assert.Equal(t, 32768, size)
}

func TestGetContextSizeForModel_Unknown(t *testing.T) {
	size := GetContextSizeForModel("unknown-model")
	assert.Equal(t, 8192, size, "unknown models should default to 8192")
}

func TestResolveDimensions_V2Model_Returns768(t *testing.T) {
	dims, err := ResolveDimensions("unclemusclez/jina-embeddings-v2-base-code", 0)
	require.NoError(t, err)
	assert.Equal(t, 768, dims)
}

func TestResolveDimensions_V4Model_Returns1024(t *testing.T) {
	dims, err := ResolveDimensions("sellerscrisp/jina-embeddings-v4-text-code-q4", 0)
	require.NoError(t, err)
	assert.Equal(t, 1024, dims)
}

func TestResolveDimensions_UnknownModel_WithDimensions_ReturnsConfigValue(t *testing.T) {
	dims, err := ResolveDimensions("qwen3-embedding:0.6b", 1024)
	require.NoError(t, err)
	assert.Equal(t, 1024, dims)
}

func TestResolveDimensions_V2Model_WithConfigOverride_ReturnsRegistryValue(t *testing.T) {
	// Registry should take precedence over config
	dims, err := ResolveDimensions("unclemusclez/jina-embeddings-v2-base-code", 1024)
	require.NoError(t, err)
	assert.Equal(t, 768, dims, "Registry value should override config")
}

func TestResolveDimensions_V4Model_WithConfigOverride_ReturnsRegistryValue(t *testing.T) {
	dims, err := ResolveDimensions("sellerscrisp/jina-embeddings-v4-text-code-q4", 768)
	require.NoError(t, err)
	assert.Equal(t, 1024, dims, "Registry value should override config")
}

func TestResolveDimensions_UnknownModel_Dimensions256_Succeeds(t *testing.T) {
	dims, err := ResolveDimensions("some-model-256", 256)
	require.NoError(t, err)
	assert.Equal(t, 256, dims)
}

func TestResolveDimensions_UnknownModel_Dimensions512_Succeeds(t *testing.T) {
	dims, err := ResolveDimensions("some-model-512", 512)
	require.NoError(t, err)
	assert.Equal(t, 512, dims)
}

func TestResolveDimensions_UnknownModel_Dimensions1536_Succeeds(t *testing.T) {
	dims, err := ResolveDimensions("some-model-1536", 1536)
	require.NoError(t, err)
	assert.Equal(t, 1536, dims)
}

func TestResolveDimensions_UnknownModel_Dimensions4096_Succeeds(t *testing.T) {
	dims, err := ResolveDimensions("some-model-4096", 4096)
	require.NoError(t, err)
	assert.Equal(t, 4096, dims)
}

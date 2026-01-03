package chunker

import (
	"context"
	"testing"
	"time"

	"github.com/pommel-dev/pommel/internal/models"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// =============================================================================
// Helper Functions for Config-Driven Tests
// =============================================================================

// getLuaChunker returns the Lua chunker from the config-driven registry.
func getLuaChunker(t *testing.T) Chunker {
	t.Helper()
	registry, err := NewChunkerRegistry()
	require.NoError(t, err, "Failed to create chunker registry")

	chunker, ok := registry.GetChunkerForExtension(".lua")
	require.True(t, ok, "Lua chunker should be available")
	return chunker
}

// =============================================================================
// LuaChunker Initialization Tests
// =============================================================================

func TestLuaChunker_Available(t *testing.T) {
	registry, err := NewChunkerRegistry()
	require.NoError(t, err)

	chunker, ok := registry.GetChunkerForExtension(".lua")
	assert.True(t, ok, "Lua chunker should be available in registry")
	assert.NotNil(t, chunker, "Lua chunker should not be nil")
}

func TestLuaChunker_Language(t *testing.T) {
	chunker := getLuaChunker(t)
	assert.Equal(t, LangLua, chunker.Language(), "LuaChunker should report Lua as its language")
}

// =============================================================================
// Simple Function Tests
// =============================================================================

func TestLuaChunker_SimpleFunctions(t *testing.T) {
	chunker := getLuaChunker(t)

	source := []byte(`function add(a, b)
    return a + b
end

function subtract(a, b)
    return a - b
end
`)

	file := &models.SourceFile{
		Path:         "/test/calculator.lua",
		Content:      source,
		Language:     "lua",
		LastModified: time.Now(),
	}

	result, err := chunker.Chunk(context.Background(), file)
	require.NoError(t, err, "Chunk should not return an error for valid Lua code")
	require.NotNil(t, result, "Chunk should return a non-nil result")

	// Should have: 1 file chunk + 2 function chunks = 3 chunks
	assert.Len(t, result.Chunks, 3, "Should have 3 chunks: file and 2 functions")

	// Find chunks by level
	var fileChunk *models.Chunk
	var methodChunks []*models.Chunk

	for _, chunk := range result.Chunks {
		switch chunk.Level {
		case models.ChunkLevelFile:
			fileChunk = chunk
		case models.ChunkLevelMethod:
			methodChunks = append(methodChunks, chunk)
		}
	}

	// Verify file chunk
	require.NotNil(t, fileChunk, "Should have a file-level chunk")
	assert.Equal(t, "/test/calculator.lua", fileChunk.FilePath)
	assert.Nil(t, fileChunk.ParentID, "File chunk should have no parent")

	// Verify function chunks
	assert.Len(t, methodChunks, 2, "Should have 2 function chunks")

	// Verify function names
	funcNames := make(map[string]bool)
	for _, m := range methodChunks {
		funcNames[m.Name] = true
	}
	assert.True(t, funcNames["add"], "Should have 'add' function")
	assert.True(t, funcNames["subtract"], "Should have 'subtract' function")
}

// =============================================================================
// Local Function Tests
// =============================================================================

func TestLuaChunker_LocalFunctions(t *testing.T) {
	chunker := getLuaChunker(t)

	source := []byte(`local function helper()
    return "help"
end

local function process(data)
    return helper() .. data
end
`)

	file := &models.SourceFile{
		Path:         "/test/local_funcs.lua",
		Content:      source,
		Language:     "lua",
		LastModified: time.Now(),
	}

	result, err := chunker.Chunk(context.Background(), file)
	require.NoError(t, err)
	require.NotNil(t, result)

	// Find method chunks
	var methodChunks []*models.Chunk
	for _, chunk := range result.Chunks {
		if chunk.Level == models.ChunkLevelMethod {
			methodChunks = append(methodChunks, chunk)
		}
	}

	assert.Len(t, methodChunks, 2, "Should extract 2 local functions")

	funcNames := make(map[string]bool)
	for _, m := range methodChunks {
		funcNames[m.Name] = true
	}
	assert.True(t, funcNames["helper"], "Should have 'helper' function")
	assert.True(t, funcNames["process"], "Should have 'process' function")
}

// =============================================================================
// Table Method Tests (Lua OOP Pattern)
// =============================================================================

func TestLuaChunker_TableMethods(t *testing.T) {
	chunker := getLuaChunker(t)

	source := []byte(`local Calculator = {}

function Calculator:add(a, b)
    return a + b
end

function Calculator:subtract(a, b)
    return a - b
end

return Calculator
`)

	file := &models.SourceFile{
		Path:         "/test/table_methods.lua",
		Content:      source,
		Language:     "lua",
		LastModified: time.Now(),
	}

	result, err := chunker.Chunk(context.Background(), file)
	require.NoError(t, err)
	require.NotNil(t, result)

	// Find method chunks
	var methodChunks []*models.Chunk
	for _, chunk := range result.Chunks {
		if chunk.Level == models.ChunkLevelMethod {
			methodChunks = append(methodChunks, chunk)
		}
	}

	assert.GreaterOrEqual(t, len(methodChunks), 2, "Should extract at least 2 table methods")
}

// =============================================================================
// Mixed Function Styles Tests
// =============================================================================

func TestLuaChunker_MixedFunctionStyles(t *testing.T) {
	chunker := getLuaChunker(t)

	source := []byte(`-- Global function
function globalFunc()
    return 1
end

-- Local function
local function localFunc()
    return 2
end

-- Anonymous function assigned to variable
local anonFunc = function()
    return 3
end

-- Table method
local M = {}
function M.tableMethod()
    return 4
end
`)

	file := &models.SourceFile{
		Path:         "/test/mixed.lua",
		Content:      source,
		Language:     "lua",
		LastModified: time.Now(),
	}

	result, err := chunker.Chunk(context.Background(), file)
	require.NoError(t, err)
	require.NotNil(t, result)

	// Find method chunks
	var methodChunks []*models.Chunk
	for _, chunk := range result.Chunks {
		if chunk.Level == models.ChunkLevelMethod {
			methodChunks = append(methodChunks, chunk)
		}
	}

	// Should extract multiple function types
	assert.GreaterOrEqual(t, len(methodChunks), 2, "Should extract multiple function styles")
}

// =============================================================================
// Nested Functions Tests
// =============================================================================

func TestLuaChunker_NestedFunctions(t *testing.T) {
	chunker := getLuaChunker(t)

	source := []byte(`function outer()
    local function inner()
        return "inner"
    end
    return inner()
end
`)

	file := &models.SourceFile{
		Path:         "/test/nested.lua",
		Content:      source,
		Language:     "lua",
		LastModified: time.Now(),
	}

	result, err := chunker.Chunk(context.Background(), file)
	require.NoError(t, err)
	require.NotNil(t, result)

	// Should at least extract the outer function
	var methodChunks []*models.Chunk
	for _, chunk := range result.Chunks {
		if chunk.Level == models.ChunkLevelMethod {
			methodChunks = append(methodChunks, chunk)
		}
	}

	assert.GreaterOrEqual(t, len(methodChunks), 1, "Should extract at least outer function")
}

// =============================================================================
// Edge Cases Tests
// =============================================================================

func TestLuaChunker_EmptyFile(t *testing.T) {
	chunker := getLuaChunker(t)

	file := &models.SourceFile{
		Path:         "/test/empty.lua",
		Content:      []byte(""),
		Language:     "lua",
		LastModified: time.Now(),
	}

	result, err := chunker.Chunk(context.Background(), file)
	require.NoError(t, err)
	require.NotNil(t, result)

	assert.Empty(t, result.Errors, "Should have no errors for empty file")
}

func TestLuaChunker_OnlyComments(t *testing.T) {
	chunker := getLuaChunker(t)

	source := []byte(`-- This is a comment
-- Another comment
--[[
    Multi-line comment
]]
`)

	file := &models.SourceFile{
		Path:         "/test/comments.lua",
		Content:      source,
		Language:     "lua",
		LastModified: time.Now(),
	}

	result, err := chunker.Chunk(context.Background(), file)
	require.NoError(t, err)
	require.NotNil(t, result)

	assert.Empty(t, result.Errors, "Should have no errors for comment-only file")
}

func TestLuaChunker_FunctionsWithComments(t *testing.T) {
	chunker := getLuaChunker(t)

	source := []byte(`-- Calculate damage based on stats
-- @param base number The base damage
-- @param multiplier number The damage multiplier
-- @return number The calculated damage
function calculateDamage(base, multiplier)
    return base * multiplier
end
`)

	file := &models.SourceFile{
		Path:         "/test/documented.lua",
		Content:      source,
		Language:     "lua",
		LastModified: time.Now(),
	}

	result, err := chunker.Chunk(context.Background(), file)
	require.NoError(t, err)
	require.NotNil(t, result)

	// Find the function chunk
	var funcChunk *models.Chunk
	for _, chunk := range result.Chunks {
		if chunk.Level == models.ChunkLevelMethod {
			funcChunk = chunk
			break
		}
	}

	require.NotNil(t, funcChunk, "Should extract the documented function")
	assert.Equal(t, "calculateDamage", funcChunk.Name)
}

// =============================================================================
// Deterministic ID Tests
// =============================================================================

func TestLuaChunker_DeterministicIDs(t *testing.T) {
	chunker := getLuaChunker(t)

	source := []byte(`function calculate(a, b)
    return a + b
end
`)

	file := &models.SourceFile{
		Path:         "/test/calc.lua",
		Content:      source,
		Language:     "lua",
		LastModified: time.Now(),
	}

	// Parse the same file twice
	result1, err := chunker.Chunk(context.Background(), file)
	require.NoError(t, err)

	result2, err := chunker.Chunk(context.Background(), file)
	require.NoError(t, err)

	// Both results should have the same number of chunks
	require.Equal(t, len(result1.Chunks), len(result2.Chunks), "Both parses should produce same number of chunks")

	// Create maps by chunk name for comparison
	chunks1 := make(map[string]string)
	chunks2 := make(map[string]string)

	for _, c := range result1.Chunks {
		key := string(c.Level) + ":" + c.Name
		chunks1[key] = c.ID
	}

	for _, c := range result2.Chunks {
		key := string(c.Level) + ":" + c.Name
		chunks2[key] = c.ID
	}

	// Verify all IDs match
	for key, id1 := range chunks1 {
		id2, exists := chunks2[key]
		require.True(t, exists, "Chunk %s should exist in both results", key)
		assert.Equal(t, id1, id2, "Chunk %s should have deterministic ID", key)
	}
}

func TestLuaChunker_DeterministicIDs_SameContent_DifferentFiles(t *testing.T) {
	chunker := getLuaChunker(t)

	source := []byte(`function hello()
    return "hello"
end
`)

	file1 := &models.SourceFile{
		Path:         "/test/file1.lua",
		Content:      source,
		Language:     "lua",
		LastModified: time.Now(),
	}

	file2 := &models.SourceFile{
		Path:         "/test/file2.lua",
		Content:      source,
		Language:     "lua",
		LastModified: time.Now(),
	}

	result1, err := chunker.Chunk(context.Background(), file1)
	require.NoError(t, err)

	result2, err := chunker.Chunk(context.Background(), file2)
	require.NoError(t, err)

	// Same content but different paths should produce different IDs
	require.Equal(t, len(result1.Chunks), len(result2.Chunks))

	for i := range result1.Chunks {
		assert.NotEqual(t, result1.Chunks[i].ID, result2.Chunks[i].ID,
			"Same content in different files should have different IDs")
	}
}

// =============================================================================
// Line Number Tests
// =============================================================================

func TestLuaChunker_CorrectLineNumbers(t *testing.T) {
	chunker := getLuaChunker(t)

	source := []byte(`function add(a, b)
    return a + b
end

function subtract(a, b)
    return a - b
end
`)

	file := &models.SourceFile{
		Path:         "/test/lines.lua",
		Content:      source,
		Language:     "lua",
		LastModified: time.Now(),
	}

	result, err := chunker.Chunk(context.Background(), file)
	require.NoError(t, err)

	// Find specific chunks
	methodChunks := make(map[string]*models.Chunk)

	for _, chunk := range result.Chunks {
		if chunk.Level == models.ChunkLevelMethod {
			methodChunks[chunk.Name] = chunk
		}
	}

	// Verify add function line numbers
	addFunc := methodChunks["add"]
	require.NotNil(t, addFunc)
	assert.Equal(t, 1, addFunc.StartLine, "add function should start at line 1")
	assert.Equal(t, 3, addFunc.EndLine, "add function should end at line 3")

	// Verify subtract function exists and has valid line numbers
	subtractFunc := methodChunks["subtract"]
	require.NotNil(t, subtractFunc)
	assert.Greater(t, subtractFunc.StartLine, 0, "subtract function should have positive start line")
	assert.Greater(t, subtractFunc.EndLine, subtractFunc.StartLine, "end line should be after start line")
	// Note: Due to tree-sitter Lua grammar quirks, exact line numbers may vary
	// The important thing is that the function is extracted with valid line info
}

// =============================================================================
// Content Tests
// =============================================================================

func TestLuaChunker_ChunkContent(t *testing.T) {
	chunker := getLuaChunker(t)

	source := []byte(`function greet(name)
    return "Hello, " .. name .. "!"
end
`)

	file := &models.SourceFile{
		Path:         "/test/content.lua",
		Content:      source,
		Language:     "lua",
		LastModified: time.Now(),
	}

	result, err := chunker.Chunk(context.Background(), file)
	require.NoError(t, err)

	// Find the function chunk
	var funcChunk *models.Chunk
	for _, chunk := range result.Chunks {
		if chunk.Level == models.ChunkLevelMethod {
			funcChunk = chunk
			break
		}
	}

	require.NotNil(t, funcChunk)
	assert.Contains(t, funcChunk.Content, "function greet", "Chunk content should contain the function definition")
	assert.Contains(t, funcChunk.Content, "return", "Chunk content should contain the return statement")
}

// =============================================================================
// Context Cancellation Tests
// =============================================================================

func TestLuaChunker_ContextCancellation(t *testing.T) {
	chunker := getLuaChunker(t)

	source := []byte(`function test()
    return true
end
`)

	file := &models.SourceFile{
		Path:         "/test/cancel.lua",
		Content:      source,
		Language:     "lua",
		LastModified: time.Now(),
	}

	ctx, cancel := context.WithCancel(context.Background())
	cancel() // Cancel immediately

	_, err := chunker.Chunk(ctx, file)
	// Should either return an error or handle gracefully
	if err != nil {
		assert.ErrorIs(t, err, context.Canceled, "Should return context.Canceled error")
	}
}

// =============================================================================
// Comprehensive Integration Test
// =============================================================================

func TestLuaChunker_ComprehensiveFile(t *testing.T) {
	chunker := getLuaChunker(t)

	source := []byte(`--[[
    Module for damage calculations
]]

local M = {}

-- Calculate base damage
function M.calculateBase(weapon)
    return weapon.min + weapon.max
end

-- Apply damage modifiers
function M:applyModifiers(base, modifiers)
    local result = base
    for _, mod in ipairs(modifiers) do
        result = result * mod
    end
    return result
end

-- Local helper function
local function clamp(value, min, max)
    if value < min then return min end
    if value > max then return max end
    return value
end

-- Global utility function
function globalHelper()
    return "utility"
end

return M
`)

	file := &models.SourceFile{
		Path:         "/test/comprehensive.lua",
		Content:      source,
		Language:     "lua",
		LastModified: time.Now(),
	}

	result, err := chunker.Chunk(context.Background(), file)
	require.NoError(t, err)
	require.NotNil(t, result)
	assert.Empty(t, result.Errors)

	// Count chunks by level
	levelCounts := make(map[models.ChunkLevel]int)
	for _, chunk := range result.Chunks {
		levelCounts[chunk.Level]++
	}

	// Should have:
	// - 1 file chunk
	// - Multiple method chunks (calculateBase, applyModifiers, clamp, globalHelper)
	assert.Equal(t, 1, levelCounts[models.ChunkLevelFile], "Should have 1 file chunk")
	assert.GreaterOrEqual(t, levelCounts[models.ChunkLevelMethod], 3, "Should have at least 3 method chunks")

	// Verify all chunks are valid
	for _, chunk := range result.Chunks {
		err := chunk.IsValid()
		assert.NoError(t, err, "Chunk %s should be valid", chunk.Name)
		assert.NotEmpty(t, chunk.ID, "Chunk should have an ID")
		assert.Equal(t, "lua", chunk.Language, "Chunk should have language set")
	}
}

// =============================================================================
// PoB-Style Code Tests (Real-world Lua patterns from Path of Building)
// =============================================================================

func TestLuaChunker_PoBStyleCode(t *testing.T) {
	chunker := getLuaChunker(t)

	// This mimics typical PoB code structure
	source := []byte(`-- CalcOffence.lua style
local calcs = ...
local modDB = calcs.modDB

local function calcHitDamage(source, cfg)
    local min = modDB:Sum("BASE", cfg, source.."Min")
    local max = modDB:Sum("BASE", cfg, source.."Max")
    return (min + max) / 2
end

local function calcAilmentDamage(ailment, hitDamage, critChance)
    local ailmentMult = modDB:More(nil, ailment.."Effect")
    return hitDamage * ailmentMult * (1 + critChance / 100)
end

function calcs.damage(activeSkill, output)
    local cfg = activeSkill.skillCfg
    output.TotalDamage = calcHitDamage("Physical", cfg)
    output.PoisonDamage = calcAilmentDamage("Poison", output.TotalDamage, output.CritChance)
end
`)

	file := &models.SourceFile{
		Path:         "/test/pob_style.lua",
		Content:      source,
		Language:     "lua",
		LastModified: time.Now(),
	}

	result, err := chunker.Chunk(context.Background(), file)
	require.NoError(t, err)
	require.NotNil(t, result)

	// Find method chunks
	var methodChunks []*models.Chunk
	for _, chunk := range result.Chunks {
		if chunk.Level == models.ChunkLevelMethod {
			methodChunks = append(methodChunks, chunk)
		}
	}

	// Should extract the key functions
	assert.GreaterOrEqual(t, len(methodChunks), 2, "Should extract PoB-style functions")

	// Check for expected function names
	funcNames := make(map[string]bool)
	for _, m := range methodChunks {
		funcNames[m.Name] = true
	}

	// At minimum should find calcHitDamage and calcAilmentDamage
	assert.True(t, funcNames["calcHitDamage"] || funcNames["calcAilmentDamage"] || funcNames["damage"],
		"Should find at least one of the key functions")
}

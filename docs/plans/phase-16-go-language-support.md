# Phase 16: Go Language Support

**Status:** Design Complete
**Branch:** `dev-go-support`
**Date:** 2025-12-29

---

## Overview

Add Go language support to Pommel's tree-sitter chunker, enabling semantic code search for Go codebases. This follows the existing pattern established by `python.go`, `csharp.go`, and `javascript.go`.

---

## Design Decisions

### Chunk Level Mapping

Go doesn't have classes, but has analogous constructs. After considering options for what would be most useful for AI agents searching Go code:

| Go Construct | Chunk Level | Rationale |
|--------------|-------------|-----------|
| `type X struct {...}` | class | Primary data structure definition |
| `type X interface {...}` | class | Contract definition |
| `func DoSomething()` | method | Top-level function |
| `func (x *X) DoSomething()` | method | Method with receiver |

### Parent Relationships

Methods with receivers will have their parent set to the **file chunk** (not the struct). This matches Go's idiom where methods can be defined in different files than their type definitions. The receiver type is captured in the chunk metadata for context.

---

## Files to Create/Modify

| File | Change |
|------|--------|
| `internal/chunker/go.go` | New - Go-specific chunker implementation |
| `internal/chunker/go_test.go` | New - Tests for Go chunker (written first - TDD) |
| `internal/chunker/treesitter.go` | Add `LangGo` constant, parser init, extension detection |
| `internal/chunker/chunker.go` | Register Go chunker in the factory |

---

## Implementation Details

### AST Node Types

Go's tree-sitter grammar uses these node types:

| Node Type | What it captures |
|-----------|------------------|
| `function_declaration` | Top-level functions: `func foo() {}` |
| `method_declaration` | Methods with receivers: `func (r *T) foo() {}` |
| `type_declaration` | Container for `type_spec` children |
| `type_spec` | The actual type: `struct_type`, `interface_type`, etc. |

### Extracting Names and Signatures

- **Functions**: `name` field gives function name, first line is signature
- **Methods**: `name` field gives method name, `receiver` field gives receiver type
- **Structs/Interfaces**: Inside `type_spec`, the `name` field gives type name

### GoChunker Structure

```go
type GoChunker struct {
    parser *Parser
}

func (c *GoChunker) Chunk(ctx context.Context, file *models.SourceFile) (*models.ChunkResult, error)
func (c *GoChunker) Language() Language
```

---

## Integration

### Changes to treesitter.go

1. Add language constant:
```go
LangGo Language = "go"
```

2. Add import:
```go
"github.com/smacker/go-tree-sitter/golang"
```

3. Initialize parser in `NewParser()`:
```go
goParser := sitter.NewParser()
goParser.SetLanguage(golang.GetLanguage())
parsers[LangGo] = goParser
```

4. Add extension detection in `DetectLanguage()`:
```go
case ".go":
    return LangGo
```

### Dependencies

The `go-tree-sitter` library already includes Go support via `github.com/smacker/go-tree-sitter/golang`. No new dependencies needed.

### File Patterns

`internal/config/defaults.go` already includes `**/*.go` in the default include patterns.

---

## Error Handling

| Scenario | Handling |
|----------|----------|
| Empty file | Return empty result with no chunks (not an error) |
| Parse error (invalid syntax) | Return error from tree-sitter, file skipped |
| Context cancellation | Check `ctx.Done()` early, return `ctx.Err()` |
| Missing name node | Skip that chunk, continue with others |

---

## Test Cases

Tests written first (TDD approach):

| Test | Description |
|------|-------------|
| `TestGoChunker_EmptyFile` | Returns empty result, no error |
| `TestGoChunker_SimpleFunction` | Top-level function extracts as method-level |
| `TestGoChunker_MethodWithReceiver` | Method extracts with correct receiver info |
| `TestGoChunker_StructType` | Struct extracts as class-level |
| `TestGoChunker_InterfaceType` | Interface extracts as class-level |
| `TestGoChunker_MultipleConstructs` | File with mix of types/functions/methods |
| `TestGoChunker_TypeAlias` | `type X = Y` handling |
| `TestGoChunker_GenericTypes` | Go 1.18+ generics: `type Stack[T any] struct{}` |

### Test Fixtures

Realistic Go code snippets covering:
- Package declaration
- Imports
- Comments and doc strings
- Exported vs unexported identifiers
- Multiple constructs in single file

---

## TDD Implementation Order

| Step | Action |
|------|--------|
| 1 | Write `go_test.go` with all test cases (they will fail) |
| 2 | Add `LangGo` constant and `DetectLanguage` case to `treesitter.go` |
| 3 | Write minimal `go.go` to make first test pass |
| 4 | Iterate: run tests, implement next feature, repeat |
| 5 | Integration: wire up in `chunker.go` |
| 6 | End-to-end: test with `pm reindex` on Pommel's own codebase |

### Test-First Sequence

```
1. TestGoChunker_EmptyFile          -> implement Chunk() shell
2. TestGoChunker_SimpleFunction     -> implement function extraction
3. TestGoChunker_MethodWithReceiver -> implement method extraction
4. TestGoChunker_StructType         -> implement struct extraction
5. TestGoChunker_InterfaceType      -> implement interface extraction
6. TestGoChunker_MultipleConstructs -> verify full integration
7. TestGoChunker_GenericTypes       -> handle Go 1.18+ generics
```

---

## Verification

After implementation, reindex the Pommel project itself:

```bash
pm reindex
pm status
pm search "port calculation" --level method
```

Expected results:
- Chunks at method level (functions, methods with receivers)
- Chunks at class level (structs, interfaces)
- Proper parent relationships to file chunks
- Improved search results for Go-specific queries

---

## Success Criteria

1. All test cases pass
2. `pm reindex` successfully indexes all 97 Go files in Pommel
3. Search results include method-level chunks (not just file-level)
4. No regressions in existing language support (Python, C#, JS/TS)

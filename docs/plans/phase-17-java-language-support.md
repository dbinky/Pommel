# Phase 17: Java Language Support

## Overview

Add full tree-sitter AST-aware chunking for Java source files, enabling semantic search across Java codebases with method-level and class-level granularity.

## AST Node Mapping

### Class-level chunks (`ChunkLevelClass`)

| Java Construct | Tree-sitter Node Type |
|----------------|----------------------|
| Class | `class_declaration` |
| Interface | `interface_declaration` |
| Enum | `enum_declaration` |
| Record | `record_declaration` |
| Annotation Type | `annotation_type_declaration` |

### Method-level chunks (`ChunkLevelMethod`)

| Java Construct | Tree-sitter Node Type |
|----------------|----------------------|
| Method | `method_declaration` |
| Constructor | `constructor_declaration` |

### Parent Relationships

- File chunk → parent is `nil`
- Class/interface/enum/record/annotation → parent is file chunk
- Method/constructor → parent is containing class-level chunk

**Note:** Flat structure - nested/inner classes get file as parent (not outer class). This matches the Go chunker approach and keeps implementation simple.

### Name & Signature Extraction

- **Name**: Extracted from AST node's `name` field
- **Signature**: First line of the construct (e.g., `public class MyService implements Service {`)

## Implementation Structure

### Files to Modify

1. **`internal/chunker/treesitter.go`**
   - Add `LangJava Language = "java"` constant
   - Import `github.com/smacker/go-tree-sitter/java`
   - Add Java parser initialization in `NewParser()`
   - Add `.java` extension detection in `DetectLanguage()`

2. **`internal/chunker/chunker.go`**
   - Register: `reg.chunkers[LangJava] = NewJavaChunker(parser)`

### Files to Create

3. **`internal/chunker/java.go`** (~200 lines)
   ```go
   type JavaChunker struct {
       parser *Parser
   }

   func NewJavaChunker(parser *Parser) *JavaChunker
   func (c *JavaChunker) Chunk(ctx context.Context, file *models.SourceFile) (*models.ChunkResult, error)
   func (c *JavaChunker) Language() Language

   // Internal methods
   func (c *JavaChunker) walkNode(node, file, parentID, result)
   func (c *JavaChunker) extractClassChunk(node, file, parentID) *models.Chunk
   func (c *JavaChunker) extractMethodChunk(node, file, parentID) *models.Chunk
   func (c *JavaChunker) createFileChunk(file) *models.Chunk
   ```

4. **`internal/chunker/java_test.go`** (~800 lines)
   - Comprehensive test suite (see Test Cases below)

## Error Handling

| Scenario | Behavior |
|----------|----------|
| Empty file | Return empty chunk result (no error) |
| Parse error | Return error from tree-sitter parser |
| Context cancelled | Check at entry, return `ctx.Err()` |
| Missing name node | Skip construct silently |
| Anonymous class | Skip (no name to extract) |

## Test Cases (TDD)

| Test | Description |
|------|-------------|
| `TestJavaChunker_EmptyFile` | Returns file chunk only |
| `TestJavaChunker_SimpleClass` | Class with one method |
| `TestJavaChunker_Interface` | Interface with method signatures |
| `TestJavaChunker_Enum` | Enum with constants and methods |
| `TestJavaChunker_Record` | Java 14+ record type |
| `TestJavaChunker_AnnotationType` | `@interface` declaration |
| `TestJavaChunker_Constructor` | Class with constructor |
| `TestJavaChunker_MultipleClasses` | File with multiple top-level classes |
| `TestJavaChunker_NestedClass` | Inner class gets file as parent (flat) |
| `TestJavaChunker_Generics` | Generic class and methods |
| `TestJavaChunker_CorrectLineNumbers` | Verify start/end lines |
| `TestJavaChunker_CorrectContent` | Content matches source |
| `TestJavaChunker_Signatures` | First line extracted correctly |
| `TestJavaChunker_DeterministicIDs` | Same input = same chunk IDs |
| `TestJavaChunker_ContextCancellation` | Respects context cancellation |

## Implementation Tasks

1. **Write test file** (`java_test.go`) - TDD approach
2. **Add LangJava to treesitter.go** - Constant, import, parser init, detection
3. **Implement JavaChunker** (`java.go`) - Main chunking logic
4. **Wire up in chunker.go** - Register the chunker
5. **Update existing tests** - Remove Java from "unsupported" test cases
6. **E2E verification** - Test with real Java codebase

## Success Criteria

- All 15 test cases pass
- Java files produce file, class, and method level chunks
- Chunk IDs are deterministic
- Line numbers and content are accurate
- Existing tests continue to pass

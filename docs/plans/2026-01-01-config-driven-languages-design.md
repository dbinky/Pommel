# Config-Driven Language Support Design

**Version:** v0.4.0
**Date:** 2025-01-01
**Status:** Draft

## Overview

Refactor language support from compiled Go code to external YAML configuration files. This enables adding new language support without rebuilding Pommel.

## Current State

Each language has a dedicated Go file (`internal/chunker/go.go`, `java.go`, etc.) with hardcoded node type mappings. Adding a language requires:
1. Writing Go code
2. Rebuilding binaries
3. New release

## Proposed Architecture

### Config Location

| Platform | Path |
|----------|------|
| macOS/Linux | `~/.local/share/pommel/languages/` |
| Windows | `%LOCALAPPDATA%\Pommel\languages\` |

### Directory Structure

```
~/.local/share/pommel/
└── languages/
    ├── csharp.yaml
    ├── go.yaml
    ├── java.yaml
    ├── javascript.yaml
    ├── python.yaml
    ├── rust.yaml
    └── typescript.yaml
```

### YAML Schema

```yaml
# Example: go.yaml
language: go
extensions:
  - .go

tree_sitter:
  grammar: go  # tree-sitter grammar name

chunk_mappings:
  file:
    - source_file

  class:
    - type_declaration
    - interface_type

  method:
    - function_declaration
    - method_declaration

  block:
    - if_statement
    - for_statement
    - switch_statement

# Optional: language-specific extraction rules
extraction:
  name_field: name           # AST field containing identifier
  doc_comment_types:
    - comment
```

### Behavior

1. **Startup (`pm start`):**
   - Read all `*.yaml` files from languages directory
   - Parse and validate each config
   - Register chunkers for each language

2. **No compiled-in fallbacks:**
   - If YAML doesn't exist, language isn't supported
   - Clean separation between code and configuration

3. **Error handling:**
   - Missing languages directory → clear error with setup instructions
   - Malformed YAML → skip with warning, continue loading others
   - Log which languages were successfully loaded

### Installation

The installer ships bundled language configs:

```bash
# install.sh additions
install_language_configs() {
    SHARE_DIR="$HOME/.local/share/pommel/languages"
    mkdir -p "$SHARE_DIR"
    # Download/extract language configs to $SHARE_DIR
}
```

## Code Changes

### Remove

- `internal/chunker/go.go`
- `internal/chunker/java.go`
- `internal/chunker/python.go`
- `internal/chunker/javascript.go`
- `internal/chunker/typescript.go`
- `internal/chunker/csharp.go`
- `internal/chunker/rust.go`
- Language-specific registration in `registry.go`

### Add

- `internal/chunker/config.go` - YAML parsing and config structs
- `internal/chunker/generic.go` - Config-driven chunker implementation
- `internal/config/paths.go` - Platform-specific path resolution
- `languages/*.yaml` - Bundled language configs (in repo, installed by installer)

### Modify

- `internal/chunker/registry.go` - Load from configs instead of hardcoded registration
- `scripts/install.sh` - Install language configs
- `scripts/install.ps1` - Install language configs (Windows)

## Benefits

1. **Extensibility:** Add languages without code changes
2. **User customization:** Modify chunking behavior via config
3. **Simpler codebase:** One generic chunker instead of N language-specific ones
4. **Community contributions:** Lower barrier for adding languages

## Migration

- No migration needed for users
- Fresh install includes all language configs
- Existing `.pommel/` project folders unchanged

## Supported Languages (v0.4.0)

1. C# (`csharp.yaml`)
2. Go (`go.yaml`)
3. Java (`java.yaml`)
4. JavaScript (`javascript.yaml`)
5. TypeScript (`typescript.yaml`)
6. Python (`python.yaml`)
7. Rust (`rust.yaml`)
8. Dart (`dart.yaml`)
9. PHP (`php.yaml`)
10. Kotlin (`kotlin.yaml`)
11. Elixir (`elixir.yaml`)
12. Solidity (`solidity.yaml`)
13. Swift (`swift.yaml`)

## Final YAML Schema

```yaml
# Language identifier and file detection
language: java                    # Unique identifier
display_name: Java                # Human-readable name
extensions:                       # File extensions (including dot)
  - .java

tree_sitter:
  grammar: java                   # tree-sitter grammar name

# Maps tree-sitter node types to Pommel chunk levels
chunk_mappings:
  # Top-level containers (classes, interfaces, modules)
  class:
    - class_declaration
    - interface_declaration
    - enum_declaration

  # Functions and methods
  method:
    - method_declaration
    - constructor_declaration

  # Control flow and logical blocks (optional)
  block:
    - if_statement
    - for_statement
    - while_statement
    - try_statement

# How to extract metadata from AST nodes
extraction:
  name_field: name                # Field containing identifier
  doc_comments:                   # Documentation comment node types
    - block_comment
    - line_comment
  doc_comment_position: preceding_siblings
```

## Open Questions

- [ ] Should we support user-level config overrides?
- [ ] Version field in YAML for future schema changes?

## Summary

Add Tree-sitter Markdown support to Pommel, enabling semantic search over `.md`, `.markdown`, `.mdown`, and `.mkdn` files. This allows AI coding agents to search documentation alongside code.

## Related Issue

Relates to expanding language support for non-code files.

## Type of Change

- [ ] Bug fix (non-breaking change that fixes an issue)
- [x] New feature (non-breaking change that adds functionality)
- [ ] Breaking change (fix or feature that would cause existing functionality to change)
- [ ] Documentation update
- [ ] Refactoring (no functional changes)
- [ ] Performance improvement
- [x] Test coverage improvement

## Changes Made

- Add markdown grammar import and registry entry in `internal/chunker/treesitter.go`
- Add `LangMarkdown` constant and parser initialization
- Add `DetectLanguage` case for `.md`, `.markdown`, `.mdown`, `.mkdn` extensions
- Create `languages/markdown.yaml` with chunk mappings (headings→class, code blocks/lists→method)
- Add markdown integration tests and fix name extraction edge case
- Bump version to 0.5.3
- Add `scripts/test-markdown.sh` for testing on markdown-heavy codebases
- Add `--batch-size` and `--cache-size` flags to `pm init` for embedding configuration
- Add `--stats-interval` flag to `pm init` for stats update frequency during indexing
- Fix status showing 0 files during active indexing (#31)
- Update README with Markdown in supported languages and new init flags

## Testing

### How has this been tested?

- [x] Unit tests added/updated
- [x] Integration tests added/updated
- [x] Manual testing performed

### Test commands run:

```bash
go test ./internal/chunker/...
go build -tags "fts5" ./...
```

### Manual testing steps:

1. Add `'**/*.md'` to `include_patterns` in `.pommel/config.yaml`
2. Run `pm reindex`
3. Run `pm search "contributing guidelines"`
4. Expected: Markdown files appear in search results with proper chunking

## Checklist

- [x] My code follows the project's code style
- [x] I have run `gofmt` and `go vet`
- [x] I have added tests that prove my fix/feature works
- [x] All new and existing tests pass
- [x] I have updated documentation (if applicable)
- [x] My changes don't introduce new warnings
- [x] This PR targets the `dev` branch (not `main`)

## Additional Notes

**Chunk mapping rationale:**
| Node Type | Chunk Level | Why |
|-----------|-------------|-----|
| `atx_heading` | class | Section headers define document structure |
| `setext_heading` | class | Alternative heading syntax |
| `fenced_code_block` | method | Code snippets are highly searchable |
| `indented_code_block` | method | Legacy code block syntax |
| `list` / `list_item` | method | Lists as searchable units |

**Recommended config for markdown-heavy projects:** Use `--batch-size 8` during init to avoid Ollama 500 errors during batch indexing:
```bash
pm init --auto --batch-size 8 --stats-interval 5 --start
```

**New pm init flags:**
- `--batch-size` - Embedding batch size (default 32)
- `--cache-size` - Embedding cache size (default 1000)
- `--stats-interval` - Stats update interval during indexing (default 10)

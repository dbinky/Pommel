# Phase 15: Feature Audit & Completion

**Status:** Not Started
**Effort:** Medium
**Dependencies:** Phase 14 (Search Scoping)

---

## Objective

Perform a comprehensive audit of all CLI commands, flags, and features documented in design documents, README, and CLAUDE.md against the actual implementation. Identify and implement any gaps to ensure documentation accurately reflects reality.

---

## Requirements

1. Audit all documented CLI commands and flags
2. Audit all documented JSON output formats
3. Audit all documented API endpoints
4. Audit all documented configuration options
5. Implement any missing features found during audit
6. Update documentation if implementation differs intentionally
7. Ensure consistency across README, CLAUDE.md, and design docs

---

## Audit Scope

### Documents to Review

| Document | Location | Contains |
|----------|----------|----------|
| README.md | `/README.md` | CLI commands, options, JSON examples |
| CLAUDE.md | `/CLAUDE.md` | CLI commands, quick start |
| Project Brief | `docs/plans/PROJECT_BRIEF.md` | Original design, CLI spec |
| v0.1 Design | `docs/plans/2025-12-28-v0.1-implementation-design.md` | Implementation details |
| v0.2 Design | `docs/plans/2025-12-29-multi-repo-design.md` | Multi-repo features |
| Phase docs | `docs/plans/phase-*.md` | Detailed implementation specs |

### Features to Audit

#### CLI Commands

| Command | Documented | Status |
|---------|------------|--------|
| `pm init` | README, CLAUDE.md | To verify |
| `pm init --auto` | README | To verify |
| `pm init --claude` | README | To verify |
| `pm init --start` | README | To verify |
| `pm init --monorepo` | v0.2 Design | To verify |
| `pm init --no-monorepo` | v0.2 Design | To verify |
| `pm start` | README | To verify |
| `pm start --foreground` | README | To verify |
| `pm stop` | README | To verify |
| `pm search <query>` | README | To verify |
| `pm search --limit` | README | To verify |
| `pm search --level` | README | To verify |
| `pm search --path` | README, v0.2 | To verify |
| `pm search --json` | README | To verify |
| `pm search --all` | v0.2 Design | To verify |
| `pm search --subproject` | v0.2 Design | To verify |
| `pm status` | README | To verify |
| `pm status --json` | README | To verify |
| `pm reindex` | README | To verify |
| `pm reindex --path` | README | To verify |
| `pm reindex --full` | v0.2 Design | To verify |
| `pm reindex --rescan-subprojects` | v0.2 Design | To verify |
| `pm config` | README | To verify |
| `pm config get <key>` | README | To verify |
| `pm config set <key> <value>` | README | To verify |
| `pm version` | CLAUDE.md | To verify |
| `pm subprojects` | v0.2 Design | To verify |

#### Global Flags

| Flag | Documented | Status |
|------|------------|--------|
| `--json` | README | To verify |
| `-p, --project` | README | To verify |
| `-v, --verbose` | Inferred | To verify |

#### JSON Output Formats

| Endpoint/Command | Documented Format | Status |
|------------------|-------------------|--------|
| `pm search --json` | README (detailed) | To verify |
| `pm status --json` | README (detailed) | To verify |
| `pm subprojects --json` | v0.2 Design | To verify |
| `/health` | v0.2 Design | To verify |

#### API Endpoints

| Endpoint | Documented | Status |
|----------|------------|--------|
| `POST /search` | Design docs | To verify |
| `GET /status` | Design docs | To verify |
| `GET /health` | v0.2 Design | To verify |
| `GET /subprojects` | v0.2 Design | To verify |
| `POST /reindex` | Design docs | To verify |

#### Configuration Options

| Option | Documented | Status |
|--------|------------|--------|
| `version` | README | To verify |
| `chunk_levels` | README | To verify |
| `include_patterns` | README | To verify |
| `exclude_patterns` | README | To verify |
| `watcher.debounce_ms` | README | To verify |
| `watcher.max_file_size` | README | To verify |
| `daemon.host` | README | To verify |
| `daemon.port` | README, v0.2 | To verify |
| `daemon.log_level` | README | To verify |
| `embedding.model` | README | To verify |
| `embedding.ollama_url` | README | To verify |
| `embedding.batch_size` | README | To verify |
| `embedding.cache_size` | README | To verify |
| `search.default_limit` | README | To verify |
| `search.default_levels` | README | To verify |
| `subprojects.auto_detect` | v0.2 Design | To verify |
| `subprojects.markers` | v0.2 Design | To verify |
| `subprojects.projects` | v0.2 Design | To verify |
| `subprojects.exclude` | v0.2 Design | To verify |

---

## Implementation Tasks

### 15.1 Create Audit Checklist

Generate a detailed checklist by parsing documentation files:

```bash
# Extract all pm commands from README
grep -E "^pm |^\`pm " README.md

# Extract all flags
grep -E "\-\-[a-z]" README.md

# Compare against actual implementation
./bin/pm --help
./bin/pm init --help
./bin/pm search --help
# ... etc
```

### 15.2 Systematic Feature Verification

For each documented feature:

1. **Verify existence** - Does the command/flag exist in code?
2. **Verify behavior** - Does it work as documented?
3. **Verify output** - Does JSON output match documented format?
4. **Document discrepancy** - Note any differences found

### 15.3 Implement Missing Features

For each missing feature found:

1. Create implementation following existing patterns
2. Add unit tests
3. Add integration tests
4. Verify against documentation

### 15.4 Update Documentation

For intentional deviations from documentation:

1. Update README.md
2. Update CLAUDE.md
3. Update relevant design documents
4. Ensure all docs are consistent

### 15.5 Verification Tests

Create end-to-end tests that verify:

```go
func TestDocumentedFeaturesExist(t *testing.T) {
    tests := []struct {
        command string
        args    []string
        wantErr bool
    }{
        {"pm", []string{"init", "--help"}, false},
        {"pm", []string{"init", "--auto", "--help"}, false},
        {"pm", []string{"init", "--claude", "--help"}, false},
        {"pm", []string{"search", "--help"}, false},
        {"pm", []string{"search", "--all", "--help"}, false},
        // ... all documented commands/flags
    }

    for _, tt := range tests {
        t.Run(strings.Join(tt.args, " "), func(t *testing.T) {
            cmd := exec.Command(tt.command, tt.args...)
            err := cmd.Run()
            if (err != nil) != tt.wantErr {
                t.Errorf("command %v: error = %v, wantErr %v",
                    tt.args, err, tt.wantErr)
            }
        })
    }
}
```

### 15.6 JSON Schema Validation

Create JSON schemas for documented output formats and validate:

```go
func TestSearchJSONOutput(t *testing.T) {
    // Run search with --json
    output := runCommand(t, "pm", "search", "test", "--json")

    // Verify required fields exist
    var result map[string]interface{}
    require.NoError(t, json.Unmarshal(output, &result))

    require.Contains(t, result, "query")
    require.Contains(t, result, "results")
    require.Contains(t, result, "total_results")
    require.Contains(t, result, "search_time_ms")
    require.Contains(t, result, "scope") // v0.2 addition

    // Verify results structure
    results := result["results"].([]interface{})
    if len(results) > 0 {
        first := results[0].(map[string]interface{})
        require.Contains(t, first, "id")
        require.Contains(t, first, "file")
        require.Contains(t, first, "start_line")
        require.Contains(t, first, "score")
        // ... etc
    }
}
```

---

## Audit Report Template

```markdown
# Pommel v0.2 Feature Audit Report

**Date:** YYYY-MM-DD
**Auditor:** [Name]

## Summary

- Total documented features: XX
- Implemented correctly: XX
- Missing/incomplete: XX
- Documentation updates needed: XX

## Detailed Findings

### CLI Commands

| Feature | Status | Notes |
|---------|--------|-------|
| `pm init` | ✅ Pass | |
| `pm init --auto` | ✅ Pass | |
| `pm init --monorepo` | ❌ Missing | Needs implementation |

### JSON Output

| Format | Status | Notes |
|--------|--------|-------|
| Search response | ⚠️ Partial | Missing `scope` field |

### Configuration

| Option | Status | Notes |
|--------|--------|-------|
| `daemon.port` | ✅ Pass | Now accepts null for hash-based |

## Action Items

1. [ ] Implement `pm init --monorepo`
2. [ ] Add `scope` to search response
3. [ ] Update README example for status output
```

---

## Testing

### Audit Tests

| Test | Description |
|------|-------------|
| `TestAllDocumentedCommands` | Every documented command exists |
| `TestAllDocumentedFlags` | Every documented flag exists |
| `TestJSONOutputFormats` | JSON matches documented schema |
| `TestConfigOptions` | All config options parsed correctly |

### Regression Tests

| Test | Description |
|------|-------------|
| `TestBackwardsCompatibility` | v0.1 commands still work |
| `TestDefaultBehavior` | Defaults match documentation |

---

## Acceptance Criteria

- [ ] Audit checklist completed for all documents
- [ ] All missing features implemented or documented as intentionally excluded
- [ ] All JSON output formats match documentation
- [ ] All configuration options work as documented
- [ ] README, CLAUDE.md, and design docs are consistent
- [ ] Audit report generated and reviewed
- [ ] No undocumented features exist (everything is documented)
- [ ] All tests pass

---

## Files Modified

| File | Change |
|------|--------|
| `README.md` | Updates for accuracy |
| `CLAUDE.md` | Updates for accuracy |
| `docs/plans/*.md` | Updates for accuracy |
| Various `internal/cli/*.go` | Missing feature implementations |
| Various `internal/api/*.go` | Missing endpoint implementations |
| `*_test.go` | Audit verification tests |

---

## Process

1. **Collect** - Gather all documented features from all docs
2. **Verify** - Check each feature against implementation
3. **Categorize** - Pass, Missing, Partial, Doc Update Needed
4. **Prioritize** - Critical (blocks users) vs Nice-to-have
5. **Implement** - Build missing critical features
6. **Document** - Update docs for intentional differences
7. **Test** - Add tests to prevent regression
8. **Report** - Generate final audit report

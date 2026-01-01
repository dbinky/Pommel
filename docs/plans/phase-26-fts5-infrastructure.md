# Phase 26: FTS5 Infrastructure

**Parent Design:** [2026-01-01-improved-search-design.md](./2026-01-01-improved-search-design.md)
**Methodology:** Strict TDD (Red-Green-Refactor)
**Branch:** `dev-improved-results`

## Overview

Implement SQLite FTS5 full-text search infrastructure to enable keyword-based search alongside existing vector search. This phase establishes the foundation for hybrid search.

## Goals

1. Create FTS5 virtual table schema for chunk content
2. Implement auto-migration to detect and create missing FTS table
3. Sync FTS index when chunks are inserted, updated, or deleted
4. Provide FTS query function for keyword search

## File Changes

### New Files
- `internal/db/fts.go` - FTS5 operations
- `internal/db/fts_test.go` - FTS5 tests
- `internal/db/migration.go` - Auto-migration logic
- `internal/db/migration_test.go` - Migration tests

### Modified Files
- `internal/db/database.go` - Add FTS sync calls to chunk operations
- `internal/db/database_test.go` - Update existing tests for FTS sync

## Implementation Tasks

### 26.1 FTS5 Table Schema

**File:** `internal/db/fts.go`

```go
// CreateFTSTable creates the chunks_fts virtual table
func (db *Database) CreateFTSTable(ctx context.Context) error

// FTS5 schema:
// CREATE VIRTUAL TABLE chunks_fts USING fts5(
//     chunk_id UNINDEXED,
//     content,
//     name,
//     file_path,
//     tokenize='porter unicode61'
// );
```

**Tests (26.1):**

| Test | Type | Description |
|------|------|-------------|
| `TestCreateFTSTable_Success` | Happy path | Creates table on fresh database |
| `TestCreateFTSTable_AlreadyExists` | Success case | No error if table already exists |
| `TestCreateFTSTable_InvalidDB` | Failure case | Returns error on closed database |
| `TestCreateFTSTable_ReadOnlyDB` | Failure case | Returns error on read-only database |
| `TestFTSTableSchema_HasExpectedColumns` | Success case | Verify chunk_id, content, name, file_path columns |
| `TestFTSTableSchema_PorterTokenizer` | Success case | Verify stemming works (e.g., "running" matches "run") |
| `TestFTSTableSchema_UnicodeSupport` | Edge case | Non-ASCII characters indexed correctly |

---

### 26.2 Auto-Migration Detection

**File:** `internal/db/migration.go`

```go
// TableExists checks if a table exists in the database
func (db *Database) TableExists(ctx context.Context, tableName string) (bool, error)

// EnsureFTSTable checks for FTS table and creates if missing
// Returns (created bool, err error) - created is true if table was just created
func (db *Database) EnsureFTSTable(ctx context.Context) (bool, error)
```

**Tests (26.2):**

| Test | Type | Description |
|------|------|-------------|
| `TestTableExists_ExistingTable` | Happy path | Returns true for chunks table |
| `TestTableExists_NonExistentTable` | Happy path | Returns false for random_xyz table |
| `TestTableExists_FTSTable` | Success case | Returns true for chunks_fts after creation |
| `TestTableExists_EmptyTableName` | Edge case | Returns false (or error) for empty string |
| `TestTableExists_SQLInjectionAttempt` | Edge case | Safely handles malicious table names |
| `TestTableExists_ClosedDB` | Failure case | Returns error on closed database |
| `TestEnsureFTSTable_CreatesWhenMissing` | Happy path | Creates table, returns (true, nil) |
| `TestEnsureFTSTable_NoOpWhenExists` | Success case | Returns (false, nil) when table exists |
| `TestEnsureFTSTable_PropagatesCreateError` | Failure case | Returns error if creation fails |
| `TestEnsureFTSTable_ConcurrentCalls` | Edge case | Multiple goroutines calling simultaneously |

---

### 26.3 FTS Sync on Insert

**File:** `internal/db/fts.go`

```go
// InsertFTSEntry adds a chunk to the FTS index
func (db *Database) InsertFTSEntry(ctx context.Context, chunk *models.Chunk) error

// Called from InsertChunk after successful chunk insertion
```

**Tests (26.3):**

| Test | Type | Description |
|------|------|-------------|
| `TestInsertFTSEntry_Success` | Happy path | Inserts entry, searchable immediately |
| `TestInsertFTSEntry_WithName` | Success case | Name field populated and searchable |
| `TestInsertFTSEntry_WithoutName` | Success case | Empty name handled correctly |
| `TestInsertFTSEntry_LongContent` | Success case | 100KB content indexed successfully |
| `TestInsertFTSEntry_EmptyContent` | Edge case | Empty content string handled |
| `TestInsertFTSEntry_SpecialCharacters` | Edge case | Code with symbols ({}[]<>) indexed |
| `TestInsertFTSEntry_NilChunk` | Failure case | Returns error for nil chunk |
| `TestInsertFTSEntry_MissingChunkID` | Failure case | Returns error for empty chunk_id |
| `TestInsertFTSEntry_NoFTSTable` | Failure case | Returns descriptive error |
| `TestInsertFTSEntry_DuplicateChunkID` | Edge case | Updates or errors appropriately |
| `TestInsertChunk_SyncsFTS` | Integration | InsertChunk also populates FTS |

---

### 26.4 FTS Sync on Update

**File:** `internal/db/fts.go`

```go
// UpdateFTSEntry updates an existing FTS entry
func (db *Database) UpdateFTSEntry(ctx context.Context, chunk *models.Chunk) error

// Called from UpdateChunk after successful chunk update
```

**Tests (26.4):**

| Test | Type | Description |
|------|------|-------------|
| `TestUpdateFTSEntry_Success` | Happy path | Updates content, new terms searchable |
| `TestUpdateFTSEntry_NameChange` | Success case | Updated name is searchable |
| `TestUpdateFTSEntry_ContentChange` | Success case | Old terms no longer match, new terms match |
| `TestUpdateFTSEntry_NonExistent` | Failure case | Returns error for unknown chunk_id |
| `TestUpdateFTSEntry_NilChunk` | Failure case | Returns error for nil chunk |
| `TestUpdateChunk_SyncsFTS` | Integration | UpdateChunk also updates FTS |

---

### 26.5 FTS Sync on Delete

**File:** `internal/db/fts.go`

```go
// DeleteFTSEntry removes a chunk from the FTS index
func (db *Database) DeleteFTSEntry(ctx context.Context, chunkID string) error

// Called from DeleteChunk after successful chunk deletion
```

**Tests (26.5):**

| Test | Type | Description |
|------|------|-------------|
| `TestDeleteFTSEntry_Success` | Happy path | Entry removed, no longer searchable |
| `TestDeleteFTSEntry_NonExistent` | Success case | No error for already-deleted entry |
| `TestDeleteFTSEntry_EmptyChunkID` | Edge case | Returns error for empty string |
| `TestDeleteFTSEntry_NoFTSTable` | Failure case | Returns descriptive error |
| `TestDeleteChunk_SyncsFTS` | Integration | DeleteChunk also removes from FTS |
| `TestDeleteFile_SyncsAllChunksFTS` | Integration | Deleting file removes all chunk FTS entries |

---

### 26.6 FTS Query Function

**File:** `internal/db/fts.go`

```go
// FTSSearch performs a full-text search and returns matching chunk IDs with scores
func (db *Database) FTSSearch(ctx context.Context, query string, limit int) ([]FTSResult, error)

type FTSResult struct {
    ChunkID string
    Score   float64  // BM25 score (lower is better in SQLite, we'll normalize)
}
```

**Tests (26.6):**

| Test | Type | Description |
|------|------|-------------|
| `TestFTSSearch_SingleTerm` | Happy path | "database" finds chunks containing "database" |
| `TestFTSSearch_MultipleTerm` | Happy path | "database connection" finds relevant chunks |
| `TestFTSSearch_NoResults` | Success case | Returns empty slice for unmatched query |
| `TestFTSSearch_LimitRespected` | Success case | Returns at most `limit` results |
| `TestFTSSearch_OrderedByRelevance` | Success case | Higher BM25 scores first |
| `TestFTSSearch_StemmedMatch` | Success case | "running" matches "run", "runs", "runner" |
| `TestFTSSearch_CaseInsensitive` | Success case | "Database" matches "database" |
| `TestFTSSearch_PartialWord` | Edge case | "data*" matches "database" (prefix search) |
| `TestFTSSearch_PhraseMatch` | Success case | `"exact phrase"` matches exact sequence |
| `TestFTSSearch_BooleanOR` | Success case | "cat OR dog" matches either |
| `TestFTSSearch_BooleanNOT` | Success case | "cat NOT dog" excludes dog |
| `TestFTSSearch_EmptyQuery` | Edge case | Returns empty results or error |
| `TestFTSSearch_SpecialCharacters` | Edge case | Query with `{}[]` handled safely |
| `TestFTSSearch_SQLInjection` | Edge case | Malicious query handled safely |
| `TestFTSSearch_ZeroLimit` | Edge case | Returns empty or uses default |
| `TestFTSSearch_NegativeLimit` | Edge case | Returns error or uses default |
| `TestFTSSearch_NoFTSTable` | Failure case | Returns descriptive error |
| `TestFTSSearch_ClosedDB` | Failure case | Returns error |
| `TestFTSSearch_ContextCancellation` | Failure case | Respects cancelled context |

---

### 26.7 Bulk FTS Population (for reindex)

**File:** `internal/db/fts.go`

```go
// PopulateFTSFromChunks rebuilds FTS index from existing chunks table
func (db *Database) PopulateFTSFromChunks(ctx context.Context) (int, error)

// Returns count of entries populated
```

**Tests (26.7):**

| Test | Type | Description |
|------|------|-------------|
| `TestPopulateFTS_EmptyChunks` | Happy path | Returns 0, no error |
| `TestPopulateFTS_SingleChunk` | Happy path | Populates 1 entry |
| `TestPopulateFTS_ManyChunks` | Success case | Populates 1000 chunks efficiently |
| `TestPopulateFTS_ClearsExisting` | Success case | Removes old FTS entries first |
| `TestPopulateFTS_Progress` | Success case | Can report progress (for large indexes) |
| `TestPopulateFTS_ContextCancellation` | Failure case | Stops and returns partial count |
| `TestPopulateFTS_NoFTSTable` | Failure case | Returns error if FTS table missing |

---

## Test Execution Order

Tests should be run in dependency order:

1. Schema tests (26.1) - Foundation
2. Migration tests (26.2) - Detection logic
3. Insert tests (26.3) - Write path
4. Update tests (26.4) - Modify path
5. Delete tests (26.5) - Remove path
6. Query tests (26.6) - Read path
7. Bulk population tests (26.7) - Reindex support

## Success Criteria

- [ ] All tests pass
- [ ] FTS table created on first daemon startup
- [ ] Auto-migration detects missing table and creates it
- [ ] Chunk CRUD operations sync to FTS
- [ ] FTS queries return relevant results with BM25 scores
- [ ] No performance regression on chunk operations (< 5ms overhead)
- [ ] Handles edge cases gracefully (empty content, special chars, etc.)

## Dependencies

- None (this is the foundation phase)

## Dependents

- Phase 27 (Hybrid Search) depends on FTS query function
- Phase 28 (Re-ranker) depends on FTS infrastructure

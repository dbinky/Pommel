# Phase 9: Schema Changes

**Status:** Not Started
**Effort:** Small
**Dependencies:** Phase 8 (Port Hashing)

---

## Objective

Extend the database schema to support sub-project metadata on chunks and create the `subprojects` table for tracking detected sub-projects. Since only the developer uses v0.1.x, the existing database will be dropped rather than migrated.

---

## Requirements

1. Add `subproject_id` and `subproject_path` columns to chunks table
2. Create `subprojects` table for sub-project registry
3. Add index on `subproject_id` for filtered queries
4. Update schema version to 2
5. Drop and recreate database (no migration needed)

---

## Implementation Tasks

### 9.1 Update Schema Constants

**File:** `internal/db/schema.go` (or equivalent)

```go
const SchemaVersion = 2

const CreateChunksTable = `
CREATE TABLE IF NOT EXISTS chunks (
    id              TEXT PRIMARY KEY,
    file_path       TEXT NOT NULL,
    start_line      INTEGER NOT NULL,
    end_line        INTEGER NOT NULL,
    chunk_level     TEXT NOT NULL,
    language        TEXT,
    name            TEXT,
    content         TEXT NOT NULL,
    content_hash    TEXT NOT NULL,
    parent_chunk_id TEXT,
    last_modified   TIMESTAMP NOT NULL,
    subproject_id   TEXT,
    subproject_path TEXT,
    FOREIGN KEY (parent_chunk_id) REFERENCES chunks(id)
);

CREATE INDEX IF NOT EXISTS idx_chunks_file_path ON chunks(file_path);
CREATE INDEX IF NOT EXISTS idx_chunks_subproject ON chunks(subproject_id);
CREATE INDEX IF NOT EXISTS idx_chunks_level ON chunks(chunk_level);
`

const CreateSubprojectsTable = `
CREATE TABLE IF NOT EXISTS subprojects (
    id              TEXT PRIMARY KEY,
    path            TEXT UNIQUE NOT NULL,
    name            TEXT,
    marker_file     TEXT,
    language_hint   TEXT,
    auto_detected   BOOLEAN DEFAULT true,
    created_at      TIMESTAMP DEFAULT CURRENT_TIMESTAMP,
    updated_at      TIMESTAMP DEFAULT CURRENT_TIMESTAMP
);
`

const CreateMetadataTable = `
CREATE TABLE IF NOT EXISTS metadata (
    key   TEXT PRIMARY KEY,
    value TEXT NOT NULL
);

INSERT OR REPLACE INTO metadata (key, value) VALUES ('schema_version', '2');
`
```

### 9.2 Update Chunk Model

**File:** `internal/models/chunk.go`

```go
type Chunk struct {
    ID            string    `json:"id"`
    FilePath      string    `json:"file_path"`
    StartLine     int       `json:"start_line"`
    EndLine       int       `json:"end_line"`
    ChunkLevel    string    `json:"chunk_level"`
    Language      string    `json:"language"`
    Name          string    `json:"name"`
    Content       string    `json:"content"`
    ContentHash   string    `json:"content_hash"`
    ParentChunkID *string   `json:"parent_chunk_id"`
    LastModified  time.Time `json:"last_modified"`

    // New fields for v0.2
    SubprojectID   *string `json:"subproject_id,omitempty"`
    SubprojectPath *string `json:"subproject_path,omitempty"`
}
```

### 9.3 Create Subproject Model

**File:** `internal/models/subproject.go` (new)

```go
package models

import "time"

type Subproject struct {
    ID           string    `json:"id"`
    Path         string    `json:"path"`
    Name         string    `json:"name,omitempty"`
    MarkerFile   string    `json:"marker_file,omitempty"`
    LanguageHint string    `json:"language_hint,omitempty"`
    AutoDetected bool      `json:"auto_detected"`
    CreatedAt    time.Time `json:"created_at"`
    UpdatedAt    time.Time `json:"updated_at"`
}
```

### 9.4 Add Subproject Database Operations

**File:** `internal/db/subprojects.go` (new)

```go
package db

import (
    "context"
    "database/sql"
    "time"

    "github.com/dbinky/pommel/internal/models"
)

// InsertSubproject adds a new subproject to the database.
func (db *Database) InsertSubproject(ctx context.Context, sp *models.Subproject) error {
    query := `
        INSERT INTO subprojects (id, path, name, marker_file, language_hint, auto_detected, created_at, updated_at)
        VALUES (?, ?, ?, ?, ?, ?, ?, ?)
        ON CONFLICT(id) DO UPDATE SET
            path = excluded.path,
            name = excluded.name,
            marker_file = excluded.marker_file,
            language_hint = excluded.language_hint,
            updated_at = excluded.updated_at
    `
    now := time.Now()
    _, err := db.conn.ExecContext(ctx, query,
        sp.ID, sp.Path, sp.Name, sp.MarkerFile, sp.LanguageHint, sp.AutoDetected, now, now)
    return err
}

// GetSubproject retrieves a subproject by ID.
func (db *Database) GetSubproject(ctx context.Context, id string) (*models.Subproject, error) {
    query := `SELECT id, path, name, marker_file, language_hint, auto_detected, created_at, updated_at
              FROM subprojects WHERE id = ?`

    var sp models.Subproject
    var name, markerFile, langHint sql.NullString

    err := db.conn.QueryRowContext(ctx, query, id).Scan(
        &sp.ID, &sp.Path, &name, &markerFile, &langHint,
        &sp.AutoDetected, &sp.CreatedAt, &sp.UpdatedAt)
    if err != nil {
        return nil, err
    }

    sp.Name = name.String
    sp.MarkerFile = markerFile.String
    sp.LanguageHint = langHint.String

    return &sp, nil
}

// ListSubprojects returns all subprojects.
func (db *Database) ListSubprojects(ctx context.Context) ([]*models.Subproject, error) {
    query := `SELECT id, path, name, marker_file, language_hint, auto_detected, created_at, updated_at
              FROM subprojects ORDER BY path`

    rows, err := db.conn.QueryContext(ctx, query)
    if err != nil {
        return nil, err
    }
    defer rows.Close()

    var subprojects []*models.Subproject
    for rows.Next() {
        var sp models.Subproject
        var name, markerFile, langHint sql.NullString

        if err := rows.Scan(&sp.ID, &sp.Path, &name, &markerFile, &langHint,
            &sp.AutoDetected, &sp.CreatedAt, &sp.UpdatedAt); err != nil {
            return nil, err
        }

        sp.Name = name.String
        sp.MarkerFile = markerFile.String
        sp.LanguageHint = langHint.String

        subprojects = append(subprojects, &sp)
    }

    return subprojects, rows.Err()
}

// DeleteSubproject removes a subproject by ID.
func (db *Database) DeleteSubproject(ctx context.Context, id string) error {
    _, err := db.conn.ExecContext(ctx, "DELETE FROM subprojects WHERE id = ?", id)
    return err
}

// GetSubprojectByPath finds a subproject containing the given file path.
func (db *Database) GetSubprojectByPath(ctx context.Context, filePath string) (*models.Subproject, error) {
    // Find the subproject whose path is a prefix of filePath
    query := `SELECT id, path, name, marker_file, language_hint, auto_detected, created_at, updated_at
              FROM subprojects
              WHERE ? LIKE path || '%'
              ORDER BY LENGTH(path) DESC
              LIMIT 1`

    var sp models.Subproject
    var name, markerFile, langHint sql.NullString

    err := db.conn.QueryRowContext(ctx, query, filePath).Scan(
        &sp.ID, &sp.Path, &name, &markerFile, &langHint,
        &sp.AutoDetected, &sp.CreatedAt, &sp.UpdatedAt)
    if err == sql.ErrNoRows {
        return nil, nil // No subproject contains this path
    }
    if err != nil {
        return nil, err
    }

    sp.Name = name.String
    sp.MarkerFile = markerFile.String
    sp.LanguageHint = langHint.String

    return &sp, nil
}
```

### 9.5 Update Chunk Insert/Query Operations

**File:** `internal/db/chunks.go`

Update insert query to include new fields:

```go
func (db *Database) InsertChunk(ctx context.Context, chunk *models.Chunk) error {
    query := `
        INSERT INTO chunks (id, file_path, start_line, end_line, chunk_level, language,
            name, content, content_hash, parent_chunk_id, last_modified,
            subproject_id, subproject_path)
        VALUES (?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?)
        ON CONFLICT(id) DO UPDATE SET
            content = excluded.content,
            content_hash = excluded.content_hash,
            last_modified = excluded.last_modified,
            subproject_id = excluded.subproject_id,
            subproject_path = excluded.subproject_path
    `
    _, err := db.conn.ExecContext(ctx, query,
        chunk.ID, chunk.FilePath, chunk.StartLine, chunk.EndLine,
        chunk.ChunkLevel, chunk.Language, chunk.Name, chunk.Content,
        chunk.ContentHash, chunk.ParentChunkID, chunk.LastModified,
        chunk.SubprojectID, chunk.SubprojectPath)
    return err
}
```

Update queries that return chunks to include new fields.

### 9.6 Add Schema Version Check

**File:** `internal/db/database.go`

```go
func (db *Database) CheckSchemaVersion(ctx context.Context) (int, error) {
    var version string
    err := db.conn.QueryRowContext(ctx,
        "SELECT value FROM metadata WHERE key = 'schema_version'").Scan(&version)
    if err == sql.ErrNoRows {
        return 0, nil // No version = v0 or uninitialized
    }
    if err != nil {
        return 0, err
    }

    v, err := strconv.Atoi(version)
    if err != nil {
        return 0, fmt.Errorf("invalid schema version: %s", version)
    }
    return v, nil
}

func (db *Database) Initialize(ctx context.Context) error {
    version, err := db.CheckSchemaVersion(ctx)
    if err != nil {
        return err
    }

    if version < SchemaVersion {
        // For v0.2, we simply drop and recreate
        // Future versions may implement proper migrations
        log.Warn("Schema version mismatch, recreating database")
        if err := db.dropAllTables(ctx); err != nil {
            return err
        }
    }

    // Create tables
    if _, err := db.conn.ExecContext(ctx, CreateChunksTable); err != nil {
        return err
    }
    if _, err := db.conn.ExecContext(ctx, CreateSubprojectsTable); err != nil {
        return err
    }
    if _, err := db.conn.ExecContext(ctx, CreateMetadataTable); err != nil {
        return err
    }

    return nil
}
```

### 9.7 Drop Existing Pommel Database

**Manual step during development:**

```bash
rm -rf .pommel/pommel.db
```

Or handled automatically by schema version check in 9.6.

---

## Testing

### Unit Tests

| Test | Description |
|------|-------------|
| `TestInsertSubproject` | Insert and retrieve subproject |
| `TestListSubprojects` | List all subprojects |
| `TestDeleteSubproject` | Delete subproject by ID |
| `TestGetSubprojectByPath` | Find subproject containing file path |
| `TestGetSubprojectByPath_Nested` | Innermost subproject wins |
| `TestInsertChunk_WithSubproject` | Chunk with subproject fields |
| `TestSchemaVersionCheck` | Detect schema version |

### Integration Tests

| Test | Description |
|------|-------------|
| `TestDatabaseInitialize_Fresh` | Creates tables on fresh DB |
| `TestDatabaseInitialize_OldSchema` | Drops and recreates on old schema |
| `TestChunkSubprojectQuery` | Query chunks by subproject_id |

---

## Acceptance Criteria

- [ ] `subprojects` table exists with correct schema
- [ ] `chunks` table has `subproject_id` and `subproject_path` columns
- [ ] Index exists on `chunks.subproject_id`
- [ ] Schema version is 2
- [ ] Old databases are dropped and recreated
- [ ] All CRUD operations work for subprojects
- [ ] Chunk insert/query includes subproject fields

---

## Files Modified

| File | Change |
|------|--------|
| `internal/db/schema.go` | Update schema version, add subprojects table |
| `internal/models/chunk.go` | Add SubprojectID, SubprojectPath fields |
| `internal/models/subproject.go` | New file: Subproject model |
| `internal/db/subprojects.go` | New file: subproject CRUD operations |
| `internal/db/chunks.go` | Update insert/query for new fields |
| `internal/db/database.go` | Schema version check, initialization |

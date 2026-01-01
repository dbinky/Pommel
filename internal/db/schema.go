package db

import (
	"context"
	"fmt"
)

const SchemaVersion = 3

// Migrate runs database migrations to ensure schema is up to date.
func (db *DB) Migrate(ctx context.Context) error {
	// Create schema_version table if it doesn't exist
	if err := db.createVersionTable(ctx); err != nil {
		return fmt.Errorf("failed to create version table: %w", err)
	}

	currentVersion, err := db.getSchemaVersion(ctx)
	if err != nil {
		return fmt.Errorf("failed to get schema version: %w", err)
	}

	if currentVersion < 1 {
		if err := db.migrateV1(ctx); err != nil {
			return fmt.Errorf("failed to run v1 migration: %w", err)
		}
	}

	if currentVersion < 2 {
		if err := db.migrateV2(ctx); err != nil {
			return fmt.Errorf("failed to run v2 migration: %w", err)
		}
	}

	if currentVersion < 3 {
		if err := db.migrateV3(ctx); err != nil {
			return fmt.Errorf("failed to run v3 migration: %w", err)
		}
	}

	return nil
}

func (db *DB) createVersionTable(ctx context.Context) error {
	_, err := db.Exec(ctx, `
		CREATE TABLE IF NOT EXISTS schema_version (
			version INTEGER PRIMARY KEY,
			applied_at DATETIME DEFAULT CURRENT_TIMESTAMP
		)
	`)
	return err
}

func (db *DB) getSchemaVersion(ctx context.Context) (int, error) {
	var version int
	err := db.QueryRow(ctx, "SELECT COALESCE(MAX(version), 0) FROM schema_version").Scan(&version)
	if err != nil {
		return 0, err
	}
	return version, nil
}

func (db *DB) setSchemaVersion(ctx context.Context, version int) error {
	_, err := db.Exec(ctx, "INSERT INTO schema_version (version) VALUES (?)", version)
	return err
}

// migrateV1 creates the initial schema with files, chunks, chunk_embeddings, and metadata tables.
func (db *DB) migrateV1(ctx context.Context) error {
	// Files table: tracks indexed source files
	if _, err := db.Exec(ctx, `
		CREATE TABLE IF NOT EXISTS files (
			id INTEGER PRIMARY KEY AUTOINCREMENT,
			path TEXT NOT NULL UNIQUE,
			content_hash TEXT NOT NULL,
			size INTEGER NOT NULL,
			modified_at DATETIME NOT NULL,
			indexed_at DATETIME DEFAULT CURRENT_TIMESTAMP,
			language TEXT
		)
	`); err != nil {
		return fmt.Errorf("failed to create files table: %w", err)
	}

	// Create index on path for fast lookups
	if _, err := db.Exec(ctx, `
		CREATE INDEX IF NOT EXISTS idx_files_path ON files(path)
	`); err != nil {
		return fmt.Errorf("failed to create files path index: %w", err)
	}

	// Chunks table: stores code chunks at various levels (file, class, method, block, line group)
	if _, err := db.Exec(ctx, `
		CREATE TABLE IF NOT EXISTS chunks (
			id TEXT PRIMARY KEY,
			file_id INTEGER NOT NULL,
			level TEXT NOT NULL,
			name TEXT,
			start_line INTEGER NOT NULL,
			end_line INTEGER NOT NULL,
			content TEXT NOT NULL,
			content_hash TEXT NOT NULL,
			parent_id TEXT,
			FOREIGN KEY (file_id) REFERENCES files(id) ON DELETE CASCADE,
			FOREIGN KEY (parent_id) REFERENCES chunks(id) ON DELETE CASCADE
		)
	`); err != nil {
		return fmt.Errorf("failed to create chunks table: %w", err)
	}

	// Create indexes for efficient chunk queries
	if _, err := db.Exec(ctx, `
		CREATE INDEX IF NOT EXISTS idx_chunks_file_id ON chunks(file_id)
	`); err != nil {
		return fmt.Errorf("failed to create chunks file_id index: %w", err)
	}

	if _, err := db.Exec(ctx, `
		CREATE INDEX IF NOT EXISTS idx_chunks_level ON chunks(level)
	`); err != nil {
		return fmt.Errorf("failed to create chunks level index: %w", err)
	}

	if _, err := db.Exec(ctx, `
		CREATE INDEX IF NOT EXISTS idx_chunks_parent_id ON chunks(parent_id)
	`); err != nil {
		return fmt.Errorf("failed to create chunks parent_id index: %w", err)
	}

	// Chunk embeddings table: sqlite-vec virtual table for vector similarity search
	// Uses 768-dimensional vectors to match Jina Code Embeddings
	if _, err := db.Exec(ctx, `
		CREATE VIRTUAL TABLE IF NOT EXISTS chunk_embeddings USING vec0(
			chunk_id TEXT PRIMARY KEY,
			embedding FLOAT[768]
		)
	`); err != nil {
		return fmt.Errorf("failed to create chunk_embeddings table: %w", err)
	}

	// Metadata table: stores project-level configuration and state
	if _, err := db.Exec(ctx, `
		CREATE TABLE IF NOT EXISTS metadata (
			key TEXT PRIMARY KEY,
			value TEXT NOT NULL,
			updated_at DATETIME DEFAULT CURRENT_TIMESTAMP
		)
	`); err != nil {
		return fmt.Errorf("failed to create metadata table: %w", err)
	}

	// Update schema version
	if err := db.setSchemaVersion(ctx, 1); err != nil {
		return fmt.Errorf("failed to set schema version: %w", err)
	}

	return nil
}

// GetSchemaVersion returns the current schema version.
func (db *DB) GetSchemaVersion(ctx context.Context) (int, error) {
	return db.getSchemaVersion(ctx)
}

// TableExists checks if a table exists in the database.
func (db *DB) TableExists(ctx context.Context, tableName string) (bool, error) {
	var count int
	err := db.QueryRow(ctx, `
		SELECT COUNT(*) FROM sqlite_master
		WHERE type='table' AND name=?
	`, tableName).Scan(&count)
	if err != nil {
		return false, err
	}
	return count > 0, nil
}

// VirtualTableExists checks if a virtual table exists in the database.
func (db *DB) VirtualTableExists(ctx context.Context, tableName string) (bool, error) {
	var count int
	err := db.QueryRow(ctx, `
		SELECT COUNT(*) FROM sqlite_master
		WHERE type='table' AND name=? AND sql LIKE '%VIRTUAL%'
	`, tableName).Scan(&count)
	if err != nil {
		return false, err
	}
	return count > 0, nil
}

// migrateV2 adds subprojects support (v0.2 multi-repo features).
func (db *DB) migrateV2(ctx context.Context) error {
	// Create subprojects table
	if _, err := db.Exec(ctx, `
		CREATE TABLE IF NOT EXISTS subprojects (
			id              TEXT PRIMARY KEY,
			path            TEXT UNIQUE NOT NULL,
			name            TEXT,
			marker_file     TEXT,
			language_hint   TEXT,
			auto_detected   INTEGER DEFAULT 1,
			created_at      DATETIME DEFAULT CURRENT_TIMESTAMP,
			updated_at      DATETIME DEFAULT CURRENT_TIMESTAMP
		)
	`); err != nil {
		return fmt.Errorf("failed to create subprojects table: %w", err)
	}

	// Create index on subprojects path
	if _, err := db.Exec(ctx, `
		CREATE INDEX IF NOT EXISTS idx_subprojects_path ON subprojects(path)
	`); err != nil {
		return fmt.Errorf("failed to create subprojects path index: %w", err)
	}

	// Add subproject columns to chunks table
	// SQLite doesn't support IF NOT EXISTS for ALTER TABLE, so we check first
	if !db.columnExists(ctx, "chunks", "subproject_id") {
		if _, err := db.Exec(ctx, `
			ALTER TABLE chunks ADD COLUMN subproject_id TEXT
		`); err != nil {
			return fmt.Errorf("failed to add subproject_id column: %w", err)
		}
	}

	if !db.columnExists(ctx, "chunks", "subproject_path") {
		if _, err := db.Exec(ctx, `
			ALTER TABLE chunks ADD COLUMN subproject_path TEXT
		`); err != nil {
			return fmt.Errorf("failed to add subproject_path column: %w", err)
		}
	}

	// Create index on chunks.subproject_id
	if _, err := db.Exec(ctx, `
		CREATE INDEX IF NOT EXISTS idx_chunks_subproject_id ON chunks(subproject_id)
	`); err != nil {
		return fmt.Errorf("failed to create chunks subproject_id index: %w", err)
	}

	// Update schema version
	if err := db.setSchemaVersion(ctx, 2); err != nil {
		return fmt.Errorf("failed to set schema version: %w", err)
	}

	return nil
}

// columnExists checks if a column exists in a table.
func (db *DB) columnExists(ctx context.Context, table, column string) bool {
	rows, err := db.Query(ctx, fmt.Sprintf("PRAGMA table_info(%s)", table))
	if err != nil {
		return false
	}
	defer rows.Close()

	for rows.Next() {
		var cid int
		var name, ctype string
		var notnull, pk int
		var dfltValue any
		if err := rows.Scan(&cid, &name, &ctype, &notnull, &dfltValue, &pk); err != nil {
			return false
		}
		if name == column {
			return true
		}
	}
	return false
}

// migrateV3 adds FTS5 full-text search support.
func (db *DB) migrateV3(ctx context.Context) error {
	// Create FTS5 table for full-text search
	if err := db.CreateFTSTable(ctx); err != nil {
		return fmt.Errorf("failed to create FTS table: %w", err)
	}

	// Populate FTS from existing chunks
	count, err := db.PopulateFTSFromChunks(ctx)
	if err != nil {
		return fmt.Errorf("failed to populate FTS table: %w", err)
	}

	// Log the number of entries populated (for debugging)
	_ = count

	// Update schema version
	if err := db.setSchemaVersion(ctx, 3); err != nil {
		return fmt.Errorf("failed to set schema version: %w", err)
	}

	return nil
}

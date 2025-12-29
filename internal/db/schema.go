package db

import (
	"context"
	"fmt"
)

const SchemaVersion = 1

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

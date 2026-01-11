package db

import (
	"context"
	"database/sql"
	"fmt"
	"os"
	"path/filepath"

	sqlite_vec "github.com/asg017/sqlite-vec-go-bindings/cgo"
	_ "github.com/mattn/go-sqlite3"
)

const (
	DatabaseFile = "index.db"
)

type DB struct {
	conn       *sql.DB
	path       string
	dimensions int
}

// Open creates or opens a database with the specified embedding dimensions.
// The dimensions parameter must match the embedding provider being used:
// - Ollama: 768 (Jina Code embeddings)
// - OpenAI: 1536 (text-embedding-3-small)
// - Voyage: 1024 (voyage-code-3)
func Open(projectRoot string, dimensions int) (*DB, error) {
	sqlite_vec.Auto()

	dbDir := filepath.Join(projectRoot, ".pommel")
	if err := os.MkdirAll(dbDir, 0755); err != nil {
		return nil, fmt.Errorf("failed to create database directory: %w", err)
	}

	dbPath := filepath.Join(dbDir, DatabaseFile)
	conn, err := sql.Open("sqlite3", dbPath)
	if err != nil {
		return nil, fmt.Errorf("failed to open database: %w", err)
	}

	if _, err := conn.Exec("PRAGMA journal_mode = WAL"); err != nil {
		conn.Close()
		return nil, fmt.Errorf("failed to enable WAL mode: %w", err)
	}

	if _, err := conn.Exec("PRAGMA foreign_keys = ON"); err != nil {
		conn.Close()
		return nil, fmt.Errorf("failed to enable foreign keys: %w", err)
	}

	db := &DB{conn: conn, path: dbPath, dimensions: dimensions}

	if err := db.verifySqliteVec(); err != nil {
		conn.Close()
		return nil, fmt.Errorf("sqlite-vec not available: %w", err)
	}

	if err := db.verifyFTS5(); err != nil {
		conn.Close()
		return nil, err
	}

	return db, nil
}

func (db *DB) verifySqliteVec() error {
	var version string
	err := db.conn.QueryRow("SELECT vec_version()").Scan(&version)
	if err != nil {
		return fmt.Errorf("vec_version() failed: %w", err)
	}
	return nil
}

// verifyFTS5 checks if FTS5 is available in the SQLite build.
// FTS5 is required for hybrid search functionality.
func (db *DB) verifyFTS5() error {
	// Try to create a temporary FTS5 table
	_, err := db.conn.Exec("CREATE VIRTUAL TABLE IF NOT EXISTS _fts5_check USING fts5(content)")
	if err != nil {
		return fmt.Errorf("FTS5 not available: %w\n\nFTS5 is required for Pommel. Build with: go build -tags fts5\nOr run tests with: go test -tags fts5 ./...\nAlternatively, use 'make build' or 'make test' which includes the fts5 tag", err)
	}
	// Clean up the test table
	_, _ = db.conn.Exec("DROP TABLE IF EXISTS _fts5_check")
	return nil
}

func (db *DB) Close() error {
	return db.conn.Close()
}

func (db *DB) Path() string {
	return db.path
}

// Dimensions returns the embedding dimensions configured for this database.
func (db *DB) Dimensions() int {
	return db.dimensions
}

func (db *DB) Conn() *sql.DB {
	return db.conn
}

func (db *DB) Exec(ctx context.Context, query string, args ...any) (sql.Result, error) {
	return db.conn.ExecContext(ctx, query, args...)
}

func (db *DB) Query(ctx context.Context, query string, args ...any) (*sql.Rows, error) {
	return db.conn.QueryContext(ctx, query, args...)
}

func (db *DB) QueryRow(ctx context.Context, query string, args ...any) *sql.Row {
	return db.conn.QueryRowContext(ctx, query, args...)
}

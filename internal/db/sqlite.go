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
	conn *sql.DB
	path string
}

func Open(projectRoot string) (*DB, error) {
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

	db := &DB{conn: conn, path: dbPath}

	if err := db.verifySqliteVec(); err != nil {
		conn.Close()
		return nil, fmt.Errorf("sqlite-vec not available: %w", err)
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

func (db *DB) Close() error {
	return db.conn.Close()
}

func (db *DB) Path() string {
	return db.path
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

package db

import (
	"context"
	"database/sql"
	"fmt"

	"github.com/pommel-dev/pommel/internal/models"
)

// InsertSubproject inserts or updates a subproject in the database.
// Uses INSERT OR REPLACE for upsert behavior.
func (db *DB) InsertSubproject(ctx context.Context, sp *models.Subproject) error {
	if err := sp.IsValid(); err != nil {
		return fmt.Errorf("invalid subproject: %w", err)
	}

	_, err := db.Exec(ctx, `
		INSERT OR REPLACE INTO subprojects (
			id, path, name, marker_file, language_hint, auto_detected, created_at, updated_at
		) VALUES (?, ?, ?, ?, ?, ?, ?, ?)
	`, sp.ID, sp.Path, sp.Name, sp.MarkerFile, sp.LanguageHint, sp.AutoDetected, sp.CreatedAt, sp.UpdatedAt)

	if err != nil {
		return fmt.Errorf("failed to insert subproject: %w", err)
	}
	return nil
}

// GetSubproject retrieves a subproject by ID.
// Returns nil, nil if the subproject is not found.
func (db *DB) GetSubproject(ctx context.Context, id string) (*models.Subproject, error) {
	row := db.QueryRow(ctx, `
		SELECT id, path, name, marker_file, language_hint, auto_detected, created_at, updated_at
		FROM subprojects WHERE id = ?
	`, id)

	sp := &models.Subproject{}
	var name, markerFile, languageHint sql.NullString

	err := row.Scan(
		&sp.ID,
		&sp.Path,
		&name,
		&markerFile,
		&languageHint,
		&sp.AutoDetected,
		&sp.CreatedAt,
		&sp.UpdatedAt,
	)

	if err == sql.ErrNoRows {
		return nil, nil
	}
	if err != nil {
		return nil, fmt.Errorf("failed to get subproject: %w", err)
	}

	sp.Name = name.String
	sp.MarkerFile = markerFile.String
	sp.LanguageHint = languageHint.String

	return sp, nil
}

// ListSubprojects returns all subprojects ordered by path.
func (db *DB) ListSubprojects(ctx context.Context) ([]*models.Subproject, error) {
	rows, err := db.Query(ctx, `
		SELECT id, path, name, marker_file, language_hint, auto_detected, created_at, updated_at
		FROM subprojects ORDER BY path
	`)
	if err != nil {
		return nil, fmt.Errorf("failed to list subprojects: %w", err)
	}
	defer rows.Close()

	var subprojects []*models.Subproject
	for rows.Next() {
		sp := &models.Subproject{}
		var name, markerFile, languageHint sql.NullString

		err := rows.Scan(
			&sp.ID,
			&sp.Path,
			&name,
			&markerFile,
			&languageHint,
			&sp.AutoDetected,
			&sp.CreatedAt,
			&sp.UpdatedAt,
		)
		if err != nil {
			return nil, fmt.Errorf("failed to scan subproject: %w", err)
		}

		sp.Name = name.String
		sp.MarkerFile = markerFile.String
		sp.LanguageHint = languageHint.String

		subprojects = append(subprojects, sp)
	}

	if err := rows.Err(); err != nil {
		return nil, fmt.Errorf("failed to iterate subprojects: %w", err)
	}

	return subprojects, nil
}

// DeleteSubproject removes a subproject by ID.
// Does not error if the subproject doesn't exist.
func (db *DB) DeleteSubproject(ctx context.Context, id string) error {
	_, err := db.Exec(ctx, `DELETE FROM subprojects WHERE id = ?`, id)
	if err != nil {
		return fmt.Errorf("failed to delete subproject: %w", err)
	}
	return nil
}

// GetSubprojectByPath finds the most specific subproject that contains the given file path.
// For nested subprojects, returns the one with the longest matching path.
// Returns nil, nil if no subproject contains the path.
func (db *DB) GetSubprojectByPath(ctx context.Context, filePath string) (*models.Subproject, error) {
	// Get all subprojects and find the best match
	subprojects, err := db.ListSubprojects(ctx)
	if err != nil {
		return nil, err
	}

	var bestMatch *models.Subproject
	bestMatchLen := -1

	for _, sp := range subprojects {
		if sp.ContainsPath(filePath) {
			if len(sp.Path) > bestMatchLen {
				bestMatch = sp
				bestMatchLen = len(sp.Path)
			}
		}
	}

	return bestMatch, nil
}

// SubprojectCount returns the total number of subprojects.
func (db *DB) SubprojectCount(ctx context.Context) (int64, error) {
	var count int64
	err := db.QueryRow(ctx, `SELECT COUNT(*) FROM subprojects`).Scan(&count)
	if err != nil {
		return 0, fmt.Errorf("failed to count subprojects: %w", err)
	}
	return count, nil
}

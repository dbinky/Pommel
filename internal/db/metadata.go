package db

import (
	"context"
	"database/sql"
	"fmt"
	"strconv"
)

// ProviderInfo contains information about the embedding provider used to index the database.
type ProviderInfo struct {
	Provider   string
	Model      string
	Dimensions int
}

// GetMetadata retrieves a metadata value by key.
// Returns empty string if the key doesn't exist.
func (db *DB) GetMetadata(ctx context.Context, key string) (string, error) {
	var value string
	err := db.QueryRow(ctx, `
		SELECT value FROM metadata WHERE key = ?
	`, key).Scan(&value)

	if err == sql.ErrNoRows {
		return "", nil
	}
	if err != nil {
		return "", fmt.Errorf("failed to get metadata '%s': %w", key, err)
	}
	return value, nil
}

// SetMetadata stores or updates a metadata key-value pair.
func (db *DB) SetMetadata(ctx context.Context, key, value string) error {
	_, err := db.Exec(ctx, `
		INSERT INTO metadata (key, value, updated_at) VALUES (?, ?, CURRENT_TIMESTAMP)
		ON CONFLICT(key) DO UPDATE SET value = excluded.value, updated_at = CURRENT_TIMESTAMP
	`, key, value)
	if err != nil {
		return fmt.Errorf("failed to set metadata '%s': %w", key, err)
	}
	return nil
}

// DeleteMetadata removes a metadata key-value pair.
func (db *DB) DeleteMetadata(ctx context.Context, key string) error {
	_, err := db.Exec(ctx, `DELETE FROM metadata WHERE key = ?`, key)
	if err != nil {
		return fmt.Errorf("failed to delete metadata '%s': %w", key, err)
	}
	return nil
}

// GetProviderInfo retrieves the stored embedding provider information.
func (db *DB) GetProviderInfo(ctx context.Context) (ProviderInfo, error) {
	info := ProviderInfo{}

	provider, err := db.GetMetadata(ctx, "provider")
	if err != nil {
		return info, err
	}
	info.Provider = provider

	model, err := db.GetMetadata(ctx, "model")
	if err != nil {
		return info, err
	}
	info.Model = model

	dimsStr, err := db.GetMetadata(ctx, "dimensions")
	if err != nil {
		return info, err
	}
	if dimsStr != "" {
		dims, err := strconv.Atoi(dimsStr)
		if err != nil {
			return info, fmt.Errorf("invalid dimensions value '%s': %w", dimsStr, err)
		}
		info.Dimensions = dims
	}

	return info, nil
}

// SetProviderInfo stores the embedding provider information.
func (db *DB) SetProviderInfo(ctx context.Context, info ProviderInfo) error {
	if err := db.SetMetadata(ctx, "provider", info.Provider); err != nil {
		return err
	}
	if err := db.SetMetadata(ctx, "model", info.Model); err != nil {
		return err
	}
	if err := db.SetMetadata(ctx, "dimensions", strconv.Itoa(info.Dimensions)); err != nil {
		return err
	}
	return nil
}

// ProviderChanged checks if the current provider configuration differs from what's stored.
// Returns false if no provider is currently stored (fresh database).
func (db *DB) ProviderChanged(ctx context.Context, provider, model string) (bool, error) {
	storedProvider, err := db.GetMetadata(ctx, "provider")
	if err != nil {
		return false, err
	}

	// No provider stored yet - not a change
	if storedProvider == "" {
		return false, nil
	}

	storedModel, err := db.GetMetadata(ctx, "model")
	if err != nil {
		return false, err
	}

	// Check if either provider or model changed
	return storedProvider != provider || storedModel != model, nil
}

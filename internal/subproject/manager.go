package subproject

import (
	"context"

	"github.com/pommel-dev/pommel/internal/config"
	"github.com/pommel-dev/pommel/internal/db"
	"github.com/pommel-dev/pommel/internal/models"
)

// Manager handles sub-project detection and management.
type Manager struct {
	db          *db.DB
	projectRoot string
	config      *config.SubprojectsConfig
}

// NewManager creates a new sub-project manager.
func NewManager(database *db.DB, projectRoot string, cfg *config.SubprojectsConfig) *Manager {
	return &Manager{
		db:          database,
		projectRoot: projectRoot,
		config:      cfg,
	}
}

// SyncSubprojects detects sub-projects and syncs with database.
// Returns (added, removed, unchanged) counts.
func (m *Manager) SyncSubprojects(ctx context.Context) (int, int, int, error) {
	// Get current subprojects from DB
	existing, err := m.db.ListSubprojects(ctx)
	if err != nil {
		return 0, 0, 0, err
	}
	existingMap := make(map[string]*models.Subproject)
	for _, sp := range existing {
		existingMap[sp.Path] = sp
	}

	// Detect subprojects if auto-detect enabled
	var detected []*DetectedSubproject
	if m.config.AutoDetect {
		detector := NewDetector(m.projectRoot, nil, m.config.Exclude)
		detected, err = detector.Scan()
		if err != nil {
			return 0, 0, 0, err
		}
	}

	// Merge with config overrides
	merged := m.mergeWithConfig(detected)

	// Sync with database
	added, removed, unchanged := 0, 0, 0
	seenPaths := make(map[string]bool)

	for _, sp := range merged {
		seenPaths[sp.Path] = true

		if _, exists := existingMap[sp.Path]; exists {
			// Already exists - count as unchanged
			// TODO: could check if fields changed and update
			unchanged++
		} else {
			// Add new
			sp.SetTimestamps()
			if err := m.db.InsertSubproject(ctx, sp); err != nil {
				return added, removed, unchanged, err
			}
			added++
		}
	}

	// Remove subprojects no longer detected (only auto-detected ones)
	for path, existingSp := range existingMap {
		if !seenPaths[path] && existingSp.AutoDetected {
			if err := m.db.DeleteSubproject(ctx, existingSp.ID); err != nil {
				return added, removed, unchanged, err
			}
			removed++
		}
	}

	return added, removed, unchanged, nil
}

// mergeWithConfig combines detected subprojects with config overrides.
func (m *Manager) mergeWithConfig(detected []*DetectedSubproject) []*models.Subproject {
	result := make([]*models.Subproject, 0)

	// Convert detected to map
	detectedMap := make(map[string]*DetectedSubproject)
	for _, d := range detected {
		detectedMap[d.Path] = d
	}

	// Apply config overrides
	for _, override := range m.config.Projects {
		sp := &models.Subproject{
			ID:           override.ID,
			Path:         override.Path,
			Name:         override.Name,
			AutoDetected: false,
		}

		// Merge with detected if exists
		if det, ok := detectedMap[override.Path]; ok {
			if sp.ID == "" {
				sp.ID = det.ID
			}
			sp.MarkerFile = det.MarkerFile
			sp.LanguageHint = det.LanguageHint
			delete(detectedMap, override.Path)
		}

		// Generate ID if still empty
		if sp.ID == "" {
			tmpSp := &models.Subproject{Path: sp.Path}
			sp.ID = tmpSp.GenerateID()
		}

		result = append(result, sp)
	}

	// Add remaining detected (not overridden)
	for _, det := range detectedMap {
		result = append(result, &models.Subproject{
			ID:           det.ID,
			Path:         det.Path,
			MarkerFile:   det.MarkerFile,
			LanguageHint: det.LanguageHint,
			AutoDetected: true,
		})
	}

	return result
}

// GetSubprojectForPath finds which subproject contains a file path.
func (m *Manager) GetSubprojectForPath(ctx context.Context, filePath string) (*models.Subproject, error) {
	return m.db.GetSubprojectByPath(ctx, filePath)
}

// AssignSubprojectToChunk determines and sets subproject fields on a chunk.
func (m *Manager) AssignSubprojectToChunk(ctx context.Context, chunk *models.Chunk) error {
	sp, err := m.GetSubprojectForPath(ctx, chunk.FilePath)
	if err != nil {
		return err
	}

	chunk.SetSubproject(sp)
	return nil
}

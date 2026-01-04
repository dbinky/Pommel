package daemon

import (
	"fmt"
	"sync"
	"time"
)

const rollingWindowSize = 10

// BatchTiming records timing for a single batch of chunks
type BatchTiming struct {
	Chunks   int
	Duration time.Duration
}

// IndexProgress tracks indexing progress with adaptive ETA calculation
type IndexProgress struct {
	TotalChunks     int
	CompletedChunks int
	StartTime       time.Time
	recentBatches   []BatchTiming

	mu sync.RWMutex
}

// NewIndexProgress creates a new progress tracker
func NewIndexProgress(totalChunks int) *IndexProgress {
	return &IndexProgress{
		TotalChunks:   totalChunks,
		StartTime:     time.Now(),
		recentBatches: make([]BatchTiming, 0, rollingWindowSize),
	}
}

// RecordBatch records a completed batch with its timing
func (p *IndexProgress) RecordBatch(chunks int, duration time.Duration) {
	if chunks <= 0 || duration <= 0 {
		// Still count chunks even if timing is invalid
		p.mu.Lock()
		if chunks > 0 {
			p.CompletedChunks += chunks
		}
		p.mu.Unlock()
		return
	}

	p.mu.Lock()
	defer p.mu.Unlock()

	p.CompletedChunks += chunks

	p.recentBatches = append(p.recentBatches, BatchTiming{
		Chunks:   chunks,
		Duration: duration,
	})

	// Keep only recent batches
	if len(p.recentBatches) > rollingWindowSize {
		p.recentBatches = p.recentBatches[len(p.recentBatches)-rollingWindowSize:]
	}
}

// ETA returns the estimated time remaining based on recent batch throughput
func (p *IndexProgress) ETA() time.Duration {
	p.mu.RLock()
	defer p.mu.RUnlock()

	if p.TotalChunks == 0 {
		return 0
	}

	remaining := p.TotalChunks - p.CompletedChunks
	if remaining <= 0 {
		return 0
	}

	// Need at least 2 data points for meaningful estimate
	if len(p.recentBatches) < 2 {
		return 0
	}

	// Calculate rate from recent batches
	var totalChunks int
	var totalDuration time.Duration
	for _, b := range p.recentBatches {
		totalChunks += b.Chunks
		totalDuration += b.Duration
	}

	if totalDuration == 0 {
		return 0
	}

	chunksPerSec := float64(totalChunks) / totalDuration.Seconds()
	if chunksPerSec <= 0 {
		return 0
	}

	return time.Duration(float64(remaining)/chunksPerSec) * time.Second
}

// Rate returns the current throughput in chunks per second
func (p *IndexProgress) Rate() float64 {
	p.mu.RLock()
	defer p.mu.RUnlock()

	if len(p.recentBatches) == 0 {
		return 0
	}

	var totalChunks int
	var totalDuration time.Duration
	for _, b := range p.recentBatches {
		totalChunks += b.Chunks
		totalDuration += b.Duration
	}

	if totalDuration == 0 {
		return 0
	}

	return float64(totalChunks) / totalDuration.Seconds()
}

// Percentage returns the completion percentage
func (p *IndexProgress) Percentage() float64 {
	p.mu.RLock()
	defer p.mu.RUnlock()

	if p.TotalChunks == 0 {
		return 100.0
	}

	return float64(p.CompletedChunks) / float64(p.TotalChunks) * 100.0
}

// FormatETA formats an ETA duration for display
func FormatETA(eta time.Duration) string {
	if eta == 0 {
		return "Calculating..."
	}

	seconds := int(eta.Seconds())
	if seconds < 60 {
		return fmt.Sprintf("%ds", seconds)
	}

	minutes := seconds / 60
	secs := seconds % 60

	if minutes < 60 {
		return fmt.Sprintf("%dm %ds", minutes, secs)
	}

	hours := minutes / 60
	mins := minutes % 60
	return fmt.Sprintf("%dh %dm", hours, mins)
}

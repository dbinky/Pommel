package daemon

import (
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
)

// === Happy Path Tests ===

func TestIndexProgress_ETA_SteadyRate(t *testing.T) {
	// Happy path: consistent batch timing gives accurate ETA
	p := NewIndexProgress(1000)

	// Simulate 10 batches of 10 chunks, each taking 1 second
	for i := 0; i < 10; i++ {
		p.RecordBatch(10, 1*time.Second)
	}

	// 100 done, 900 remaining, at 10/sec = 90 seconds
	eta := p.ETA()
	assert.InDelta(t, 90*time.Second, eta, float64(5*time.Second))
}

func TestIndexProgress_ETA_VariableRate(t *testing.T) {
	// Success scenario: variable timing uses recent average
	p := NewIndexProgress(1000)

	// Older batches: slow (ignored in rolling window)
	for i := 0; i < 5; i++ {
		p.RecordBatch(10, 5*time.Second) // 2 chunks/sec
	}

	// Recent batches: fast
	for i := 0; i < 10; i++ {
		p.RecordBatch(10, 500*time.Millisecond) // 20 chunks/sec
	}

	// Should use recent rate (~20/sec), not overall average
	eta := p.ETA()
	remaining := 1000 - 150 // 850 chunks
	expectedETA := time.Duration(float64(remaining) / 20.0 * float64(time.Second))

	assert.InDelta(t, expectedETA, eta, float64(10*time.Second))
}

func TestIndexProgress_Rate(t *testing.T) {
	p := NewIndexProgress(100)

	p.RecordBatch(10, 1*time.Second)
	p.RecordBatch(10, 1*time.Second)

	rate := p.Rate()
	assert.InDelta(t, 10.0, rate, 0.5) // ~10 chunks/sec
}

func TestIndexProgress_Percentage(t *testing.T) {
	p := NewIndexProgress(200)

	p.RecordBatch(50, time.Second)

	assert.Equal(t, 25.0, p.Percentage())
}

// === Edge Case Tests ===

func TestIndexProgress_ETA_NoData(t *testing.T) {
	// Edge case: no batches recorded yet
	p := NewIndexProgress(100)

	eta := p.ETA()
	assert.Equal(t, time.Duration(0), eta) // Calculating...
}

func TestIndexProgress_ETA_OneBatch(t *testing.T) {
	// Edge case: only one batch (not enough for average)
	p := NewIndexProgress(100)
	p.RecordBatch(10, time.Second)

	eta := p.ETA()
	assert.Equal(t, time.Duration(0), eta) // Need more data
}

func TestIndexProgress_ETA_Complete(t *testing.T) {
	// Edge case: all chunks done
	p := NewIndexProgress(100)

	for i := 0; i < 10; i++ {
		p.RecordBatch(10, time.Second)
	}

	eta := p.ETA()
	assert.Equal(t, time.Duration(0), eta) // Nothing remaining
}

func TestIndexProgress_ETA_ZeroTotal(t *testing.T) {
	// Edge case: zero total chunks
	p := NewIndexProgress(0)

	eta := p.ETA()
	assert.Equal(t, time.Duration(0), eta)
}

func TestIndexProgress_RollingWindow_Size(t *testing.T) {
	// Edge case: rolling window limits data points
	p := NewIndexProgress(10000)

	// Record more batches than window size
	for i := 0; i < 50; i++ {
		p.RecordBatch(10, time.Second)
	}

	// Internal window should be capped
	assert.LessOrEqual(t, len(p.recentBatches), 10)
}

// === Failure Scenario Tests ===

func TestIndexProgress_RecordBatch_ZeroDuration(t *testing.T) {
	// Failure scenario: zero duration batch (shouldn't panic)
	p := NewIndexProgress(100)

	assert.NotPanics(t, func() {
		p.RecordBatch(10, 0)
	})

	// Should still track chunks
	assert.Equal(t, 10, p.CompletedChunks)
}

func TestIndexProgress_RecordBatch_Negative(t *testing.T) {
	// Failure scenario: negative values (shouldn't happen but handle gracefully)
	p := NewIndexProgress(100)

	assert.NotPanics(t, func() {
		p.RecordBatch(-5, time.Second)
	})
}

// === ETA Formatting Tests ===

func TestFormatETA_Calculating(t *testing.T) {
	display := FormatETA(0)
	assert.Equal(t, "Calculating...", display)
}

func TestFormatETA_Seconds(t *testing.T) {
	display := FormatETA(45 * time.Second)
	assert.Equal(t, "45s", display)
}

func TestFormatETA_Minutes(t *testing.T) {
	display := FormatETA(95 * time.Second)
	assert.Equal(t, "1m 35s", display)
}

func TestFormatETA_Hours(t *testing.T) {
	display := FormatETA(3723 * time.Second) // 1h 2m 3s
	assert.Equal(t, "1h 2m", display)
}

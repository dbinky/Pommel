package setup

import (
	"context"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestDependencyStatus_Constants(t *testing.T) {
	// Verify status constants have expected values
	assert.Equal(t, Status(0), StatusMissing, "StatusMissing should be 0")
	assert.Equal(t, Status(1), StatusInstalled, "StatusInstalled should be 1")
	assert.Equal(t, Status(2), StatusRunning, "StatusRunning should be 2")
	assert.Equal(t, Status(3), StatusReady, "StatusReady should be 3")
}

func TestStatus_String(t *testing.T) {
	tests := []struct {
		status   Status
		expected string
	}{
		{StatusMissing, "missing"},
		{StatusInstalled, "installed"},
		{StatusRunning, "running"},
		{StatusReady, "ready"},
		{Status(99), "unknown"},
	}

	for _, tc := range tests {
		t.Run(tc.expected, func(t *testing.T) {
			assert.Equal(t, tc.expected, tc.status.String())
		})
	}
}

func TestNewDetector(t *testing.T) {
	// Test that NewDetector creates a detector with ollama dependency
	detector := NewDetector()

	require.NotNil(t, detector, "NewDetector should return a non-nil detector")
	require.NotEmpty(t, detector.dependencies, "Detector should have at least one dependency")

	// Find ollama dependency
	var ollamaDep *Dependency
	for i := range detector.dependencies {
		if detector.dependencies[i].Name == "ollama" {
			ollamaDep = &detector.dependencies[i]
			break
		}
	}

	require.NotNil(t, ollamaDep, "Detector should include ollama as a dependency")
	assert.True(t, ollamaDep.Required, "Ollama should be a required dependency")
	assert.NotEmpty(t, ollamaDep.CheckCmd, "Ollama should have a check command")
	assert.NotEmpty(t, ollamaDep.InstallHint, "Ollama should have an install hint")
}

func TestDetector_Check_Missing(t *testing.T) {
	// Test that Check returns StatusMissing when command is not found
	detector := &Detector{
		dependencies: []Dependency{
			{
				Name:        "nonexistent-command-12345",
				Required:    true,
				CheckCmd:    "nonexistent-command-12345 --version",
				InstallHint: "Install the nonexistent command",
			},
		},
	}

	ctx := context.Background()
	statuses := detector.Check(ctx)

	require.Len(t, statuses, 1, "Should return one status for one dependency")
	assert.Equal(t, "nonexistent-command-12345", statuses[0].Dependency.Name)
	assert.Equal(t, StatusMissing, statuses[0].Status, "Status should be missing for nonexistent command")
	assert.Empty(t, statuses[0].Version, "Version should be empty for missing dependency")
}

func TestDetector_Check_Installed(t *testing.T) {
	// Test that Check returns StatusInstalled when command exists
	// Using 'go' command which should be available in test environment
	detector := &Detector{
		dependencies: []Dependency{
			{
				Name:        "go",
				Required:    true,
				CheckCmd:    "go version",
				InstallHint: "Install Go from https://go.dev",
			},
		},
	}

	ctx := context.Background()
	statuses := detector.Check(ctx)

	require.Len(t, statuses, 1, "Should return one status for one dependency")
	assert.Equal(t, "go", statuses[0].Dependency.Name)
	assert.GreaterOrEqual(t, int(statuses[0].Status), int(StatusInstalled),
		"Status should be at least Installed for available command")
	assert.NotEmpty(t, statuses[0].Version, "Version should be captured for installed dependency")
}

func TestDetector_Check_MultipleDependencies(t *testing.T) {
	// Test that Check returns statuses for all dependencies
	detector := &Detector{
		dependencies: []Dependency{
			{
				Name:        "go",
				Required:    true,
				CheckCmd:    "go version",
				InstallHint: "Install Go",
			},
			{
				Name:        "missing-dep",
				Required:    false,
				CheckCmd:    "missing-dep-12345 --version",
				InstallHint: "Install missing-dep",
			},
		},
	}

	ctx := context.Background()
	statuses := detector.Check(ctx)

	require.Len(t, statuses, 2, "Should return statuses for all dependencies")

	// First dependency (go) should be installed
	assert.Equal(t, "go", statuses[0].Dependency.Name)
	assert.GreaterOrEqual(t, int(statuses[0].Status), int(StatusInstalled))

	// Second dependency (missing) should be missing
	assert.Equal(t, "missing-dep", statuses[1].Dependency.Name)
	assert.Equal(t, StatusMissing, statuses[1].Status)
}

func TestDetector_IsOllamaRunning(t *testing.T) {
	// Test IsOllamaRunning method
	// Note: This test may fail if Ollama is actually running, but it tests the interface
	detector := NewDetector()

	ctx := context.Background()
	running := detector.IsOllamaRunning(ctx)

	// We just verify the method returns a boolean without panicking
	// The actual result depends on whether Ollama is running
	assert.IsType(t, bool(false), running, "IsOllamaRunning should return a boolean")
}

func TestDetector_IsModelAvailable(t *testing.T) {
	// Test IsModelAvailable method
	detector := NewDetector()

	ctx := context.Background()

	// Test with a model that definitely doesn't exist
	available := detector.IsModelAvailable(ctx, "nonexistent-model-xyz-123")

	// Should return false for a nonexistent model
	assert.False(t, available, "Should return false for nonexistent model")
}

func TestDependency_Struct(t *testing.T) {
	// Test that Dependency struct has all expected fields
	dep := Dependency{
		Name:        "test-dep",
		Required:    true,
		CheckCmd:    "test-dep --version",
		InstallCmd:  "brew install test-dep",
		InstallHint: "Install test-dep using Homebrew",
	}

	assert.Equal(t, "test-dep", dep.Name)
	assert.True(t, dep.Required)
	assert.Equal(t, "test-dep --version", dep.CheckCmd)
	assert.Equal(t, "brew install test-dep", dep.InstallCmd)
	assert.Equal(t, "Install test-dep using Homebrew", dep.InstallHint)
}

func TestDependencyStatus_Struct(t *testing.T) {
	// Test that DependencyStatus struct has all expected fields
	dep := Dependency{Name: "test-dep", Required: true}
	status := DependencyStatus{
		Dependency: dep,
		Status:     StatusInstalled,
		Version:    "1.0.0",
		Error:      nil,
	}

	assert.Equal(t, dep, status.Dependency)
	assert.Equal(t, StatusInstalled, status.Status)
	assert.Equal(t, "1.0.0", status.Version)
	assert.Nil(t, status.Error)
}

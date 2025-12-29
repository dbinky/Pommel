package setup

import (
	"context"
	"os/exec"
	"runtime"
	"strings"
)

// Status represents the status of a dependency
type Status int

const (
	// StatusMissing indicates the dependency is not installed
	StatusMissing Status = iota
	// StatusInstalled indicates the dependency is installed but not running
	StatusInstalled
	// StatusRunning indicates the dependency is installed and running
	StatusRunning
	// StatusReady indicates the dependency is fully ready for use
	StatusReady
)

// String returns a string representation of the status
func (s Status) String() string {
	switch s {
	case StatusMissing:
		return "missing"
	case StatusInstalled:
		return "installed"
	case StatusRunning:
		return "running"
	case StatusReady:
		return "ready"
	default:
		return "unknown"
	}
}

// Dependency represents an external dependency required by Pommel
type Dependency struct {
	Name        string
	Required    bool
	CheckCmd    string
	InstallCmd  string
	InstallHint string
}

// DependencyStatus represents the current status of a dependency
type DependencyStatus struct {
	Dependency Dependency
	Status     Status
	Version    string
	Error      error
}

// Detector checks for required dependencies
type Detector struct {
	dependencies []Dependency
}

// NewDetector creates a new dependency detector with default dependencies
func NewDetector() *Detector {
	return &Detector{
		dependencies: []Dependency{
			{
				Name:        "ollama",
				Required:    true,
				CheckCmd:    "ollama --version",
				InstallCmd:  getOllamaInstallCmd(),
				InstallHint: "Install Ollama from https://ollama.ai",
			},
		},
	}
}

// getOllamaInstallCmd returns the platform-specific install command for Ollama
func getOllamaInstallCmd() string {
	switch runtime.GOOS {
	case "darwin":
		return "brew install ollama"
	case "linux":
		return "curl -fsSL https://ollama.ai/install.sh | sh"
	default:
		return ""
	}
}

// Check checks all dependencies and returns their statuses
func (d *Detector) Check(ctx context.Context) []DependencyStatus {
	statuses := make([]DependencyStatus, len(d.dependencies))

	for i, dep := range d.dependencies {
		status := DependencyStatus{
			Dependency: dep,
			Status:     StatusMissing,
		}

		// Run the check command
		parts := strings.Fields(dep.CheckCmd)
		if len(parts) > 0 {
			cmd := exec.CommandContext(ctx, parts[0], parts[1:]...)
			output, err := cmd.CombinedOutput()
			if err == nil {
				// Command succeeded - dependency is at least installed
				status.Status = StatusInstalled
				status.Version = strings.TrimSpace(string(output))

				// For ollama, check if it's running and if model is available
				if dep.Name == "ollama" {
					if d.IsOllamaRunning(ctx) {
						status.Status = StatusRunning
					}
				}
			} else {
				status.Error = err
			}
		}

		statuses[i] = status
	}

	return statuses
}

// IsOllamaRunning checks if the Ollama service is running
func (d *Detector) IsOllamaRunning(ctx context.Context) bool {
	cmd := exec.CommandContext(ctx, "curl", "-s", "http://localhost:11434")
	err := cmd.Run()
	return err == nil
}

// IsModelAvailable checks if a specific model is available in Ollama
func (d *Detector) IsModelAvailable(ctx context.Context, model string) bool {
	cmd := exec.CommandContext(ctx, "ollama", "list")
	output, err := cmd.Output()
	if err != nil {
		return false
	}
	return strings.Contains(string(output), model)
}

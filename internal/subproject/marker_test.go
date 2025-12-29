package subproject

import "testing"

func TestMatchesMarkerPattern(t *testing.T) {
	tests := []struct {
		filename string
		pattern  string
		expected bool
	}{
		// Exact matches
		{"go.mod", "go.mod", true},
		{"package.json", "package.json", true},
		{"Cargo.toml", "Cargo.toml", true},
		{"go.sum", "go.mod", false},
		{"go.mod.backup", "go.mod", false},

		// Wildcard patterns
		{"MyApp.csproj", "*.csproj", true},
		{"Another.csproj", "*.csproj", true},
		{"csproj", "*.csproj", false}, // Must have prefix
		{"MySolution.sln", "*.sln", true},
		{"file.txt", "*.sln", false},
	}

	for _, tt := range tests {
		t.Run(tt.filename+"_"+tt.pattern, func(t *testing.T) {
			result := MatchesMarkerPattern(tt.filename, tt.pattern)
			if result != tt.expected {
				t.Errorf("MatchesMarkerPattern(%q, %q) = %v, want %v",
					tt.filename, tt.pattern, result, tt.expected)
			}
		})
	}
}

func TestIsMarkerFile(t *testing.T) {
	tests := []struct {
		filename string
		patterns []string
		expected bool
	}{
		// Default patterns (nil patterns uses DefaultMarkerPatterns)
		{"go.mod", nil, true},
		{"package.json", nil, true},
		{"Cargo.toml", nil, true},
		{"MyProject.csproj", nil, true},
		{"Solution.sln", nil, true},
		{"random.txt", nil, false},

		// Custom patterns
		{"Makefile", []string{"Makefile", "CMakeLists.txt"}, true},
		{"CMakeLists.txt", []string{"Makefile", "CMakeLists.txt"}, true},
		{"go.mod", []string{"Makefile", "CMakeLists.txt"}, false},

		// Empty custom patterns uses defaults
		{"go.mod", []string{}, true},
	}

	for _, tt := range tests {
		name := tt.filename
		if tt.patterns != nil {
			name += "_custom"
		}
		t.Run(name, func(t *testing.T) {
			result := IsMarkerFile(tt.filename, tt.patterns)
			if result != tt.expected {
				t.Errorf("IsMarkerFile(%q, %v) = %v, want %v",
					tt.filename, tt.patterns, result, tt.expected)
			}
		})
	}
}

func TestGetLanguageHint(t *testing.T) {
	tests := []struct {
		markerFile string
		expected   string
	}{
		{"go.mod", "go"},
		{"package.json", "javascript"},
		{"Cargo.toml", "rust"},
		{"pom.xml", "java"},
		{"build.gradle", "java"},
		{"pyproject.toml", "python"},
		{"setup.py", "python"},
		{"MyProject.csproj", "csharp"},
		{"Solution.sln", "csharp"},
		{"unknown.file", ""},
	}

	for _, tt := range tests {
		t.Run(tt.markerFile, func(t *testing.T) {
			result := GetLanguageHint(tt.markerFile)
			if result != tt.expected {
				t.Errorf("GetLanguageHint(%q) = %q, want %q",
					tt.markerFile, result, tt.expected)
			}
		})
	}
}

func TestGenerateSubprojectID(t *testing.T) {
	tests := []struct {
		path     string
		expected string
	}{
		{"myproject", "myproject"},
		{"MyProject", "myproject"},
		{"my-project", "my-project"},
		{"my_project", "my-project"},
		{"src/myproject", "myproject"},
		{"path/to/deep/project", "project"},
		{"project.name", "project-name"},
		{"Project With Spaces", "project-with-spaces"},
	}

	for _, tt := range tests {
		t.Run(tt.path, func(t *testing.T) {
			result := GenerateSubprojectID(tt.path)
			if result != tt.expected {
				t.Errorf("GenerateSubprojectID(%q) = %q, want %q",
					tt.path, result, tt.expected)
			}
		})
	}
}

func TestDefaultMarkerPatterns(t *testing.T) {
	// Ensure DefaultMarkerPatterns contains expected patterns
	expectedPatterns := []string{
		"*.sln",
		"*.csproj",
		"go.mod",
		"Cargo.toml",
		"pom.xml",
		"build.gradle",
		"package.json",
		"pyproject.toml",
		"setup.py",
	}

	if len(DefaultMarkerPatterns) != len(expectedPatterns) {
		t.Errorf("DefaultMarkerPatterns has %d patterns, want %d",
			len(DefaultMarkerPatterns), len(expectedPatterns))
	}

	patternSet := make(map[string]bool)
	for _, p := range DefaultMarkerPatterns {
		patternSet[p] = true
	}

	for _, expected := range expectedPatterns {
		if !patternSet[expected] {
			t.Errorf("DefaultMarkerPatterns missing expected pattern: %q", expected)
		}
	}
}

func TestDefaultPriority(t *testing.T) {
	// Ensure DefaultPriority is defined and reasonable
	if DefaultPriority != 999 {
		t.Errorf("DefaultPriority = %d, want 999", DefaultPriority)
	}
}

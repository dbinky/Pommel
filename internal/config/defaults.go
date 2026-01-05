package config

// Default returns a Config with sensible default values
func Default() *Config {
	return &Config{
		Version: 1,
		ChunkLevels: []string{
			"method",
			"class",
			"file",
		},
		IncludePatterns: []string{
			"**/*.go",
			"**/*.cs",
			"**/*.py",
			"**/*.js",
			"**/*.ts",
			"**/*.jsx",
			"**/*.tsx",
			"**/*.java",
			"**/*.rs",
		},
		ExcludePatterns: []string{
			"**/node_modules/**",
			"**/bin/**",
			"**/obj/**",
			"**/__pycache__/**",
			"**/.git/**",
			"**/.pommel/**",
		},
		Watcher: WatcherConfig{
			DebounceMs:  500,
			MaxFileSize: 1048576, // 1MB
		},
		Daemon: DaemonConfig{
			Host:     "127.0.0.1",
			Port:     nil, // nil = use hash-based port calculation
			LogLevel: "info",
		},
		Embedding: EmbeddingConfig{
			Provider:  "ollama",
			Model:     "unclemusclez/jina-embeddings-v2-base-code", // Legacy: kept for backwards compatibility
			OllamaURL: "http://localhost:11434",                    // Legacy: kept for backwards compatibility
			BatchSize: 32,
			CacheSize: 1000,
			Ollama: OllamaProviderConfig{
				URL:   "http://localhost:11434",
				Model: "unclemusclez/jina-embeddings-v2-base-code",
			},
			OpenAI: OpenAIProviderConfig{
				Model: "text-embedding-3-small",
			},
			Voyage: VoyageProviderConfig{
				Model: "voyage-code-3",
			},
		},
		Search: SearchConfig{
			DefaultLimit: 10,
			DefaultLevels: []string{
				"method",
				"class",
			},
			Hybrid:   DefaultHybridSearchConfig(),
			Reranker: DefaultRerankerConfig(),
		},
		Subprojects: SubprojectsConfig{
			AutoDetect: true,
			Markers: []string{
				"*.sln",
				"*.csproj",
				"go.mod",
				"Cargo.toml",
				"pom.xml",
				"build.gradle",
				"package.json",
				"pyproject.toml",
				"setup.py",
			},
			Projects: nil,
			Exclude:  nil,
		},
	}
}

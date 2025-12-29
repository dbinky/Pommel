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
			"**/*.cs",
			"**/*.py",
			"**/*.js",
			"**/*.ts",
			"**/*.jsx",
			"**/*.tsx",
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
			Model:     "unclemusclez/jina-embeddings-v2-base-code",
			OllamaURL: "http://localhost:11434",
			BatchSize: 32,
			CacheSize: 1000,
		},
		Search: SearchConfig{
			DefaultLimit: 10,
			DefaultLevels: []string{
				"method",
				"class",
			},
		},
	}
}

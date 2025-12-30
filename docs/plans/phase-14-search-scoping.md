# Phase 14: Search Scoping

**Status:** Not Started
**Effort:** Medium
**Dependencies:** Phase 13 (Init Changes)

---

## Objective

Implement intelligent search scoping that auto-detects the current sub-project from the working directory, with explicit override flags (`--all`, `--path`, `--subproject`). Update JSON output to include scope information.

---

## Requirements

1. Auto-detect sub-project from current working directory
2. `--all` flag searches entire index (no filtering)
3. `--path` flag filters by path prefix
4. `--subproject` flag filters by sub-project ID
5. Mutual exclusivity: `--path` and `--subproject` cannot be combined
6. Updated JSON output includes scope metadata
7. Human-readable output shows scope context

---

## Implementation Tasks

### 14.1 Add Search Scope Flags

**File:** `internal/cli/search.go`

```go
var (
    searchLimit      int
    searchLevel      []string
    searchPath       string
    searchJSON       bool
    searchAll        bool
    searchSubproject string
)

func init() {
    rootCmd.AddCommand(searchCmd)
    searchCmd.Flags().IntVarP(&searchLimit, "limit", "n", 10, "Maximum number of results")
    searchCmd.Flags().StringSliceVarP(&searchLevel, "level", "l", nil, "Chunk levels to include")
    searchCmd.Flags().StringVarP(&searchPath, "path", "p", "", "Filter by path prefix")
    searchCmd.Flags().BoolVarP(&searchJSON, "json", "j", false, "Output as JSON")
    searchCmd.Flags().BoolVar(&searchAll, "all", false, "Search entire index (no scope filtering)")
    searchCmd.Flags().StringVar(&searchSubproject, "subproject", "", "Filter by sub-project ID")
}

var searchCmd = &cobra.Command{
    Use:   "search <query>",
    Short: "Semantic search across the codebase",
    Args:  cobra.ExactArgs(1),
    RunE:  runSearch,
}
```

### 14.2 Implement Scope Resolution

**File:** `internal/cli/search.go`

```go
type SearchScope struct {
    Mode           string  `json:"mode"`            // "auto", "all", "path", "subproject"
    Subproject     *string `json:"subproject"`      // Sub-project ID if applicable
    ResolvedPath   *string `json:"resolved_path"`   // Path prefix used for filtering
}

func resolveSearchScope(projectRoot string, client *Client) (*SearchScope, error) {
    // Check for conflicting flags
    if searchPath != "" && searchSubproject != "" {
        return nil, fmt.Errorf("cannot use --path and --subproject together")
    }

    // Explicit --all
    if searchAll {
        if searchPath != "" {
            log.Warn("--all searches everything; --path ignored")
        }
        return &SearchScope{Mode: "all"}, nil
    }

    // Explicit --subproject
    if searchSubproject != "" {
        // Validate subproject exists
        sp, err := client.GetSubproject(searchSubproject)
        if err != nil {
            return nil, err
        }
        if sp == nil {
            available, _ := client.ListSubprojects()
            names := make([]string, len(available))
            for i, s := range available {
                names[i] = s.ID
            }
            return nil, fmt.Errorf("unknown sub-project '%s'. Available: %s",
                searchSubproject, strings.Join(names, ", "))
        }
        return &SearchScope{
            Mode:         "subproject",
            Subproject:   &sp.ID,
            ResolvedPath: &sp.Path,
        }, nil
    }

    // Explicit --path
    if searchPath != "" {
        return &SearchScope{
            Mode:         "path",
            ResolvedPath: &searchPath,
        }, nil
    }

    // Auto-detect from cwd
    cwd, err := os.Getwd()
    if err != nil {
        return &SearchScope{Mode: "all"}, nil
    }

    relPath, err := filepath.Rel(projectRoot, cwd)
    if err != nil || strings.HasPrefix(relPath, "..") {
        // cwd is outside project root
        return nil, fmt.Errorf("current directory is outside project root. Use --all or specify --path")
    }

    if relPath == "." {
        // At project root - search everything
        return &SearchScope{Mode: "all"}, nil
    }

    // Find which subproject contains this path
    sp, err := client.GetSubprojectForPath(relPath)
    if err != nil {
        return nil, err
    }

    if sp == nil {
        // Not in any subproject - search everything
        return &SearchScope{Mode: "all"}, nil
    }

    return &SearchScope{
        Mode:         "auto",
        Subproject:   &sp.ID,
        ResolvedPath: &sp.Path,
    }, nil
}
```

### 14.3 Update Search API Request

**File:** `internal/api/types.go`

```go
type SearchRequest struct {
    Query  string       `json:"query"`
    Limit  int          `json:"limit"`
    Levels []string     `json:"levels,omitempty"`
    Scope  SearchScope  `json:"scope"`
}

type SearchScope struct {
    Mode  string `json:"mode"`            // "all", "path", "subproject"
    Value string `json:"value,omitempty"` // path prefix or subproject ID
}
```

### 14.4 Update Search Handler

**File:** `internal/daemon/handlers.go` or `internal/api/handlers.go`

```go
func (h *Handler) handleSearch(w http.ResponseWriter, r *http.Request) {
    var req api.SearchRequest
    if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
        http.Error(w, err.Error(), http.StatusBadRequest)
        return
    }

    ctx := r.Context()

    // Generate query embedding
    queryEmbedding, err := h.embedder.Embed(ctx, req.Query)
    if err != nil {
        http.Error(w, err.Error(), http.StatusInternalServerError)
        return
    }

    // Build search options
    opts := db.SearchOptions{
        Embedding: queryEmbedding,
        Limit:     req.Limit,
        Levels:    req.Levels,
    }

    // Apply scope filtering
    switch req.Scope.Mode {
    case "all":
        // No filtering
    case "path":
        opts.PathPrefix = req.Scope.Value
    case "subproject":
        opts.SubprojectID = req.Scope.Value
    }

    // Execute search
    results, err := h.db.Search(ctx, opts)
    if err != nil {
        http.Error(w, err.Error(), http.StatusInternalServerError)
        return
    }

    // Build response
    resp := buildSearchResponse(req.Query, req.Scope, results)
    json.NewEncoder(w).Encode(resp)
}
```

### 14.5 Update Database Search Query

**File:** `internal/db/search.go`

```go
type SearchOptions struct {
    Embedding    []float32
    Limit        int
    Levels       []string
    PathPrefix   string
    SubprojectID string
}

func (db *Database) Search(ctx context.Context, opts SearchOptions) ([]*models.SearchResult, error) {
    // Build WHERE clause
    conditions := []string{}
    args := []interface{}{}

    if len(opts.Levels) > 0 {
        placeholders := make([]string, len(opts.Levels))
        for i, level := range opts.Levels {
            placeholders[i] = "?"
            args = append(args, level)
        }
        conditions = append(conditions, fmt.Sprintf("c.chunk_level IN (%s)", strings.Join(placeholders, ",")))
    }

    if opts.PathPrefix != "" {
        conditions = append(conditions, "c.file_path LIKE ? || '%'")
        args = append(args, opts.PathPrefix)
    }

    if opts.SubprojectID != "" {
        conditions = append(conditions, "c.subproject_id = ?")
        args = append(args, opts.SubprojectID)
    }

    whereClause := ""
    if len(conditions) > 0 {
        whereClause = "WHERE " + strings.Join(conditions, " AND ")
    }

    query := fmt.Sprintf(`
        SELECT c.id, c.file_path, c.start_line, c.end_line, c.chunk_level,
               c.language, c.name, c.content, c.subproject_id, c.subproject_path,
               c.parent_chunk_id,
               vec_distance_cosine(e.embedding, ?) as distance
        FROM chunks c
        JOIN embeddings e ON c.id = e.chunk_id
        %s
        ORDER BY distance ASC
        LIMIT ?
    `, whereClause)

    args = append([]interface{}{serializeEmbedding(opts.Embedding)}, args...)
    args = append(args, opts.Limit)

    rows, err := db.conn.QueryContext(ctx, query, args...)
    // ... parse results ...
}
```

### 14.6 Update Search Response Format

**File:** `internal/api/types.go`

```go
type SearchResponse struct {
    Query        string               `json:"query"`
    Scope        SearchScopeResponse  `json:"scope"`
    Results      []*SearchResult      `json:"results"`
    TotalResults int                  `json:"total_results"`
    SearchTimeMs int64                `json:"search_time_ms"`
}

type SearchScopeResponse struct {
    Mode         string  `json:"mode"`
    Subproject   *string `json:"subproject,omitempty"`
    ResolvedPath *string `json:"resolved_path,omitempty"`
}

type SearchResult struct {
    ID         string             `json:"id"`
    File       string             `json:"file"`
    StartLine  int                `json:"start_line"`
    EndLine    int                `json:"end_line"`
    Level      string             `json:"level"`
    Language   string             `json:"language"`
    Name       string             `json:"name"`
    Score      float64            `json:"score"`
    Content    string             `json:"content"`
    Subproject *SubprojectRef     `json:"subproject,omitempty"`
    Parent     *ParentRef         `json:"parent,omitempty"`
}

type SubprojectRef struct {
    ID   string `json:"id"`
    Path string `json:"path"`
    Name string `json:"name,omitempty"`
}

type ParentRef struct {
    ID    string `json:"id"`
    Name  string `json:"name"`
    Level string `json:"level"`
}
```

### 14.7 Update CLI Output

**File:** `internal/cli/search.go`

```go
func displaySearchResults(resp *api.SearchResponse, jsonOutput bool) error {
    if jsonOutput {
        return json.NewEncoder(os.Stdout).Encode(resp)
    }

    // Human-readable output
    fmt.Printf("\nSearching in: ")
    switch resp.Scope.Mode {
    case "all":
        fmt.Println("all sub-projects")
    case "auto", "subproject":
        fmt.Printf("%s (%s)\n", *resp.Scope.Subproject, *resp.Scope.ResolvedPath)
    case "path":
        fmt.Printf("path %s\n", *resp.Scope.ResolvedPath)
    }
    fmt.Println()

    if len(resp.Results) == 0 {
        fmt.Println("No results found")
        return nil
    }

    for _, r := range resp.Results {
        // Show subproject tag for --all searches
        if resp.Scope.Mode == "all" && r.Subproject != nil {
            fmt.Printf("  [%s] ", r.Subproject.ID)
        } else {
            fmt.Print("  ")
        }

        fmt.Printf("%s:%d-%d (%s)\n", r.File, r.StartLine, r.EndLine, r.Level)
        fmt.Printf("  %-50s %.2f\n", r.Name, r.Score)

        // Show truncated content
        content := strings.ReplaceAll(r.Content, "\n", " ")
        if len(content) > 80 {
            content = content[:77] + "..."
        }
        fmt.Printf("  %s\n\n", content)
    }

    // Summary
    resultWord := "result"
    if resp.TotalResults != 1 {
        resultWord = "results"
    }
    fmt.Printf("%d %s • %dms\n", resp.TotalResults, resultWord, resp.SearchTimeMs)

    return nil
}
```

---

## Testing

### Unit Tests

| Test | Description |
|------|-------------|
| `TestResolveScope_All` | --all returns mode=all |
| `TestResolveScope_Path` | --path returns mode=path |
| `TestResolveScope_Subproject` | --subproject returns mode=subproject |
| `TestResolveScope_Conflict` | --path + --subproject errors |
| `TestResolveScope_AutoDetect` | Detects from cwd |
| `TestResolveScope_ProjectRoot` | At root returns mode=all |
| `TestResolveScope_OutsideProject` | Outside project errors |

### Integration Tests

| Test | Description |
|------|-------------|
| `TestSearch_AllScope` | --all returns results from all subprojects |
| `TestSearch_SubprojectScope` | Results only from specified subproject |
| `TestSearch_PathScope` | Results only from path prefix |
| `TestSearch_AutoScope` | Correct subproject auto-detected |
| `TestSearch_JSONOutput` | Response includes scope metadata |

---

## Acceptance Criteria

- [ ] `pm search` in subproject dir auto-scopes to that subproject
- [ ] `pm search --all` returns results from entire index
- [ ] `pm search --path src/api/` filters by path prefix
- [ ] `pm search --subproject auth-service` filters by subproject
- [ ] `--path` and `--subproject` together produce error
- [ ] JSON output includes `scope` object with mode and resolved values
- [ ] Human output shows "Searching in:" context line
- [ ] `--all` output shows `[subproject]` tags on results
- [ ] Unknown subproject produces helpful error with available list

---

## Files Modified

| File | Change |
|------|--------|
| `internal/cli/search.go` | New flags, scope resolution, output formatting |
| `internal/api/types.go` | Updated request/response types |
| `internal/daemon/handlers.go` | Handle scope in search request |
| `internal/db/search.go` | Add scope filtering to queries |
| `internal/cli/client.go` | GetSubprojectForPath method |

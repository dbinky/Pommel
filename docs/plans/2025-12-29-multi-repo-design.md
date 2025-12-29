# Pommel v0.2: Multi-Repo Support Design

**Version:** 0.2.0-draft
**Status:** Design Complete
**Builds On:** v0.1.x (Phases 1-7 complete)
**Branch:** dev-multi-repo

---

## Executive Summary

Pommel v0.2 extends the single-project semantic code search system to support:

1. **Multi-Instance Isolation** — Multiple Pommel daemons running simultaneously across unrelated projects, each with completely isolated search results
2. **Monorepo Support** — Single daemon managing multiple sub-projects within a monorepo, with intelligent search scoping

The key principle: isolation by default, cross-project search when explicitly requested.

---

## Use Cases

### Use Case 1: Multi-Instance Isolation

A developer has two Claude Code instances open:
- Instance A working on **Pommel** (`~/Repos/Pommel`)
- Instance B working on **psecsapi** (`~/Repos/psecsapi`)

Each instance runs `pm search` and receives results only from its own codebase. Pommel's authentication code never appears in psecsapi searches.

**Solution:** One daemon per project (unchanged from v0.1), with hash-based port assignment to avoid conflicts.

### Use Case 2: Monorepo with Sub-Projects

A monorepo contains multiple sub-projects:
```
/enterprise-app/
├── packages/frontend/      (package.json)
├── packages/shared/        (package.json)
├── services/api/           (go.mod)
├── services/auth/          (go.mod)
└── services/payments/      (go.mod)
```

When working in `services/auth/`, searches default to auth-service code. Use `--all` to find related patterns across the entire monorepo.

**Solution:** Unified index with sub-project metadata, intelligent search scoping based on current working directory.

---

## Architecture Overview

### Multi-Instance Isolation

- Each project runs its own daemon (unchanged from v0.1)
- Port derived from project path hash: `49152 + (hash(abs_path) % 16384)`
- Completely isolated databases — no cross-contamination
- CLI auto-discovers daemon port via hash calculation (no state file needed)

### Monorepo Support

- Single daemon at monorepo root watches everything
- Unified index stores all code with sub-project metadata on each chunk
- Sub-projects auto-detected via marker files (`.sln`, `go.mod`, `package.json`, etc.)
- Search defaults to current sub-project scope, expandable with `--path` or `--all`

**Key Principle:** From the daemon's perspective, a monorepo is just a project with rich sub-project metadata. The intelligence lives in search scoping, not in separate indexes.

---

## Data Model Changes

### Chunks Table (Updated)

```sql
chunks:
  id              TEXT PRIMARY KEY
  file_path       TEXT        -- relative to project root (unchanged)
  start_line      INTEGER
  end_line        INTEGER
  chunk_level     TEXT        -- method, class, file
  language        TEXT
  parent_chunk_id TEXT
  content_hash    TEXT
  last_modified   TIMESTAMP
  subproject_id   TEXT        -- NEW: nullable, e.g., "frontend", "auth-service"
  subproject_path TEXT        -- NEW: nullable, e.g., "services/auth"
```

### New Sub-Projects Table

```sql
subprojects:
  id              TEXT PRIMARY KEY  -- slug: "frontend", "auth-service"
  path            TEXT UNIQUE       -- relative path: "services/auth"
  name            TEXT              -- display name (optional)
  marker_file     TEXT              -- what triggered detection: "go.mod"
  language_hint   TEXT              -- primary language (nullable)
  auto_detected   BOOLEAN           -- true if found via marker, false if manual config
```

### Indexing Behavior

- When a file is indexed, Pommel determines its sub-project by finding the nearest ancestor directory with a marker file
- Files outside any sub-project have `NULL` subproject_id (root-level code)
- Sub-project assignment is recalculated on reindex

---

## Sub-Project Detection

### Marker Files

| Language/Platform | Markers |
|-------------------|---------|
| C# | `.sln`, `.csproj` |
| Go | `go.mod` |
| Node.js/JS/TS | `package.json` |
| Python | `pyproject.toml`, `setup.py` |
| Rust | `Cargo.toml` |
| Java | `pom.xml`, `build.gradle` |

### Marker Priority

When a directory contains multiple markers, use this priority:
1. `.sln` (solution encompasses projects)
2. `.csproj`, `go.mod`, `Cargo.toml`, `pom.xml`, `build.gradle` (compiled languages)
3. `package.json`, `pyproject.toml` (interpreted languages)

### Nested Markers

Innermost marker wins. A file at `/services/auth/handlers/validate.go` belongs to the sub-project defined by `/services/auth/go.mod`, not `/services/go.mod`.

---

## Search Behavior

### Scope Resolution Order

When `pm search "query"` is executed:

1. **Explicit `--all` flag** → Search entire index, no filtering
2. **Explicit `--path` flag** → Filter to specified path prefix
3. **Explicit `--subproject` flag** → Filter to named sub-project
4. **Auto-detect from cwd** → Find which sub-project contains current working directory
5. **Fallback** → If cwd is at project root or outside any sub-project, search everything

### CLI Scope Detection

```
User runs: cd /monorepo/services/auth && pm search "validation"

CLI logic:
1. Get cwd: /monorepo/services/auth/handlers
2. Get project root: /monorepo (from .pommel/)
3. Compute relative path: services/auth/handlers
4. Query daemon: "which subproject contains services/auth/handlers?"
5. Daemon returns: subproject_id="auth-service", subproject_path="services/auth"
6. Search with filter: WHERE subproject_id = 'auth-service'
```

### Search API Changes

```json
POST /search
{
  "query": "validation logic",
  "limit": 10,
  "level": ["method", "class"],
  "scope": {
    "mode": "auto" | "all" | "path" | "subproject",
    "value": null | "services/auth" | "auth-service"
  }
}
```

### CLI Flags

| Flag | Behavior |
|------|----------|
| (none) | Auto-detect sub-project from cwd |
| `--path src/api/` | Search only paths matching prefix |
| `--subproject auth-service` | Search specific sub-project by id |
| `--all` | Search entire index |

---

## Search Output Format

### JSON Response

```json
{
  "query": "user validation",
  "scope": {
    "mode": "auto",
    "subproject": "auth-service",
    "resolved_path": "services/auth"
  },
  "results": [
    {
      "id": "chunk-abc123",
      "file": "services/auth/handlers/validate.go",
      "start_line": 45,
      "end_line": 78,
      "level": "method",
      "language": "go",
      "name": "ValidateUserInput",
      "score": 0.92,
      "content": "func ValidateUserInput(req *Request) error {...}",
      "subproject": {
        "id": "auth-service",
        "path": "services/auth",
        "name": "Auth Service"
      },
      "parent": {
        "id": "chunk-parent456",
        "name": "handlers/validate.go",
        "level": "file"
      }
    }
  ],
  "total_results": 12,
  "search_time_ms": 38
}
```

### Human-Readable Output

```
$ pm search "validation"

Searching in: auth-service (services/auth)

  services/auth/handlers/validate.go:45-78 (method)
  ValidateUserInput                                    0.92
  func ValidateUserInput(req *Request) error {...}

  services/auth/models/user.go:12-35 (method)
  Validate                                             0.87
  func (u *User) Validate() error {...}

12 results (showing top 10) • 38ms
```

### Cross-Project Results (with `--all`)

```
$ pm search "validation" --all

Searching in: all sub-projects

  [auth-service] services/auth/handlers/validate.go:45-78
  ValidateUserInput                                    0.92

  [frontend] packages/frontend/src/utils/validate.ts:10-25
  validateForm                                         0.89

  [backend-api] services/api/middleware/validate.go:33-67
  ValidateRequest                                      0.85

Results from 3 sub-projects • 52ms
```

---

## Init and Configuration

### Enhanced `pm init` Flow

```
$ cd /my-monorepo
$ pm init

Scanning for project markers...

Found 4 sub-projects:
  • frontend        (packages/frontend)     package.json
  • backend-api     (services/api)          go.mod
  • auth-service    (services/auth)         go.mod
  • shared          (packages/shared)       package.json

Initialize as monorepo with these sub-projects? [Y/n] y

✓ Created .pommel/config.yaml
✓ Initialized database
✓ Registered 4 sub-projects

Run 'pm start' to begin indexing.
```

### Init Flags

| Flag | Behavior |
|------|----------|
| `--auto` | Auto-detect languages (unchanged) |
| `--claude` | Add Pommel instructions to CLAUDE.md — in monorepo, updates root + each sub-project |
| `--start` | Start daemon after init (unchanged) |
| `--monorepo` | Assume monorepo, no prompt |
| `--no-monorepo` | Skip sub-project detection entirely |

### `--claude` in Monorepo Mode

```
$ pm init --monorepo --claude

Found 4 sub-projects...
✓ Created .pommel/config.yaml
✓ Initialized database
✓ Updated CLAUDE.md (monorepo root)
✓ Updated packages/frontend/CLAUDE.md
✓ Updated services/api/CLAUDE.md
✓ Updated services/auth/CLAUDE.md
✓ Created packages/shared/CLAUDE.md

Run 'pm start' to begin indexing.
```

Each CLAUDE.md gets instructions appropriate to its location.

### Configuration Schema

```yaml
# .pommel/config.yaml
version: 2

# Existing config (unchanged)
chunk_levels: [method, class, file]
include_patterns: ["**/*.cs", "**/*.go", "**/*.ts"]
exclude_patterns: ["**/node_modules/**", "**/bin/**"]

daemon:
  host: "127.0.0.1"
  port: null  # null = use hash-based port; set value to override

# New: sub-project configuration
subprojects:
  auto_detect: true          # scan for marker files on startup

  markers:                   # marker files to look for
    - "*.sln"
    - "*.csproj"
    - "go.mod"
    - "package.json"
    - "pyproject.toml"
    - "setup.py"
    - "Cargo.toml"
    - "pom.xml"
    - "build.gradle"

  # Manual overrides/additions
  projects:
    - id: auth-service        # override auto-detected name
      path: services/auth
      name: "Auth Service"

    - id: legacy-scripts      # define sub-project with no marker
      path: tools/scripts
      name: "Legacy Scripts"

  # Exclude from sub-project detection (still indexed, just no subproject_id)
  exclude:
    - "docs"
    - "scripts"
```

---

## Port Hashing & Daemon Discovery

### Port Calculation

```go
func DaemonPort(projectRoot string) int {
    absPath, _ := filepath.Abs(projectRoot)
    h := fnv.New32a()
    h.Write([]byte(absPath))
    return 49152 + int(h.Sum32()%16384)
}
```

- **Deterministic:** Same path always yields same port
- **Range:** 49152–65535 (IANA dynamic/private ports)
- **No state file needed** for port discovery

### CLI Discovery Flow

```
1. CLI invoked in /monorepo/services/auth
2. Walk up to find .pommel/ → /monorepo/.pommel/
3. Compute port: DaemonPort("/monorepo") → 52847
4. Connect to localhost:52847
```

### Collision Handling

Hash collisions are rare (~0.006% chance with 100 projects). If daemon fails to bind:

1. Log error: `Port 52847 in use (collision with another Pommel project?)`
2. Suggest: `Add 'daemon.port: 52850' to .pommel/config.yaml to override`

Config override takes precedence over hash calculation.

### Health Check

CLI verifies it's talking to the right daemon:

```json
GET /health
{
  "project_root": "/monorepo",
  "version": "0.2.0"
}
```

If `project_root` doesn't match, CLI errors: `Daemon at port 52847 is serving a different project.`

---

## Daemon Startup & Runtime Detection

### Startup Sequence

```
$ pm start

Starting Pommel daemon...
  ✓ Config loaded
  ✓ Port 52847 available
  ✓ Database connected (schema v2)
  ✓ Ollama reachable (jina-embeddings-v2-base-code)

Scanning for sub-project changes...
  • Found new sub-project: payments (services/payments/go.mod)
  ✓ Registered 1 new sub-project

Scanning for file changes since last run (2025-12-29 10:30:00)...
  • 12 files modified
  • 3 files added
  • 1 file deleted
  ✓ Queued 16 files for re-indexing

Daemon running on localhost:52847
Indexing 16 files...
```

### Startup Tasks (in order)

| Step | Action |
|------|--------|
| 1 | Load config, validate schema |
| 2 | Bind port (hash-based or config override) |
| 3 | Verify Ollama connection |
| 4 | Scan for new/removed sub-projects (compare markers vs. database) |
| 5 | Full filesystem scan — compare file mtimes against `last_modified` in database |
| 6 | Queue changed/new files for indexing |
| 7 | Begin serving API (searches work immediately against existing index) |
| 8 | Process index queue in background |

### Runtime Sub-Project Detection

When file watcher sees a new marker file:

```
File created: services/notifications/go.mod

Watcher detects marker file:
  → Register new sub-project "notifications" (services/notifications)
  → Scan services/notifications/ for source files
  → Queue all found files for indexing
  → Update subproject_id on any already-indexed files in that path
```

### Marker File Events

| Event | Action |
|-------|--------|
| Marker created | Register new sub-project, index its files |
| Marker deleted | Log warning, keep sub-project (may be temporary); remove on `--rescan-subprojects` |
| Marker moved | Treat as delete + create |

### State Tracking

```json
// .pommel/state.json
{
  "last_scan": "2025-12-29T10:30:00Z",
  "daemon_pid": 12345,
  "indexed_files": 342,
  "subprojects_hash": "abc123"
}
```

---

## Error Handling & Edge Cases

### Port & Daemon Errors

| Scenario | Error Message | Resolution |
|----------|---------------|------------|
| Port collision on startup | `Port 52847 in use. Another Pommel project may have the same port hash.` | Add `daemon.port: <free_port>` to config |
| Daemon not running | `Daemon not running. Start with 'pm start'` | Standard behavior (unchanged) |
| Wrong daemon at port | `Daemon at port 52847 serves '/other/project', not '/monorepo'` | Kill stale daemon, restart correct one |
| Daemon unreachable | `Cannot connect to daemon at localhost:52847` | Check if daemon crashed, restart |

### Sub-Project Edge Cases

| Scenario | Behavior |
|----------|----------|
| File outside any sub-project | Indexed with `subproject_id = NULL`, included in `--all` and root-level searches |
| Nested markers | Innermost marker wins — file belongs to most specific sub-project |
| Marker file deleted | Sub-project remains until explicit removal or `pm reindex --rescan-subprojects` |
| Marker file added | Detected immediately by file watcher; sub-project registered, files indexed |
| Sub-project directory renamed | Old chunks orphaned; `pm reindex` corrects |

### Search Scope Errors

| Scenario | Error Message |
|----------|---------------|
| `--subproject foo` but "foo" doesn't exist | `Unknown sub-project 'foo'. Available: auth-service, frontend, backend-api` |
| `--path` and `--subproject` both specified | `Cannot use --path and --subproject together. Use one or the other.` |
| `--path` and `--all` both specified | `--all searches everything; --path ignored.` (warning, not error) |
| Auto-detect fails (cwd outside project) | `Current directory is outside project root. Use --all or specify --path.` |

### Reindex Commands

```bash
# Normal reindex (keeps sub-project assignments, re-scans for new sub-projects)
pm reindex

# Force full rebuild (drops and recreates everything)
pm reindex --full

# Rescan sub-projects only (no file reindexing)
pm reindex --rescan-subprojects
```

---

## Implementation Phases

*Phases 1-7 complete (v0.1.x foundation)*

| Phase | Scope | Effort |
|-------|-------|--------|
| **Phase 8: Port Hashing** | Replace fixed port 7420 with hash-based calculation (49152-65535), add `/health` endpoint returning `project_root` | Small |
| **Phase 9: Schema Changes** | Add `subproject_id`, `subproject_path` to chunks; create `subprojects` table; drop existing database | Small |
| **Phase 10: Sub-Project Detection** | Marker file scanning, config schema for `subprojects:` block, registration logic | Medium |
| **Phase 11: Startup Scanning** | Full filesystem scan on start, compare mtimes, detect new/changed files, queue for indexing | Medium |
| **Phase 12: Runtime Detection** | Watch for marker file creation/deletion, dynamic sub-project registration | Small |
| **Phase 13: Init Changes** | Monorepo detection prompt, `--monorepo`/`--no-monorepo` flags, multi-CLAUDE.md updates | Medium |
| **Phase 14: Search Scoping** | Auto-detect cwd scope, `--all`/`--path`/`--subproject` flags, updated JSON output | Medium |

### Implementation Order

8 → 9 → 10 → 11 → 12 → 13 → 14

**Rationale:** Infrastructure first (ports, schema, detection, scanning), then user-facing changes (init, search). Search scoping comes last since it depends on everything else.

---

## Success Criteria

Pommel v0.2 will be considered successful when:

1. **Multi-Instance Isolation**
   - Two Pommel daemons can run simultaneously on different projects without port conflicts
   - Search results are completely isolated between projects

2. **Monorepo Detection**
   - `pm init` correctly identifies monorepos with 3+ sub-projects
   - Marker files are detected for all supported languages

3. **Search Scoping**
   - Auto-detection correctly scopes searches to current sub-project
   - `--all` returns results from all sub-projects
   - `--path` and `--subproject` work as expected

4. **Startup Scanning**
   - Daemon detects and indexes file changes on restart
   - New sub-projects are discovered without manual intervention

5. **Runtime Detection**
   - Creating a new `go.mod` triggers sub-project registration within seconds
   - New sub-project files are indexed automatically

---

## Document History

| Version | Date | Author | Changes |
|---------|------|--------|---------|
| 0.2.0-draft | 2025-12-29 | Ryan + Claude | Initial multi-repo design |

---

*This document is intended for use with Claude Code for implementation planning.*

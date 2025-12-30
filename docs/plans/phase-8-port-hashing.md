# Phase 8: Port Hashing

**Status:** Not Started
**Effort:** Small
**Dependencies:** None (builds on v0.1.x foundation)

---

## Objective

Replace the fixed daemon port (7420) with a deterministic hash-based port calculation. This enables multiple Pommel daemons to run simultaneously across different projects without port conflicts.

---

## Requirements

1. Port derived from absolute project path using FNV-1a hash
2. Port range: 49152–65535 (IANA dynamic/private ports)
3. Config override takes precedence over hash calculation
4. `/health` endpoint returns `project_root` for validation
5. CLI validates it's connected to the correct daemon

---

## Implementation Tasks

### 8.1 Create Port Calculation Function

**File:** `internal/daemon/port.go` (new)

```go
package daemon

import (
    "hash/fnv"
    "path/filepath"
)

const (
    PortRangeStart = 49152
    PortRangeSize  = 16384 // 49152 to 65535
)

// CalculatePort returns a deterministic port for the given project root.
// The port is derived from the absolute path hash, falling in range 49152-65535.
func CalculatePort(projectRoot string) (int, error) {
    absPath, err := filepath.Abs(projectRoot)
    if err != nil {
        return 0, err
    }

    h := fnv.New32a()
    h.Write([]byte(absPath))

    port := PortRangeStart + int(h.Sum32()%uint32(PortRangeSize))
    return port, nil
}
```

**Tests:** `internal/daemon/port_test.go`
- Test determinism (same path → same port)
- Test range bounds (always 49152-65535)
- Test different paths yield different ports (probabilistic)

### 8.2 Update Config Schema

**File:** `internal/config/config.go`

Update `DaemonConfig` struct:

```go
type DaemonConfig struct {
    Host string `yaml:"host" mapstructure:"host"`
    Port *int   `yaml:"port" mapstructure:"port"` // nil = use hash, non-nil = override
}
```

**Behavior:**
- If `Port` is nil (not set in config), use `CalculatePort()`
- If `Port` is set, use that value directly

### 8.3 Update Daemon Startup

**File:** `internal/daemon/daemon.go`

Modify `Start()` or initialization:

```go
func (d *Daemon) determinePort() (int, error) {
    // Config override takes precedence
    if d.config.Daemon.Port != nil {
        return *d.config.Daemon.Port, nil
    }

    // Calculate from project root
    return CalculatePort(d.projectRoot)
}
```

Update logging to show which port is being used and why:
- `Starting daemon on port 52847 (calculated from project path)`
- `Starting daemon on port 8080 (config override)`

### 8.4 Add /health Endpoint

**File:** `internal/daemon/handlers.go` or `internal/api/handlers.go`

```go
type HealthResponse struct {
    ProjectRoot string `json:"project_root"`
    Version     string `json:"version"`
    Status      string `json:"status"`
    Uptime      int64  `json:"uptime_seconds"`
}

func (d *Daemon) handleHealth(w http.ResponseWriter, r *http.Request) {
    resp := HealthResponse{
        ProjectRoot: d.projectRoot,
        Version:     version.Version,
        Status:      "healthy",
        Uptime:      int64(time.Since(d.startTime).Seconds()),
    }
    json.NewEncoder(w).Encode(resp)
}
```

Register route: `GET /health`

### 8.5 Update CLI Client

**File:** `internal/cli/client.go`

Update `getDaemonURL()` or equivalent:

```go
func getDaemonURL(projectRoot string, cfg *config.Config) (string, error) {
    var port int

    if cfg.Daemon.Port != nil {
        port = *cfg.Daemon.Port
    } else {
        var err error
        port, err = daemon.CalculatePort(projectRoot)
        if err != nil {
            return "", err
        }
    }

    return fmt.Sprintf("http://%s:%d", cfg.Daemon.Host, port), nil
}
```

### 8.6 Add Daemon Validation

**File:** `internal/cli/client.go`

Before making requests, validate we're talking to the right daemon:

```go
func (c *Client) validateDaemon() error {
    resp, err := c.httpClient.Get(c.baseURL + "/health")
    if err != nil {
        return fmt.Errorf("cannot connect to daemon: %w", err)
    }
    defer resp.Body.Close()

    var health HealthResponse
    if err := json.NewDecoder(resp.Body).Decode(&health); err != nil {
        return fmt.Errorf("invalid health response: %w", err)
    }

    if health.ProjectRoot != c.expectedProjectRoot {
        return fmt.Errorf("daemon at port %d serves '%s', not '%s'",
            c.port, health.ProjectRoot, c.expectedProjectRoot)
    }

    return nil
}
```

### 8.7 Handle Port Collision

**File:** `internal/daemon/daemon.go`

When binding fails, provide helpful error:

```go
listener, err := net.Listen("tcp", addr)
if err != nil {
    if isAddrInUse(err) {
        return fmt.Errorf("port %d in use (possible hash collision with another Pommel project). "+
            "Add 'daemon.port: <free_port>' to .pommel/config.yaml to override", port)
    }
    return err
}
```

---

## Testing

### Unit Tests

| Test | Description |
|------|-------------|
| `TestCalculatePort_Deterministic` | Same path always returns same port |
| `TestCalculatePort_InRange` | Port always between 49152-65535 |
| `TestCalculatePort_DifferentPaths` | Different paths yield different ports |
| `TestDeterminePort_ConfigOverride` | Config port takes precedence |
| `TestDeterminePort_HashFallback` | Uses hash when config is nil |

### Integration Tests

| Test | Description |
|------|-------------|
| `TestHealthEndpoint` | Returns correct project_root and version |
| `TestClientValidation_Correct` | Validation passes for matching daemon |
| `TestClientValidation_Wrong` | Validation fails for mismatched daemon |
| `TestPortCollision_ErrorMessage` | Helpful message on port conflict |

---

## Acceptance Criteria

- [ ] `pm start` uses hash-based port when no config override
- [ ] `pm start` uses config port when `daemon.port` is set
- [ ] `GET /health` returns `project_root` and `version`
- [ ] CLI validates daemon before making requests
- [ ] Helpful error message on port collision
- [ ] Two projects can run daemons simultaneously

---

## Files Modified

| File | Change |
|------|--------|
| `internal/daemon/port.go` | New file: port calculation |
| `internal/daemon/port_test.go` | New file: port tests |
| `internal/config/config.go` | Update DaemonConfig.Port to pointer |
| `internal/daemon/daemon.go` | Use calculated port, collision handling |
| `internal/daemon/handlers.go` | Add /health endpoint |
| `internal/cli/client.go` | Calculate port, validate daemon |

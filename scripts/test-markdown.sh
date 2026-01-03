#!/bin/bash
# Test Pommel markdown support on a markdown-heavy codebase
# Usage: ./scripts/test-markdown.sh /path/to/markdown-project

set -e

PROJECT_DIR="${1:-.}"
PM_BIN="${PM_BIN:-pm}"
POMMELD_BIN="${POMMELD_BIN:-pommeld}"

# Colors
RED='\033[0;31m'
GREEN='\033[0;32m'
YELLOW='\033[1;33m'
NC='\033[0m' # No Color

info() { echo -e "${GREEN}[INFO]${NC} $1"; }
warn() { echo -e "${YELLOW}[WARN]${NC} $1"; }
error() { echo -e "${RED}[ERROR]${NC} $1"; }

# Validate project directory
if [ ! -d "$PROJECT_DIR" ]; then
    error "Directory not found: $PROJECT_DIR"
    exit 1
fi

cd "$PROJECT_DIR"
info "Testing markdown support in: $(pwd)"

# Count markdown files
MD_COUNT=$(find . -name "*.md" -not -path "./.pommel/*" -not -path "./.git/*" 2>/dev/null | wc -l | tr -d ' ')
info "Found $MD_COUNT markdown files"

if [ "$MD_COUNT" -eq 0 ]; then
    warn "No markdown files found in this directory"
    exit 1
fi

# Step 1: Check Ollama
info "Checking Ollama status..."
if ! curl -s http://localhost:11434/api/tags >/dev/null 2>&1; then
    error "Ollama not running. Start with: ollama serve"
    exit 1
fi

# Check for embedding model
if ! curl -s http://localhost:11434/api/tags | grep -q "jina-embeddings"; then
    warn "Embedding model not found. Pulling..."
    ollama pull unclemusclez/jina-embeddings-v2-base-code
fi
info "Ollama ready"

# Step 2: Clean previous index
if [ -d ".pommel" ]; then
    warn "Removing existing .pommel directory..."
    $PM_BIN stop 2>/dev/null || true
    rm -rf .pommel
fi

# Step 3: Initialize with markdown support
info "Initializing Pommel..."
$PM_BIN init

# Step 4: Configure for markdown-only (or add markdown to existing)
info "Configuring for markdown files..."
cat > .pommel/config.yaml << 'YAML'
chunk_levels:
    - method
    - class
    - file
daemon:
    host: 127.0.0.1
    port: null
    log_level: info
embedding:
    model: unclemusclez/jina-embeddings-v2-base-code
    ollama_url: http://localhost:11434
    batch_size: 8
    cache_size: 1000
exclude_patterns:
    - '**/node_modules/**'
    - '**/vendor/**'
    - '**/.git/**'
    - '**/.pommel/**'
include_patterns:
    - '**/*.md'
    - '**/*.markdown'
search:
    default_limit: 10
    default_levels:
        - method
        - class
    hybrid:
        enabled: true
        rrf_k: 60
        vector_weight: 0.7
        keyword_weight: 0.3
    reranker:
        enabled: true
        model: ""
        timeout_ms: 2000
        fallback: heuristic
        candidates: 20
version: 1
watcher:
    debounce_ms: 500
    max_file_size: 1048576
YAML

# Step 5: Start daemon and wait for indexing
info "Starting daemon..."
$PM_BIN start

info "Waiting for initial indexing (this may take a while for large codebases)..."
sleep 5

# Poll until indexing is complete
MAX_WAIT=300  # 5 minutes
WAITED=0
while true; do
    STATUS=$($PM_BIN status 2>/dev/null || echo "error")
    if echo "$STATUS" | grep -q "Ready"; then
        break
    fi
    if echo "$STATUS" | grep -q "Indexing"; then
        INDEXED=$(echo "$STATUS" | grep -oE "Files:\s+[0-9]+" | grep -oE "[0-9]+" || echo "0")
        echo -ne "\r[INFO] Indexing... Files: $INDEXED"
    fi
    sleep 2
    WAITED=$((WAITED + 2))
    if [ $WAITED -ge $MAX_WAIT ]; then
        error "Indexing timeout after ${MAX_WAIT}s"
        exit 1
    fi
done
echo ""

# Step 6: Show status
info "Index complete!"
$PM_BIN status

# Step 7: Run test searches
echo ""
info "Running test searches..."
echo ""

echo "=== Search 1: 'getting started installation' ==="
$PM_BIN search "getting started installation" --limit 3
echo ""

echo "=== Search 2: 'API reference documentation' ==="
$PM_BIN search "API reference documentation" --limit 3
echo ""

echo "=== Search 3: 'configuration options settings' ==="
$PM_BIN search "configuration options settings" --limit 3
echo ""

echo "=== Search 4: 'troubleshooting common errors' ==="
$PM_BIN search "troubleshooting common errors" --limit 3
echo ""

# Step 8: Interactive mode hint
info "Setup complete! Try your own searches:"
echo "  $PM_BIN search \"your query here\" --limit 5"
echo ""
echo "Useful commands:"
echo "  $PM_BIN status      # Check index status"
echo "  $PM_BIN reindex     # Force reindex"
echo "  $PM_BIN stop        # Stop daemon"

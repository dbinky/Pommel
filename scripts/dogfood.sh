#!/usr/bin/env bash
#
# Pommel Dogfooding Script
# Tests Pommel on its own codebase to validate functionality
#
# Usage: ./scripts/dogfood.sh [--json] [--skip-cleanup] [--results-file FILE]
#
# Exit codes:
#   0 - All tests passed
#   1 - Build failed
#   2 - Ollama not available (skipped gracefully)
#   3 - Daemon failed to start
#   4 - Search tests failed
#   5 - Cleanup failed

set -euo pipefail

# Configuration
SCRIPT_DIR="$(cd "$(dirname "${BASH_SOURCE[0]}")" && pwd)"
PROJECT_ROOT="$(cd "${SCRIPT_DIR}/.." && pwd)"
POMMEL_DIR="${PROJECT_ROOT}/.pommel"
PM_BIN="${PROJECT_ROOT}/bin/pm"
POMMELD_BIN="${PROJECT_ROOT}/bin/pommeld"
RESULTS_FILE=""
JSON_OUTPUT=false
SKIP_CLEANUP=false
MAX_INDEX_WAIT=120  # seconds
DAEMON_START_WAIT=5  # seconds

# Parse arguments
while [[ $# -gt 0 ]]; do
    case $1 in
        --json)
            JSON_OUTPUT=true
            shift
            ;;
        --skip-cleanup)
            SKIP_CLEANUP=true
            shift
            ;;
        --results-file)
            RESULTS_FILE="$2"
            shift 2
            ;;
        *)
            echo "Unknown option: $1" >&2
            exit 1
            ;;
    esac
done

# Initialize results structure
declare -A TEST_RESULTS
INDEXING_TIME=0
TOTAL_FILES=0
TOTAL_CHUNKS=0
DAEMON_PID=""
ISSUES_FOUND=()
START_TIME=$(date +%s)

# Logging functions
log_info() {
    if [[ "${JSON_OUTPUT}" == "false" ]]; then
        echo "[INFO] $1"
    fi
}

log_error() {
    if [[ "${JSON_OUTPUT}" == "false" ]]; then
        echo "[ERROR] $1" >&2
    fi
    ISSUES_FOUND+=("$1")
}

log_success() {
    if [[ "${JSON_OUTPUT}" == "false" ]]; then
        echo "[OK] $1"
    fi
}

# Cleanup function
cleanup() {
    local exit_code=$?

    log_info "Cleaning up..."

    # Stop daemon if running
    if [[ -n "${DAEMON_PID}" ]] && kill -0 "${DAEMON_PID}" 2>/dev/null; then
        log_info "Stopping daemon (PID: ${DAEMON_PID})..."
        "${PM_BIN}" stop --project "${PROJECT_ROOT}" 2>/dev/null || kill "${DAEMON_PID}" 2>/dev/null || true
        sleep 1
    fi

    # Remove .pommel directory unless skipped
    if [[ "${SKIP_CLEANUP}" == "false" ]] && [[ -d "${POMMEL_DIR}" ]]; then
        log_info "Removing ${POMMEL_DIR}..."
        rm -rf "${POMMEL_DIR}"
    fi

    exit "${exit_code}"
}

trap cleanup EXIT

# Output JSON results
output_json_results() {
    local end_time=$(date +%s)
    local total_time=$((end_time - START_TIME))
    local tests_passed=0
    local tests_failed=0

    for key in "${!TEST_RESULTS[@]}"; do
        if [[ "${TEST_RESULTS[$key]}" == "passed" ]]; then
            ((tests_passed++))
        else
            ((tests_failed++))
        fi
    done

    local search_results="["
    local first=true
    for key in "${!TEST_RESULTS[@]}"; do
        if [[ "${first}" == "true" ]]; then
            first=false
        else
            search_results+=","
        fi
        search_results+="{\"query\":\"${key}\",\"status\":\"${TEST_RESULTS[$key]}\"}"
    done
    search_results+="]"

    local issues_json="["
    first=true
    for issue in "${ISSUES_FOUND[@]}"; do
        if [[ "${first}" == "true" ]]; then
            first=false
        else
            issues_json+=","
        fi
        issues_json+="\"${issue}\""
    done
    issues_json+="]"

    cat <<EOF
{
    "status": "$( [[ ${tests_failed} -eq 0 ]] && echo "success" || echo "failure" )",
    "indexing": {
        "total_files": ${TOTAL_FILES},
        "total_chunks": ${TOTAL_CHUNKS},
        "indexing_time_seconds": ${INDEXING_TIME}
    },
    "search_tests": {
        "total": $((tests_passed + tests_failed)),
        "passed": ${tests_passed},
        "failed": ${tests_failed},
        "results": ${search_results}
    },
    "total_time_seconds": ${total_time},
    "issues": ${issues_json}
}
EOF
}

# Step 1: Build Pommel binaries
log_info "Building Pommel binaries..."
cd "${PROJECT_ROOT}"
if ! make build > /dev/null 2>&1; then
    log_error "Failed to build Pommel binaries"
    exit 1
fi

if [[ ! -x "${PM_BIN}" ]] || [[ ! -x "${POMMELD_BIN}" ]]; then
    log_error "Build completed but binaries not found or not executable"
    exit 1
fi
log_success "Binaries built successfully"

# Step 2: Check if Ollama is running
log_info "Checking Ollama availability..."
OLLAMA_URL="${OLLAMA_URL:-http://localhost:11434}"

if ! curl -sf "${OLLAMA_URL}" > /dev/null 2>&1; then
    log_info "Ollama is not running at ${OLLAMA_URL}"
    log_info "Skipping dogfooding tests (Ollama required)"

    if [[ "${JSON_OUTPUT}" == "true" ]]; then
        cat <<EOF
{
    "status": "skipped",
    "reason": "Ollama not available at ${OLLAMA_URL}",
    "suggestion": "Start Ollama with 'ollama serve' and ensure the embedding model is installed"
}
EOF
    fi
    exit 2
fi

# Check if embedding model is available
if ! curl -sf "${OLLAMA_URL}/api/tags" | grep -q "jina-embeddings"; then
    log_info "Jina embeddings model not found in Ollama"
    log_info "Install with: ollama pull unclemusclez/jina-embeddings-v2-base-code"

    if [[ "${JSON_OUTPUT}" == "true" ]]; then
        cat <<EOF
{
    "status": "skipped",
    "reason": "Embedding model not installed",
    "suggestion": "Run: ollama pull unclemusclez/jina-embeddings-v2-base-code"
}
EOF
    fi
    exit 2
fi
log_success "Ollama is available with embedding model"

# Step 3: Clean up existing .pommel directory
if [[ -d "${POMMEL_DIR}" ]]; then
    log_info "Removing existing .pommel directory..."
    rm -rf "${POMMEL_DIR}"
fi

# Step 4: Initialize Pommel
log_info "Initializing Pommel in ${PROJECT_ROOT}..."
if ! "${PM_BIN}" init --project "${PROJECT_ROOT}" > /dev/null 2>&1; then
    log_error "Failed to initialize Pommel"
    exit 1
fi
log_success "Pommel initialized"

# Step 5: Start the daemon
log_info "Starting Pommel daemon..."
INDEX_START=$(date +%s)

if ! "${PM_BIN}" start --project "${PROJECT_ROOT}" > /dev/null 2>&1; then
    log_error "Failed to start Pommel daemon"
    exit 3
fi

# Wait for daemon to start
sleep "${DAEMON_START_WAIT}"

# Get daemon PID
if [[ -f "${POMMEL_DIR}/daemon.pid" ]]; then
    DAEMON_PID=$(cat "${POMMEL_DIR}/daemon.pid")
    log_success "Daemon started (PID: ${DAEMON_PID})"
else
    log_error "Daemon started but PID file not found"
    exit 3
fi

# Step 6: Wait for indexing to complete
log_info "Waiting for indexing to complete (max ${MAX_INDEX_WAIT}s)..."
wait_count=0
while [[ ${wait_count} -lt ${MAX_INDEX_WAIT} ]]; do
    status_json=$("${PM_BIN}" status --json --project "${PROJECT_ROOT}" 2>/dev/null || echo '{}')

    # Extract pending changes and indexing status
    pending=$(echo "${status_json}" | grep -o '"pending_changes":[0-9]*' | grep -o '[0-9]*' || echo "-1")
    is_indexing=$(echo "${status_json}" | grep -o '"is_indexing":[a-z]*' | grep -o 'true\|false' || echo "unknown")

    if [[ "${pending}" == "0" ]] && [[ "${is_indexing}" == "false" ]]; then
        break
    fi

    sleep 2
    ((wait_count+=2))

    if [[ "${JSON_OUTPUT}" == "false" ]] && [[ $((wait_count % 10)) -eq 0 ]]; then
        echo "  Still indexing... (${wait_count}s elapsed, pending: ${pending})"
    fi
done

INDEX_END=$(date +%s)
INDEXING_TIME=$((INDEX_END - INDEX_START))

if [[ ${wait_count} -ge ${MAX_INDEX_WAIT} ]]; then
    log_error "Indexing did not complete within ${MAX_INDEX_WAIT} seconds"
    exit 4
fi

log_success "Indexing completed in ${INDEXING_TIME} seconds"

# Get final statistics
final_status=$("${PM_BIN}" status --json --project "${PROJECT_ROOT}" 2>/dev/null || echo '{}')
TOTAL_FILES=$(echo "${final_status}" | grep -o '"total_files":[0-9]*' | grep -o '[0-9]*' || echo "0")
TOTAL_CHUNKS=$(echo "${final_status}" | grep -o '"total_chunks":[0-9]*' | grep -o '[0-9]*' || echo "0")

log_info "Indexed ${TOTAL_FILES} files into ${TOTAL_CHUNKS} chunks"

# Step 7: Run test searches
log_info "Running search tests..."

# Test search function
run_search_test() {
    local query="$1"
    local expected_patterns="$2"  # Comma-separated list of patterns
    local test_name="${query}"

    log_info "Testing: \"${query}\""

    search_result=$("${PM_BIN}" search "${query}" --json --limit 10 --project "${PROJECT_ROOT}" 2>&1)

    if [[ $? -ne 0 ]]; then
        log_error "Search failed for query: ${query}"
        TEST_RESULTS["${test_name}"]="failed"
        return 1
    fi

    # Check if results contain expected patterns
    local all_found=true
    IFS=',' read -ra patterns <<< "${expected_patterns}"
    for pattern in "${patterns[@]}"; do
        pattern=$(echo "${pattern}" | xargs)  # Trim whitespace
        if ! echo "${search_result}" | grep -qi "${pattern}"; then
            log_error "Expected '${pattern}' in results for query: ${query}"
            all_found=false
            break
        fi
    done

    if [[ "${all_found}" == "true" ]]; then
        log_success "Query '${query}' returned expected results"
        TEST_RESULTS["${test_name}"]="passed"
        return 0
    else
        TEST_RESULTS["${test_name}"]="failed"
        return 1
    fi
}

# Run the test searches
test_failures=0

# Test 1: Embedding generation
if ! run_search_test "embedding generation" "ollama,embed"; then
    ((test_failures++))
fi

# Test 2: File watcher debounce
if ! run_search_test "file watcher debounce" "watcher,debounce"; then
    ((test_failures++))
fi

# Test 3: Vector search
if ! run_search_test "vector search" "search"; then
    ((test_failures++))
fi

# Test 4: CLI command
if ! run_search_test "CLI command" "cobra,command"; then
    ((test_failures++))
fi

# Additional tests for comprehensive coverage
# Test 5: Chunking logic
if ! run_search_test "code chunking" "chunk"; then
    ((test_failures++))
fi

# Test 6: Configuration handling
if ! run_search_test "configuration loading" "config"; then
    ((test_failures++))
fi

# Test 7: API handlers
if ! run_search_test "HTTP API handler" "handler,router"; then
    ((test_failures++))
fi

# Test 8: Database operations
if ! run_search_test "database schema" "sqlite,schema"; then
    ((test_failures++))
fi

# Step 8: Output results
log_info "Generating results..."

if [[ "${JSON_OUTPUT}" == "true" ]]; then
    json_output=$(output_json_results)
    echo "${json_output}"

    if [[ -n "${RESULTS_FILE}" ]]; then
        echo "${json_output}" > "${RESULTS_FILE}"
    fi
else
    echo ""
    echo "=========================================="
    echo "  Pommel Dogfooding Results"
    echo "=========================================="
    echo ""
    echo "Indexing Statistics:"
    echo "  - Total files: ${TOTAL_FILES}"
    echo "  - Total chunks: ${TOTAL_CHUNKS}"
    echo "  - Indexing time: ${INDEXING_TIME}s"
    echo ""
    echo "Search Test Results:"
    tests_passed=0
    tests_failed=0
    for key in "${!TEST_RESULTS[@]}"; do
        if [[ "${TEST_RESULTS[$key]}" == "passed" ]]; then
            echo "  [PASS] ${key}"
            ((tests_passed++))
        else
            echo "  [FAIL] ${key}"
            ((tests_failed++))
        fi
    done
    echo ""
    echo "Summary: ${tests_passed} passed, ${tests_failed} failed"

    if [[ ${#ISSUES_FOUND[@]} -gt 0 ]]; then
        echo ""
        echo "Issues Found:"
        for issue in "${ISSUES_FOUND[@]}"; do
            echo "  - ${issue}"
        done
    fi
    echo ""
    echo "=========================================="
fi

# Write results to file if specified (non-JSON mode)
if [[ -n "${RESULTS_FILE}" ]] && [[ "${JSON_OUTPUT}" == "false" ]]; then
    {
        echo "# Pommel Dogfooding Results"
        echo ""
        echo "Date: $(date -u +%Y-%m-%dT%H:%M:%SZ)"
        echo ""
        echo "## Indexing Statistics"
        echo ""
        echo "| Metric | Value |"
        echo "|--------|-------|"
        echo "| Total Files | ${TOTAL_FILES} |"
        echo "| Total Chunks | ${TOTAL_CHUNKS} |"
        echo "| Indexing Time | ${INDEXING_TIME}s |"
        echo ""
        echo "## Search Test Results"
        echo ""
        echo "| Query | Status |"
        echo "|-------|--------|"
        for key in "${!TEST_RESULTS[@]}"; do
            echo "| ${key} | ${TEST_RESULTS[$key]} |"
        done
        echo ""
        if [[ ${#ISSUES_FOUND[@]} -gt 0 ]]; then
            echo "## Issues Found"
            echo ""
            for issue in "${ISSUES_FOUND[@]}"; do
                echo "- ${issue}"
            done
        fi
    } > "${RESULTS_FILE}"
fi

# Exit with appropriate code
if [[ ${test_failures} -gt 0 ]]; then
    exit 4
fi

log_success "All dogfooding tests passed!"
exit 0

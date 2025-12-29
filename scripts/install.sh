#!/usr/bin/env bash
#
# Pommel Installation Script
#
# Usage:
#   curl -fsSL https://raw.githubusercontent.com/dbinky/Pommel/main/scripts/install.sh | bash
#
# Or clone and run locally:
#   ./scripts/install.sh
#
set -e

# Colors for output
RED='\033[0;31m'
GREEN='\033[0;32m'
YELLOW='\033[1;33m'
BLUE='\033[0;34m'
NC='\033[0m' # No Color

info() { echo -e "${BLUE}[INFO]${NC} $1"; }
success() { echo -e "${GREEN}[OK]${NC} $1"; }
warn() { echo -e "${YELLOW}[WARN]${NC} $1"; }
error() { echo -e "${RED}[ERROR]${NC} $1"; exit 1; }

# Detect OS and architecture
detect_platform() {
    OS=$(uname -s | tr '[:upper:]' '[:lower:]')
    ARCH=$(uname -m)

    case "$ARCH" in
        x86_64) ARCH="amd64" ;;
        aarch64|arm64) ARCH="arm64" ;;
        *) error "Unsupported architecture: $ARCH" ;;
    esac

    case "$OS" in
        darwin|linux) ;;
        *) error "Unsupported OS: $OS (only macOS and Linux are supported)" ;;
    esac

    info "Detected platform: ${OS}/${ARCH}"
}

# Check for required dependencies
check_dependencies() {
    info "Checking dependencies..."

    # Check for Go
    if ! command -v go &> /dev/null; then
        error "Go is required but not installed. Install from https://go.dev/dl/"
    fi
    GO_VERSION=$(go version | grep -oE 'go[0-9]+\.[0-9]+' | sed 's/go//')
    success "Go ${GO_VERSION} found"

    # Check for Ollama
    if ! command -v ollama &> /dev/null; then
        warn "Ollama not found. Pommel requires Ollama for embeddings."
        echo ""
        echo "  Install Ollama from: https://ollama.ai/download"
        echo ""
        read -p "Continue without Ollama? (y/N) " -n 1 -r
        echo
        if [[ ! $REPLY =~ ^[Yy]$ ]]; then
            exit 1
        fi
        OLLAMA_INSTALLED=false
    else
        success "Ollama found"
        OLLAMA_INSTALLED=true
    fi
}

# Determine install directory
get_install_dir() {
    if [[ -d "$HOME/.local/bin" ]]; then
        INSTALL_DIR="$HOME/.local/bin"
    elif [[ -w "/usr/local/bin" ]]; then
        INSTALL_DIR="/usr/local/bin"
    else
        INSTALL_DIR="$HOME/.local/bin"
        mkdir -p "$INSTALL_DIR"
    fi
    info "Install directory: $INSTALL_DIR"
}

# Build and install Pommel
install_pommel() {
    info "Installing Pommel..."

    # Create temp directory for build
    TEMP_DIR=$(mktemp -d)
    trap "rm -rf $TEMP_DIR" EXIT

    # Clone repository
    info "Cloning Pommel repository..."
    git clone --depth 1 https://github.com/dbinky/Pommel.git "$TEMP_DIR/pommel"

    cd "$TEMP_DIR/pommel"

    # Build binaries
    info "Building binaries..."

    # Get version info
    VERSION=$(git describe --tags 2>/dev/null || git rev-parse --short HEAD)
    COMMIT=$(git rev-parse --short HEAD)
    DATE=$(date -u +"%Y-%m-%dT%H:%M:%SZ")
    LDFLAGS="-s -w -X main.version=${VERSION} -X main.commit=${COMMIT} -X main.date=${DATE}"

    go build -trimpath -ldflags "$LDFLAGS" -o pm ./cmd/pm
    go build -trimpath -ldflags "$LDFLAGS" -o pommeld ./cmd/pommeld

    # Install binaries
    info "Installing to $INSTALL_DIR..."
    mv pm "$INSTALL_DIR/pm"
    mv pommeld "$INSTALL_DIR/pommeld"
    chmod +x "$INSTALL_DIR/pm" "$INSTALL_DIR/pommeld"

    success "Installed pm and pommeld to $INSTALL_DIR"
}

# Pull the embedding model
setup_embedding_model() {
    if [[ "$OLLAMA_INSTALLED" != "true" ]]; then
        warn "Skipping embedding model setup (Ollama not installed)"
        return
    fi

    MODEL="unclemusclez/jina-embeddings-v2-base-code"

    info "Pulling embedding model: $MODEL"
    info "This may take a few minutes on first run (~300MB)..."

    # Check if Ollama is running
    if ! curl -s http://localhost:11434/ > /dev/null 2>&1; then
        warn "Ollama is not running. Starting Ollama..."
        if [[ "$OS" == "darwin" ]]; then
            open -a Ollama 2>/dev/null || ollama serve &
        else
            ollama serve &
        fi
        sleep 3
    fi

    ollama pull "$MODEL" || warn "Failed to pull model. Run 'ollama pull $MODEL' manually."
    success "Embedding model ready"
}

# Check if install dir is in PATH
check_path() {
    if [[ ":$PATH:" != *":$INSTALL_DIR:"* ]]; then
        echo ""
        warn "$INSTALL_DIR is not in your PATH"
        echo ""
        echo "  Add this to your shell profile (~/.bashrc, ~/.zshrc, etc.):"
        echo ""
        echo "    export PATH=\"\$PATH:$INSTALL_DIR\""
        echo ""
        echo "  Then run: source ~/.bashrc  (or restart your terminal)"
        echo ""
    fi
}

# Verify installation
verify_install() {
    echo ""
    if command -v pm &> /dev/null; then
        success "Installation complete!"
        echo ""
        pm version 2>/dev/null || "$INSTALL_DIR/pm" version
    else
        success "Binaries installed to $INSTALL_DIR"
        echo ""
        "$INSTALL_DIR/pm" version 2>/dev/null || info "Run: $INSTALL_DIR/pm version"
    fi
}

# Print usage instructions
print_usage() {
    echo ""
    echo "━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━"
    echo ""
    echo "  Getting Started:"
    echo ""
    echo "    1. Navigate to your project:  cd /path/to/your/project"
    echo "    2. Initialize Pommel:         pm init"
    echo "    3. Start the daemon:          pm start"
    echo "    4. Search your code:          pm search \"your query\""
    echo ""
    echo "  For AI agents (Claude Code, etc.):"
    echo ""
    echo "    pm search \"authentication middleware\" --json"
    echo ""
    echo "━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━"
    echo ""
}

# Main
main() {
    echo ""
    echo "╔═══════════════════════════════════════════════════════════════════╗"
    echo "║                     Pommel Installer                              ║"
    echo "║           Semantic Code Search for AI Agents                      ║"
    echo "╚═══════════════════════════════════════════════════════════════════╝"
    echo ""

    detect_platform
    check_dependencies
    get_install_dir
    install_pommel
    setup_embedding_model
    check_path
    verify_install
    print_usage
}

main "$@"

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

    go build -tags fts5 -trimpath -ldflags "$LDFLAGS" -o pm ./cmd/pm
    go build -tags fts5 -trimpath -ldflags "$LDFLAGS" -o pommeld ./cmd/pommeld

    # Install binaries
    info "Installing to $INSTALL_DIR..."
    mv pm "$INSTALL_DIR/pm"
    mv pommeld "$INSTALL_DIR/pommeld"
    chmod +x "$INSTALL_DIR/pm" "$INSTALL_DIR/pommeld"

    success "Installed pm and pommeld to $INSTALL_DIR"
}

# Install language configuration files
install_language_configs() {
    info "Installing language configuration files..."

    LANG_CONFIG_DIR="$HOME/.local/share/pommel/languages"

    # Create the language config directory
    if ! mkdir -p "$LANG_CONFIG_DIR" 2>/dev/null; then
        warn "Failed to create directory: $LANG_CONFIG_DIR"
        warn "Language configs not installed. You may need to copy them manually."
        return 1
    fi

    # Check if we have language files to copy (we're still in the cloned repo)
    if [[ ! -d "languages" ]]; then
        warn "Language configs directory not found in repository"
        return 1
    fi

    # Count available language files
    local lang_count=$(find languages -name "*.yaml" -type f 2>/dev/null | wc -l | tr -d ' ')
    if [[ "$lang_count" -eq 0 ]]; then
        warn "No language configuration files found in repository"
        return 1
    fi

    info "Found $lang_count language configuration files"

    # Copy all .yaml files from languages/ to the config directory
    local copied=0
    local failed=0
    for lang_file in languages/*.yaml; do
        if [[ -f "$lang_file" ]]; then
            local filename=$(basename "$lang_file")
            if cp "$lang_file" "$LANG_CONFIG_DIR/$filename" 2>/dev/null; then
                ((copied++))
            else
                warn "Failed to copy: $filename"
                ((failed++))
            fi
        fi
    done

    if [[ "$failed" -gt 0 ]]; then
        warn "Copied $copied language configs, $failed failed"
        return 1
    fi

    success "Installed $copied language configs to $LANG_CONFIG_DIR"
    return 0
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

# Detect user's shell config file
get_shell_config() {
    local shell_name=$(basename "$SHELL")
    case "$shell_name" in
        zsh)
            echo "$HOME/.zshrc"
            ;;
        bash)
            # Prefer .bashrc, fall back to .bash_profile for macOS
            if [[ -f "$HOME/.bashrc" ]]; then
                echo "$HOME/.bashrc"
            else
                echo "$HOME/.bash_profile"
            fi
            ;;
        fish)
            echo "$HOME/.config/fish/config.fish"
            ;;
        *)
            # Default to .profile for other shells
            warn "Unknown shell '$shell_name', falling back to .profile"
            echo "$HOME/.profile"
            ;;
    esac
}

# Add install dir to PATH in shell config
add_to_path() {
    local config_file=$(get_shell_config)
    local shell_name=$(basename "$SHELL")
    local export_line

    # Create config file if it doesn't exist
    if [[ ! -f "$config_file" ]]; then
        mkdir -p "$(dirname "$config_file")"
        touch "$config_file"
    fi

    # Check if already added
    if grep -q "$INSTALL_DIR" "$config_file" 2>/dev/null; then
        info "PATH entry already exists in $config_file"
        return 0
    fi

    # Format export line based on shell
    if [[ "$shell_name" == "fish" ]]; then
        export_line="set -gx PATH \$PATH $INSTALL_DIR"
    else
        export_line="export PATH=\"\$PATH:$INSTALL_DIR\""
    fi

    # Add to config file
    echo "" >> "$config_file"
    echo "# Added by Pommel installer" >> "$config_file"
    echo "$export_line" >> "$config_file"

    success "Added PATH entry to $config_file"
    info "Run 'source $config_file' or restart your terminal to apply"
}

# Check if install dir is in PATH
check_path() {
    if [[ ":$PATH:" != *":$INSTALL_DIR:"* ]]; then
        echo ""
        warn "$INSTALL_DIR is not in your PATH"
        echo ""
        read -p "Would you like to add it to your shell config automatically? (Y/n) " -n 1 -r
        echo
        if [[ -z "$REPLY" || $REPLY =~ ^[Yy]$ ]]; then
            add_to_path
        else
            echo ""
            echo "  To add manually, put this in your shell profile (~/.bashrc, ~/.zshrc, etc.):"
            echo ""
            echo "    export PATH=\"\$PATH:$INSTALL_DIR\""
            echo ""
        fi
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
    echo "  Installed locations:"
    echo ""
    echo "    Binaries:          $INSTALL_DIR/pm, $INSTALL_DIR/pommeld"
    echo "    Language configs:  ~/.local/share/pommel/languages/"
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
    install_language_configs
    setup_embedding_model
    check_path
    verify_install
    print_usage
}

main "$@"

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

# Source-only mode for testing
if [[ "$1" == "--source-only" ]]; then
    return 0 2>/dev/null || exit 0
fi

# Repository info
REPO="dbinky/Pommel"

# Colors for output
RED='\033[0;31m'
GREEN='\033[0;32m'
YELLOW='\033[1;33m'
BLUE='\033[0;34m'
CYAN='\033[0;36m'
NC='\033[0m' # No Color

info() { echo -e "${BLUE}[INFO]${NC} $1"; }
success() { echo -e "${GREEN}[OK]${NC} $1"; }
warn() { echo -e "${YELLOW}[WARN]${NC} $1"; }
error() { echo -e "${RED}[ERROR]${NC} $1"; exit 1; }
step() { echo -e "${CYAN}$1${NC}"; }

# Global variables
VERSION=""
IS_UPGRADE=false
CURRENT_VERSION=""
SELECTED_PROVIDER=""
OLLAMA_REMOTE_URL=""
OPENAI_API_KEY=""
VOYAGE_API_KEY=""
OLLAMA_INSTALLED=false
INSTALL_DIR=""
OS=""
ARCH=""

# Get latest version from GitHub
get_latest_version() {
    local api_url="https://api.github.com/repos/$REPO/releases/latest"
    local response

    response=$(curl -s "$api_url" 2>/dev/null) || {
        # Fall back to getting version from git tags
        VERSION="latest"
        return
    }

    VERSION=$(echo "$response" | grep -o '"tag_name": *"[^"]*"' | cut -d'"' -f4)
    VERSION=${VERSION:-"latest"}
}

# Detect existing installation
detect_existing_install() {
    IS_UPGRADE=false
    CURRENT_VERSION=""

    if command -v pm &> /dev/null; then
        CURRENT_VERSION=$(pm version 2>/dev/null | grep -oE '[0-9]+\.[0-9]+\.[0-9]+' | head -1) || true
        if [[ -n "$CURRENT_VERSION" ]]; then
            IS_UPGRADE=true
        fi
    fi
}

# Check if global config exists with a provider
has_existing_provider_config() {
    local config_dir
    if [[ -n "$XDG_CONFIG_HOME" ]]; then
        config_dir="$XDG_CONFIG_HOME/pommel"
    else
        config_dir="$HOME/.config/pommel"
    fi

    if [[ -f "$config_dir/config.yaml" ]]; then
        if grep -q "provider:" "$config_dir/config.yaml" 2>/dev/null; then
            return 0
        fi
    fi
    return 1
}

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
    step "[1/5] Checking dependencies..."
    echo ""

    # Check for Go
    if ! command -v go &> /dev/null; then
        error "Go is required but not installed. Install from https://go.dev/dl/"
    fi
    GO_VERSION=$(go version | grep -oE 'go[0-9]+\.[0-9]+' | sed 's/go//')
    success "Go ${GO_VERSION} found"
}

# Provider selection
select_provider() {
    echo ""
    step "[2/5] Configure embedding provider"
    echo ""
    echo "  How would you like to generate embeddings?"
    echo ""
    echo "  1) Local Ollama    - Free, runs on this machine (~300MB model)"
    echo "  2) Remote Ollama   - Free, connect to Ollama on another machine"
    echo "  3) OpenAI API      - Paid, no local setup required"
    echo "  4) Voyage AI       - Paid, optimized for code search"
    echo ""
    read -p "  Choice [1]: " choice < /dev/tty
    choice=${choice:-1}

    case $choice in
        1) setup_local_ollama ;;
        2) setup_remote_ollama ;;
        3) setup_openai ;;
        4) setup_voyage ;;
        *)
            warn "Invalid choice. Please enter 1-4."
            select_provider
            ;;
    esac
}

setup_local_ollama() {
    SELECTED_PROVIDER="ollama"
    success "Selected: Local Ollama"

    # Check for Ollama
    if ! command -v ollama &> /dev/null; then
        warn "Ollama not found on this machine."
        echo ""
        echo "  Install Ollama from: https://ollama.ai/download"
        echo ""
        read -p "  Continue anyway? (y/N) " -n 1 -r < /dev/tty
        echo
        if [[ ! $REPLY =~ ^[Yy]$ ]]; then
            select_provider
            return
        fi
        OLLAMA_INSTALLED=false
    else
        success "Ollama found"
        OLLAMA_INSTALLED=true
    fi
}

setup_remote_ollama() {
    SELECTED_PROVIDER="ollama-remote"
    echo ""
    read -p "  Enter Ollama server URL (e.g., http://192.168.1.100:11434): " url < /dev/tty

    if [[ -z "$url" ]]; then
        warn "URL is required for remote Ollama"
        setup_remote_ollama
        return
    fi

    OLLAMA_REMOTE_URL="$url"
    success "Selected: Remote Ollama at $url"
}

setup_openai() {
    SELECTED_PROVIDER="openai"
    echo ""
    read -p "  Enter your OpenAI API key (leave blank to configure later): " key < /dev/tty

    if [[ -n "$key" ]]; then
        info "Validating API key..."
        if validate_openai_key "$key"; then
            OPENAI_API_KEY="$key"
            success "API key validated"
        else
            warn "Invalid API key. Run 'pm config provider' later to configure."
            OPENAI_API_KEY=""
        fi
    else
        OPENAI_API_KEY=""
        info "Skipped. Run 'pm config provider' to add your API key later."
    fi
}

setup_voyage() {
    SELECTED_PROVIDER="voyage"
    echo ""
    read -p "  Enter your Voyage AI API key (leave blank to configure later): " key < /dev/tty

    if [[ -n "$key" ]]; then
        info "Validating API key..."
        if validate_voyage_key "$key"; then
            VOYAGE_API_KEY="$key"
            success "API key validated"
        else
            warn "Invalid API key. Run 'pm config provider' later to configure."
            VOYAGE_API_KEY=""
        fi
    else
        VOYAGE_API_KEY=""
        info "Skipped. Run 'pm config provider' to add your API key later."
    fi
}

validate_openai_key() {
    local key="$1"
    local response
    local http_code

    response=$(curl -s -w "\n%{http_code}" \
        -H "Authorization: Bearer $key" \
        -H "Content-Type: application/json" \
        -d '{"model": "text-embedding-3-small", "input": "test"}' \
        "https://api.openai.com/v1/embeddings" 2>/dev/null) || return 1

    http_code=$(echo "$response" | tail -1)

    if [[ "$http_code" == "200" ]]; then
        return 0
    else
        return 1
    fi
}

validate_voyage_key() {
    local key="$1"
    local response
    local http_code

    response=$(curl -s -w "\n%{http_code}" \
        -H "Authorization: Bearer $key" \
        -H "Content-Type: application/json" \
        -d '{"model": "voyage-code-3", "input": ["test"]}' \
        "https://api.voyageai.com/v1/embeddings" 2>/dev/null) || return 1

    http_code=$(echo "$response" | tail -1)

    if [[ "$http_code" == "200" ]]; then
        return 0
    else
        return 1
    fi
}

# Write global configuration
write_global_config() {
    local config_dir
    if [[ -n "$XDG_CONFIG_HOME" ]]; then
        config_dir="$XDG_CONFIG_HOME/pommel"
    else
        config_dir="$HOME/.config/pommel"
    fi

    mkdir -p "$config_dir"

    local config_file="$config_dir/config.yaml"

    cat > "$config_file" << EOF
# Pommel global configuration
# Generated by install script

embedding:
  provider: $SELECTED_PROVIDER
EOF

    case $SELECTED_PROVIDER in
        ollama)
            cat >> "$config_file" << EOF
  ollama:
    url: "http://localhost:11434"
    model: "unclemusclez/jina-embeddings-v2-base-code"
EOF
            ;;
        ollama-remote)
            cat >> "$config_file" << EOF
  ollama:
    url: "$OLLAMA_REMOTE_URL"
    model: "unclemusclez/jina-embeddings-v2-base-code"
EOF
            ;;
        openai)
            if [[ -n "$OPENAI_API_KEY" ]]; then
                cat >> "$config_file" << EOF
  openai:
    api_key: "$OPENAI_API_KEY"
    model: "text-embedding-3-small"
EOF
            else
                cat >> "$config_file" << EOF
  openai:
    # api_key: "" # Set via OPENAI_API_KEY environment variable or run 'pm config provider'
    model: "text-embedding-3-small"
EOF
            fi
            ;;
        voyage)
            if [[ -n "$VOYAGE_API_KEY" ]]; then
                cat >> "$config_file" << EOF
  voyage:
    api_key: "$VOYAGE_API_KEY"
    model: "voyage-code-3"
EOF
            else
                cat >> "$config_file" << EOF
  voyage:
    # api_key: "" # Set via VOYAGE_API_KEY environment variable or run 'pm config provider'
    model: "voyage-code-3"
EOF
            fi
            ;;
    esac

    success "Configuration saved to $config_file"
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

# Install language configuration files (called from within cloned repo)
install_language_configs_from_repo() {
    step "[4/5] Installing language configurations..."
    echo ""

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

    # Copy all .yaml files from languages/ to the config directory
    local copied=0
    local failed=0
    for lang_file in languages/*.yaml; do
        if [[ -f "$lang_file" ]]; then
            local filename=$(basename "$lang_file")
            if cp "$lang_file" "$LANG_CONFIG_DIR/$filename" 2>/dev/null; then
                copied=$((copied + 1))
            else
                warn "Failed to copy: $filename"
                failed=$((failed + 1))
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

# Build and install Pommel
install_pommel() {
    step "[3/5] Installing Pommel..."
    echo ""

    # Save original directory
    ORIG_DIR=$(pwd)

    # Create temp directory for build
    TEMP_DIR=$(mktemp -d)
    trap "cd '$ORIG_DIR' 2>/dev/null; rm -rf '$TEMP_DIR'" EXIT

    # Clone repository
    info "Cloning Pommel repository..."
    git clone --depth 1 https://github.com/$REPO.git "$TEMP_DIR/pommel"

    cd "$TEMP_DIR/pommel"

    # Build binaries
    info "Building binaries..."

    # Get version info
    VERSION=$(git describe --tags 2>/dev/null || git rev-parse --short HEAD)
    COMMIT=$(git rev-parse --short HEAD)
    DATE=$(date -u +"%Y-%m-%dT%H:%M:%SZ")
    LDFLAGS="-s -w -X main.version=${VERSION} -X main.commit=${COMMIT} -X main.date=${DATE}"

    # On macOS, set C++ include path for tree-sitter YAML parser
    if [[ "$OS" == "darwin" ]]; then
        SDK_PATH=$(xcrun --show-sdk-path 2>/dev/null)
        if [[ -n "$SDK_PATH" ]]; then
            export CGO_CXXFLAGS="-isystem ${SDK_PATH}/usr/include/c++/v1"
        fi
    fi

    go build -tags fts5 -trimpath -ldflags "$LDFLAGS" -o pm ./cmd/pm
    go build -tags fts5 -trimpath -ldflags "$LDFLAGS" -o pommeld ./cmd/pommeld

    # Install binaries
    info "Installing to $INSTALL_DIR..."
    mv pm "$INSTALL_DIR/pm"
    mv pommeld "$INSTALL_DIR/pommeld"
    chmod +x "$INSTALL_DIR/pm" "$INSTALL_DIR/pommeld"

    success "Installed pm and pommeld to $INSTALL_DIR"

    # Install language configs while still in the cloned repo
    install_language_configs_from_repo
}

# Pull the embedding model for local Ollama
setup_embedding_model() {
    if [[ "$SELECTED_PROVIDER" != "ollama" ]] || [[ "$OLLAMA_INSTALLED" != "true" ]]; then
        return
    fi

    step "[5/5] Setting up embedding model..."
    echo ""

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
        read -p "Would you like to add it to your shell config automatically? (Y/n) " -n 1 -r < /dev/tty
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
    echo "  Change provider later:"
    echo ""
    echo "    pm config provider"
    echo ""
    echo "  Installed locations:"
    echo ""
    echo "    Binaries:          $INSTALL_DIR/pm, $INSTALL_DIR/pommeld"
    echo "    Global config:     ~/.config/pommel/config.yaml"
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

    # Get version info
    get_latest_version

    # Check for existing install
    detect_existing_install

    if [[ "$IS_UPGRADE" == "true" ]]; then
        info "Previous install detected (v${CURRENT_VERSION}) - upgrading to ${VERSION}"
    else
        info "Installing Pommel ${VERSION}"
    fi
    echo ""

    detect_platform
    check_dependencies
    get_install_dir

    # Provider selection (skip on upgrade if config exists)
    if [[ "$IS_UPGRADE" == "true" ]] && has_existing_provider_config; then
        info "Using existing provider configuration"
    else
        select_provider
        write_global_config
    fi

    install_pommel
    setup_embedding_model
    check_path
    verify_install
    print_usage
}

main "$@"

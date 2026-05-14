#!/bin/bash
# =============================================================================
# Bootstrap AI Cabin Environment
# =============================================================================
# This script bootstraps a new AI Cabin environment from scratch.
#
# What it does:
#   - Creates directories for AI_CABIN_HOME, AI_CABIN_DESK, AI_CABIN_WORKDIR
#   - Copies desk/ from templates/desk/ (complete desk with AGENTS.md, skills/, etc.)
#   - Creates .envrc with the configured paths
#
# What it does NOT do:
#   - Copy cabin files (use existing cabin/opencode-go/ or other)
#   - Create AI_CABIN_HOME subdirs (left to 'make setup')
#
# Usage:
#   ./bootstrap-cabin.sh <base-path> [env-name]
#
# Examples:
#   # Test environment in /tmp
#   ./bootstrap-cabin.sh /tmp/ai-cabin-test-1
#
#   # Production environment in home
#   ./bootstrap-cabin.sh /home/user/ai-cabin-prod
#
#   # Custom environment with name
#   ./bootstrap-cabin.sh /home/user/my-cabins project-x
#
# If base-path doesn't exist, it will be created.
# =============================================================================

set -euo pipefail

# Parse arguments
if [ $# -lt 1 ]; then
    echo "Usage: $0 <base-path> [env-name]"
    echo ""
    echo "Examples:"
    echo "  $0 /tmp/ai-cabin-test-1"
    echo "  $0 /home/user/ai-cabin-prod"
    echo "  $0 /home/user/my-cabins project-x"
    exit 1
fi

BASE_PATH="$(realpath ${1})"
ENV_NAME="${2:-$(basename "${BASE_PATH}")}"

# Derive directories from base path
AI_CABIN_HOME="${BASE_PATH}/home"
AI_CABIN_DESK="${BASE_PATH}/workflow"
AI_CABIN_WORKDIR="${BASE_PATH}/workdir"

# Source paths (from workspace)
SCRIPT_DIR="$(cd "$(dirname "${BASH_SOURCE[0]}")" && pwd)"
WORKSPACE_DIR="$(dirname "$(dirname "${SCRIPT_DIR}")")"
DESK_SOURCE="${WORKSPACE_DIR}/templates/desk"

# Colors for output
RED='\033[0;31m'
GREEN='\033[0;32m'
YELLOW='\033[1;33m'
BLUE='\033[0;34m'
NC='\033[0m' # No Color

# =============================================================================
# Helper Functions
# =============================================================================

log_info() {
    echo -e "${BLUE}[INFO]${NC} $1"
}

log_success() {
    echo -e "${GREEN}[OK]${NC} $1"
}

log_warning() {
    echo -e "${YELLOW}[WARN]${NC} $1"
}

log_error() {
    echo -e "${RED}[ERROR]${NC} $1"
}

# =============================================================================
# Main Setup
# =============================================================================

main() {
    echo "=============================================="
    echo "Bootstrap AI Cabin Environment"
    echo "=============================================="
    echo ""
    log_info "Environment: ${ENV_NAME}"
    log_info "Base path: ${BASE_PATH}"
    echo ""

    # Check if base path exists
    if [ -d "${BASE_PATH}" ]; then
        log_warning "Base path already exists: ${BASE_PATH}"
        read -p "Continue? This will overwrite existing files. (y/N) " -n 1 -r
        echo
        if [[ ! $REPLY =~ ^[Yy]$ ]]; then
            log_info "Aborted"
            exit 0
        fi
    fi

    # Step 1: Create directories
    log_info "Step 1: Creating directories..."
    mkdir -p "${AI_CABIN_HOME}"
    mkdir -p "${AI_CABIN_DESK}"
    mkdir -p "${AI_CABIN_WORKDIR}"
    log_success "Directories created"

    # Step 2: Copy desk template (complete desk with AGENTS.md, skills/, etc.)
    log_info "Step 2: Copying desk template..."
    if [ -d "${DESK_SOURCE}" ]; then
        cp -r "${DESK_SOURCE}"/* "${AI_CABIN_DESK}/"
        log_success "Desk template copied to ${AI_CABIN_DESK}/"
    else
        log_error "Desk template directory not found at ${DESK_SOURCE}"
        exit 1
    fi

    # Step 3: Create .envrc
    log_info "Step 3: Creating .envrc..."
    cat > "${BASE_PATH}/.envrc" << EOF
# =============================================================================
# AI Cabin Environment: ${ENV_NAME}
# =============================================================================
# Bootstrapped: $(date)
# Base path: ${BASE_PATH}
#
# Usage:
#   1. Copy this file to your cabin:
#      cp ${BASE_PATH}/.envrc /path/to/your/cabin/.envrc
#   2. Allow direnv:
#      cd /path/to/your/cabin && direnv allow
#   3. Run setup:
#      make setup
#   4. Start cabin:
#      make docker-up
# =============================================================================

# AI_CABIN_HOME: Home directory for agent data (empty, make setup creates subdirs)
export AI_CABIN_HOME=${AI_CABIN_HOME}

# AI_CABIN_DESK: Shared desk directory (populated by bootstrap script)
export AI_CABIN_DESK=${AI_CABIN_DESK}

# AI_CABIN_WORKDIR: Work directory for git repos (empty)
export AI_CABIN_WORKDIR=${AI_CABIN_WORKDIR}

# SCW_PROJECT_ID: Scaleway project ID (replace with real one)
export SCW_PROJECT_ID=e60d561f-8d71-4253-8f71-1d70a83c2575

# OPENCODE_SERVER_PASSWORD: OpenCode web UI password (replace with real one)
export OPENCODE_SERVER_PASSWORD=change-me

# SCW_SECRET_KEY: Optional (greyproxy injects it if running)
# export SCW_SECRET_KEY=<your-secret-key>

# CONTAINER_WORKDIR: Transparent mode (same as workdir)
# Uncomment for remap mode:
# export CONTAINER_WORKDIR=/workspace/
EOF
    log_success ".envrc created at ${BASE_PATH}/.envrc"

    # Step 4: Create summary
    echo ""
    echo "=============================================="
    echo "Environment Ready"
    echo "=============================================="
    echo ""
    log_info "Environment: ${ENV_NAME}"
    log_info "Base path: ${BASE_PATH}"
    log_info "AI_CABIN_HOME:     ${AI_CABIN_HOME} (empty)"
    log_info "AI_CABIN_DESK: ${AI_CABIN_DESK} (populated)"
    log_info "AI_CABIN_WORKDIR:  ${AI_CABIN_WORKDIR} (empty)"
    echo ""
    log_info "Next steps:"
    echo "  1. Copy .envrc to your cabin:"
    echo "     cp ${BASE_PATH}/.envrc /path/to/your/cabin/.envrc"
    echo ""
    echo "  2. Go to your cabin and allow direnv:"
    echo "     cd /path/to/your/cabin"
    echo "     direnv allow"
    echo ""
    echo "  3. Run setup (creates AI_CABIN_HOME subdirs, copies config):"
    echo "     make setup"
    echo ""
    echo "  4. Start cabin:"
    echo "     make docker-up"
    echo ""
    log_success "Environment is ready!"
}

# Run main
main "$@"

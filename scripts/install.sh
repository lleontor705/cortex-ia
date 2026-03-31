#!/bin/bash
# ─────────────────────────────────────────────────────────────────────────────
# install.sh — Install cortex-ia
#
# Usage:
#   curl -sSL https://raw.githubusercontent.com/lleontor705/cortex-ia/main/scripts/install.sh | bash
#   curl -sSL https://raw.githubusercontent.com/lleontor705/cortex-ia/main/scripts/install.sh | bash -s -- v0.1.0
# ─────────────────────────────────────────────────────────────────────────────

set -euo pipefail

REPO="lleontor705/cortex-ia"
INSTALL_DIR="${INSTALL_DIR:-/usr/local/bin}"
VERSION="${1:-latest}"

RED='\033[0;31m'
GREEN='\033[0;32m'
CYAN='\033[0;36m'
BOLD='\033[1m'
NC='\033[0m'

info()    { printf "${CYAN}[INFO]${NC}  %s\n" "$1"; }
success() { printf "${GREEN}[OK]${NC}    %s\n" "$1"; }
error()   { printf "${RED}[ERROR]${NC} %s\n" "$1" >&2; exit 1; }

require_command() {
  command -v "$1" >/dev/null 2>&1 || error "Required command not found: $1"
}

sha256_file() {
  if command -v sha256sum >/dev/null 2>&1; then
    sha256sum "$1" | awk '{print $1}'
    return
  fi
  if command -v shasum >/dev/null 2>&1; then
    shasum -a 256 "$1" | awk '{print $1}'
    return
  fi
  error "No SHA-256 tool found (need sha256sum or shasum)"
}

printf "\n${BOLD}${CYAN}"
echo "  ╔═══════════════════════════════════════════════════════════╗"
echo "  ║              cortex-ia Installer                          ║"
echo "  ║  AI Agent Ecosystem Configurator                          ║"
echo "  ╚═══════════════════════════════════════════════════════════╝"
printf "${NC}\n"

OS="$(uname -s | tr '[:upper:]' '[:lower:]')"
ARCH="$(uname -m)"

case "$OS" in
  linux)  OS="linux" ;;
  darwin) OS="darwin" ;;
  *)      error "Unsupported OS: $OS. Use 'go install' instead." ;;
esac

case "$ARCH" in
  x86_64|amd64)  ARCH="amd64" ;;
  aarch64|arm64) ARCH="arm64" ;;
  *)             error "Unsupported architecture: $ARCH" ;;
esac

info "Detected platform: ${OS}/${ARCH}"
require_command curl
require_command tar

if [ "$VERSION" = "latest" ]; then
  VERSION=$(curl -sSf "https://api.github.com/repos/${REPO}/releases/latest" | grep '"tag_name"' | sed -E 's/.*"([^"]+)".*/\1/')
  if [ -z "$VERSION" ]; then
    error "Could not determine latest version. Specify: $0 v0.1.0"
  fi
fi

info "Installing cortex-ia ${VERSION}"

ARCHIVE="cortex-ia_${VERSION#v}_${OS}_${ARCH}.tar.gz"
URL="https://github.com/${REPO}/releases/download/${VERSION}/${ARCHIVE}"
CHECKSUMS_URL="https://github.com/${REPO}/releases/download/${VERSION}/checksums.txt"

TMPDIR=$(mktemp -d)
trap 'rm -rf "$TMPDIR"' EXIT

info "Downloading ${URL}..."
if ! curl -sSfL "$URL" -o "${TMPDIR}/${ARCHIVE}"; then
  error "Download failed. Check https://github.com/${REPO}/releases"
fi

info "Downloading release checksums..."
if ! curl -sSfL "$CHECKSUMS_URL" -o "${TMPDIR}/checksums.txt"; then
  error "Checksum download failed. Refusing to install without checksum verification."
fi

info "Verifying archive checksum..."
EXPECTED_SUM="$(grep -F "  ${ARCHIVE}" "${TMPDIR}/checksums.txt" | awk '{print $1}')"
if [ -z "${EXPECTED_SUM}" ]; then
  error "Could not find checksum entry for ${ARCHIVE}"
fi
ACTUAL_SUM="$(sha256_file "${TMPDIR}/${ARCHIVE}")"
if [ "${EXPECTED_SUM}" != "${ACTUAL_SUM}" ]; then
  error "Checksum verification failed for ${ARCHIVE}"
fi
success "Checksum verified"

info "Extracting..."
tar -xzf "${TMPDIR}/${ARCHIVE}" -C "$TMPDIR"

if [ ! -f "${TMPDIR}/cortex-ia" ]; then
  error "Binary not found in archive"
fi

chmod +x "${TMPDIR}/cortex-ia"

info "Installing to ${INSTALL_DIR}/cortex-ia..."
if [ ! -d "$INSTALL_DIR" ]; then
  mkdir -p "$INSTALL_DIR" 2>/dev/null || sudo mkdir -p "$INSTALL_DIR"
fi
if [ -w "$INSTALL_DIR" ]; then
  mv "${TMPDIR}/cortex-ia" "${INSTALL_DIR}/cortex-ia"
else
  sudo /bin/mv "${TMPDIR}/cortex-ia" "${INSTALL_DIR}/cortex-ia"
fi

if command -v cortex-ia &> /dev/null; then
  success "cortex-ia installed: $(cortex-ia version 2>/dev/null || echo "$VERSION")"
else
  echo "  cortex-ia installed to ${INSTALL_DIR}/cortex-ia"
  echo "  Add to PATH: export PATH=\"${INSTALL_DIR}:\$PATH\""
fi

printf "\n${BOLD}${GREEN}Installation complete!${NC}\n\n"
echo "  Quick start:"
echo "    cortex-ia              # Interactive TUI"
echo "    cortex-ia install      # Auto-detect and configure all agents"
echo "    cortex-ia detect       # Show detected agents"
echo ""
echo "  Docs: https://github.com/${REPO}"
echo ""

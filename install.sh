#!/usr/bin/env bash
set -euo pipefail

# ws installer
# Usage: curl -sSL https://raw.githubusercontent.com/weslien/ws/main/install.sh | bash
# Or:    wget -qO- https://raw.githubusercontent.com/weslien/ws/main/install.sh | bash

REPO="weslien/ws"
BINARY="ws"
VERSION="${VERSION:-}"      # override via env: VERSION=0.1.3 curl ... | bash
INSTALL_DIR="${INSTALL_DIR:-/usr/local/bin}"

# ---------- helpers ----------

log()  { echo "[ws-install] $*" >&2; }
die()  { log "error: $*"; exit 1; }

command_exists() { command -v "$1" >/dev/null 2>&1; }

get_os() {
  case "$(uname -s)" in
    Linux*)  echo "linux";;
    Darwin*) echo "darwin";;
    *)       echo "unknown";;
  esac
}

get_arch() {
  case "$(uname -m)" in
    x86_64|amd64)  echo "amd64";;
    arm64|aarch64) echo "arm64";;
    *)             echo "$(uname -m)";;
  esac
}

# ---------- version resolution ----------

resolve_version() {
  if [ -n "${VERSION:-}" ]; then
    echo "$VERSION"
    return
  fi
  # fetch latest release tag from GitHub API
  local latest
  if command_exists curl; then
    latest=$(curl -sL "https://api.github.com/repos/${REPO}/releases/latest" 2>/dev/null | grep -oP '"tag_name":\s*"\K[^"]+' || true)
  fi
  if [ -z "${latest:-}" ]; then
    # No semver releases yet — fall back to continuous prerelease
    latest=$(curl -sL "https://api.github.com/repos/${REPO}/releases/tags/continuous" 2>/dev/null | grep -oP '"tag_name":\s*"\K[^"]+' || true)
  fi
  if [ -z "${latest:-}" ]; then
    # No releases at all — can't determine version
    echo ""
    return 1
  fi
  echo "$latest"
}

# ---------- install methods ----------

install_prebuilt() {
  local ver="$1"
  local os_name="$2"
  local arch="$3"

  local name="${BINARY}-${ver}-${os_name}-${arch}.tar.gz"
  local url="https://github.com/${REPO}/releases/download/${ver}/${name}"

  log "downloading ${url} ..."
  local tmpdir
  tmpdir=$(mktemp -d)

  if ! curl -sLf --proto '=https' "${url}" -o "${tmpdir}/${name}"; then
    rm -rf "${tmpdir}"
    return 1
  fi

  tar -xzf "${tmpdir}/${name}" -C "${tmpdir}"
  local bin_path
  bin_path=$(find "${tmpdir}" -maxdepth 2 -type f -name "${BINARY}" | head -n1)
  [ -n "${bin_path}" ] || { rm -rf "${tmpdir}"; return 1; }

  install_binary "${bin_path}"
  rm -rf "${tmpdir}"
}

install_go() {
  local ver="${1:-}"
  log "Go detected; installing via go install ..."
  local target
  if [ -n "${ver:-}" ]; then
    target="github.com/${REPO}/cmd/${BINARY}@${ver}"
  else
    target="github.com/${REPO}/cmd/${BINARY}@latest"
  fi
  GOPATH="${GOPATH:-${HOME}/go}"
  GOBIN="${GOBIN:-${GOPATH}/bin}"
  if ! (export GOBIN="${GOBIN}"; go install "${target}"); then
    return 1
  fi
  local bin_path="${GOBIN}/${BINARY}"
  install_binary "${bin_path}"
}

install_source() {
  log "building from source ..."
  local tmpdir
  tmpdir=$(mktemp -d)

  local repo_dir="${tmpdir}/src"
  git clone --depth 1 "https://github.com/${REPO}.git" "${repo_dir}"
  (cd "${repo_dir}" && go build -o "${tmpdir}/${BINARY}.bin" "./cmd/${BINARY}")
  install_binary "${tmpdir}/${BINARY}.bin"
  rm -rf "${tmpdir}"
}

install_binary() {
  local src="$1"
  local dst="${INSTALL_DIR%/}/${BINARY}"

  # Create install dir if needed
  if [ ! -d "$INSTALL_DIR" ]; then
    mkdir -p "$INSTALL_DIR" || die "cannot create ${INSTALL_DIR}"
  fi

  # Check writability — if not, try sudo or ~/.local/bin
  if [ -w "$INSTALL_DIR" ] || [ ! -e "$INSTALL_DIR" ]; then
    : # writable
  else
    log "${INSTALL_DIR} not writable, trying ${HOME}/.local/bin ..."
    INSTALL_DIR="${HOME}/.local/bin"
    mkdir -p "$INSTALL_DIR"
    dst="${INSTALL_DIR}/${BINARY}"
  fi

  # Move binary into place
  if [ -w "$(dirname "$dst")" ]; then
    cp "${src}" "${dst}"
    chmod +x "${dst}"
  else
    log "sudo required to write to ${dst}"
    sudo cp "${src}" "${dst}"
    sudo chmod +x "${dst}"
  fi

  log "installed ${dst}"

  # Verify
  if command_exists "$BINARY"; then
    local installed_ver
    installed_ver=$($BINARY --version 2>/dev/null || echo "unknown")
    log "version: ${installed_ver}"
  else
    log "WARN: ${dst} is not in your PATH. Add this to your shell profile:"
    log "  export PATH=\"${INSTALL_DIR}:\${PATH}\""
  fi
}

# ---------- main ----------

main() {
  local os_name arch ver
  os_name=$(get_os)
  arch=$(get_arch)

  [ "$os_name" != "unknown" ] || die "unsupported OS: $(uname -s)"

  log "detected platform: ${os_name}/${arch}"

  # Resolve version
  ver=""
  if resolve_version >/dev/null 2>&1; then
    ver=$(resolve_version)
    log "target version: ${ver}"
  else
    log "no release found, will build from source"
  fi

  # 1) Try prebuilt binary from GitHub release
  if [ -n "${ver:-}" ]; then
    if install_prebuilt "$ver" "$os_name" "$arch"; then
      log "success (prebuilt binary)"
      return 0
    fi
    log "no prebuilt binary found, trying alternative methods ..."
  fi

  # 2) Try go install (skip if version is a non-semver prerelease)
  if command_exists go; then
    if [ -n "${ver:-}" ] && echo "$ver" | grep -qE '^v[0-9]\.'; then
      if install_go "$ver"; then
        log "success (go install)"
        return 0
      fi
      log "go install failed, falling back to source build ..."
    else
      log "go install: skipping non-semver version '$ver', falling back to source ..."
    fi
  fi

  # 3) Try source build (needs git + go)
  if command_exists git && command_exists go; then
    if install_source; then
      log "success (built from source)"
      return 0
    fi
  fi

  die "could not install ${BINARY}. Please install Go (https://go.dev/dl) and run:\n  go install github.com/${REPO}/cmd/${BINARY}@latest"
}

main "$@"

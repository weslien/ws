#!/usr/bin/env bash
set -euo pipefail

# ws installer
# Usage: curl -sSL https://raw.githubusercontent.com/weslien/ws/main/install.sh | bash
# Or:    wget -qO- https://raw.githubusercontent.com/weslien/ws/main/install.sh | bash
#
# Install methods, in order of preference:
#   1. Prebuilt binary from the latest GitHub release (checksum-verified
#      against the published .sha256 asset)
#   2. Source build at the release tag (git + go; version injected via ldflags)
#   3. go install (version string shows 'dev')
#
# Overrides (env):
#   VERSION=0.9.1        install a specific release tag (default: latest release)
#   INSTALL_DIR=/path    where to place the binary (default: /usr/local/bin,
#                        falls back to ~/.local/bin if not writable)

REPO="weslien/ws"
BINARY="ws"
VERSION="${VERSION:-}"
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

# resolve_version prints the latest RELEASE tag (never a bare push-tag):
# releases/latest is the single source of truth — a tag like v0.9.0 exists
# on GitHub even when its release pipeline failed (v0.7.0–v0.9.0 all did),
# and the tags API is NOT semver-ordered, so both would install the wrong
# or broken thing. Only explicit VERSION= pins may name a non-release tag.
resolve_version() {
  if [ -n "${VERSION:-}" ]; then
    echo "$VERSION"
    return 0
  fi
  local latest
  if command_exists curl; then
    latest=$(curl -sL "https://api.github.com/repos/${REPO}/releases/latest" 2>/dev/null \
      | sed -n 's/.*"tag_name": *"\([^"]*\)".*/\1/p' | head -n1)
  elif command_exists wget; then
    latest=$(wget -qO- "https://api.github.com/repos/${REPO}/releases/latest" 2>/dev/null \
      | sed -n 's/.*"tag_name": *"\([^"]*\)".*/\1/p' | head -n1)
  fi
  if [ -z "${latest:-}" ]; then
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

  local fetch_cmd
  if command_exists curl; then
    fetch_cmd=(curl -sfL --proto '=https' -o)
  elif command_exists wget; then
    # wget follows redirects by default; --secure-protocol enforces TLS.
    fetch_cmd=(wget -q --secure-protocol=auto -O)
  else
    rm -rf "${tmpdir}"
    return 1
  fi

  if ! "${fetch_cmd[@]}" "${tmpdir}/${name}" "${url}"; then
    rm -rf "${tmpdir}"
    return 1
  fi

  # Verify the tarball against the published .sha256 asset when present.
  # Missing asset -> warn and continue; mismatch -> fail (main() falls
  # through to a source build at the tag, which does not trust the artifact).
  if ! "${fetch_cmd[@]}" "${tmpdir}/${name}.sha256" "${url}.sha256" 2>/dev/null; then
    : # no checksum published for this release (pre-v0.9.1 releases may miss it)
  fi
  local expected=""
  if [ -s "${tmpdir}/${name}.sha256" ]; then
    expected=$(awk '{print $1}' "${tmpdir}/${name}.sha256")
  fi
  if [ -n "${expected:-}" ]; then
    local actual=""
    if command_exists sha256sum; then
      actual=$(sha256sum "${tmpdir}/${name}" | awk '{print $1}')
    elif command_exists shasum; then
      actual=$(shasum -a 256 "${tmpdir}/${name}" | awk '{print $1}')
    else
      log "no sha256 tool found; skipping checksum verification"
    fi
    if [ -n "${actual}" ] && [ "${actual}" != "${expected}" ]; then
      log "checksum mismatch for ${name}"
      log "  expected: ${expected}"
      log "  actual:   ${actual}"
      rm -rf "${tmpdir}"
      return 1
    fi
    if [ -n "${actual}" ]; then
      log "checksum ok (${actual:0:12}...)"
    fi
  else
    log "no checksum asset published; continuing without verification"
  fi

  tar -xzf "${tmpdir}/${name}" -C "${tmpdir}"
  local bin_path
  bin_path=$(find "${tmpdir}" -maxdepth 2 -type f -name "${BINARY}" | head -n1)
  if [ -z "${bin_path}" ]; then
    rm -rf "${tmpdir}"
    return 1
  fi

  install_binary "${bin_path}"
  rm -rf "${tmpdir}"
}

install_go() {
  local ver="${1:-}"
  log "installing via go install ..."
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
  local ver="${1:-}"
  local tmpdir
  tmpdir=$(mktemp -d)

  local repo_dir="${tmpdir}/src"
  # Build the exact release tag when we know it. If the tagged clone fails,
  # fall back to main WITHOUT the version stamp — never label a build with a
  # version it is not (the pre-fix script cloned main while stamping the
  # resolved tag, which is how 'ws --version' lied during the v0.7.0–v0.9.0
  # release outage).
  local clone_ok=1
  if [ -n "${ver:-}" ]; then
    git clone --depth 1 --branch "${ver}" "https://github.com/${REPO}.git" "${repo_dir}" 2>/dev/null || clone_ok=0
    if [ "${clone_ok}" = "0" ]; then
      log "tag ${ver} not clonable; building main with NO version stamp"
      ver=""
    fi
  fi
  if [ -z "${ver:-}" ] && [ ! -d "${repo_dir}" ]; then
    git clone --depth 1 "https://github.com/${REPO}.git" "${repo_dir}"
  fi
  local ldflags="-s -w"
  if [ -n "${ver:-}" ]; then
    ldflags="${ldflags} -X main.version=${ver}"
  fi
  (cd "${repo_dir}" && go build -ldflags "${ldflags}" -o "${tmpdir}/${BINARY}.bin" "./cmd/${BINARY}")
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
    cp "$src" "$dst"
    chmod +x "$dst"
  else
    log "sudo required to write to ${dst}"
    sudo cp "$src" "$dst"
    sudo chmod +x "$dst"
  fi

  log "installed ${dst}"

  # Verify the binary we just placed — NOT whatever 'ws' happens to be
  # first in PATH (a stale system install would otherwise report its own
  # version here and confuse the user).
  if [ -x "${dst}" ]; then
    local installed_ver
    installed_ver=$("${dst}" --version 2>/dev/null || echo "unknown")
    log "version: ${installed_ver}"
  fi
  local bin_dir
  bin_dir=$(dirname "${dst}")
  case ":${PATH}:" in
    *":${bin_dir}:"*) : ;;
    *) log "WARN: ${bin_dir} is not in your PATH. Add this to your shell profile:"
       log "  export PATH=\"${bin_dir}:\${PATH}\"" ;;
  esac
}

# ---------- main ----------

main() {
  local os_name arch ver
  os_name=$(get_os)
  arch=$(get_arch)

  [ "$os_name" != "unknown" ] || die "unsupported OS: $(uname -s)"

  log "detected platform: ${os_name}/${arch}"

  # Resolve version: latest RELEASE by default; VERSION= pins a tag.
  ver=""
  if ver=$(resolve_version); then
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
    log "prebuilt binary unavailable, trying source build ..."
  fi

  # 2) Source build at the release tag (needs git + go) — preferred over
  #    go install because it injects the version string via ldflags
  #    (go install of a local module without a release tag shows 'dev').
  if command_exists git && command_exists go; then
    if install_source "$ver"; then
      log "success (built from source @ ${ver:-main})"
      return 0
    fi
    log "source build failed, trying go install ..."
  fi

  # 3) Try go install as last resort (version string will be 'dev' unless
  #    a tag was resolved; go install resolves the tag properly)
  if command_exists go; then
    if install_go "$ver"; then
      log "success (go install)"
      return 0
    fi
  fi

  die "could not install ${BINARY}. Please install Go (https://go.dev/dl) and run:
  go install github.com/${REPO}/cmd/${BINARY}@latest"
}

main "$@"

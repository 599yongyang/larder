#!/usr/bin/env bash
set -euo pipefail

cd "$(dirname "$0")/.."
export PATH="$PWD/.tools/bin:$PATH"
export npm_config_cache="$PWD/frontend/.npm-cache"

target="${1:-all}"
if [[ "$target" == "-h" || "$target" == "--help" ]]; then
  cat <<'USAGE'
Usage: ./scripts/package.sh [macos|windows|all]

Environment:
  ARCH=amd64|arm64       Windows target architecture (default: amd64)
  INSTALL_SCOPE=user     NSIS install scope (default: task default, machine)
  SKIP_DMG=1             macOS: only build Larder.app, skip the DMG

Outputs:
  macOS:   bin/Larder.app, bin/*.dmg
  Windows: bin/larder-<arch>-installer.exe
USAGE
  exit 0
fi

if ! command -v wails3 >/dev/null 2>&1; then
  echo "wails3 not found. Add the repo Wails toolchain to PATH first:" >&2
  echo "  export PATH=\"$PWD/.tools/bin:\$PATH\"" >&2
  exit 1
fi

package_macos() {
  echo "==> packaging macOS app"
  wails3 task package
  if [[ -d bin/Larder.app ]]; then
    echo "==> packaging macOS updater zip"
    rm -f bin/Larder-darwin-arm64.zip
    (cd bin && zip -qry Larder-darwin-arm64.zip Larder.app)
  fi
  if [[ "${SKIP_DMG:-0}" == "1" ]]; then
    echo "Skipped DMG (SKIP_DMG=1)"
  else
    echo "==> packaging macOS DMG"
    wails3 task darwin:package:dmg
  fi
  echo
  echo "macOS artifacts:"
  find bin -maxdepth 1 \( -name '*.app' -o -name '*.dmg' -o -name 'Larder-*.zip' \) -print | sort
}

package_windows() {
  local arch="${ARCH:-amd64}"
  if ! command -v makensis >/dev/null 2>&1; then
    echo "makensis not found. Install NSIS first:" >&2
    echo "  brew install makensis" >&2
    exit 1
  fi
  echo "==> packaging Windows installer (${arch})"
  wails3 task windows:package ARCH="$arch" ${INSTALL_SCOPE:+INSTALL_SCOPE="$INSTALL_SCOPE"}
  echo
  echo "Windows artifacts:"
  ls -lh "bin/larder-${arch}-installer.exe"
}


write_checksums() {
  local files=()
  [[ -f bin/Larder.dmg ]] && files+=("Larder.dmg")
  [[ -f bin/Larder-darwin-arm64.zip ]] && files+=("Larder-darwin-arm64.zip")
  [[ -f bin/larder-amd64-installer.exe ]] && files+=("larder-amd64-installer.exe")
  if [[ ${#files[@]} -gt 0 ]]; then
    (cd bin && shasum -a 256 "${files[@]}" > checksums.txt)
    echo
    echo "checksums:"
    cat bin/checksums.txt
  fi
}

case "$target" in
  macos) package_macos ;;
  windows) package_windows ;;
  all) package_macos; echo; package_windows ;;
  *) echo "Unknown target: $target" >&2; echo "Use: macos, windows, or all" >&2; exit 1 ;;
esac

write_checksums

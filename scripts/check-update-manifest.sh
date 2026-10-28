#!/usr/bin/env bash
set -euo pipefail

# Validate updates/macos-arm64.json against a built updater archive.
#
# Usage:
#   ./scripts/check-update-manifest.sh [version] [archive]
#
# Set VERIFY_REMOTE=1 to also require the artifact URL to answer with a
# successful HTTP status.

cd "$(dirname "$0")/.."

version="${1:-}"
archive="${2:-}"
manifest="updates/macos-arm64.json"

if [[ ! -f "$manifest" ]]; then
  echo "missing manifest: $manifest" >&2
  exit 1
fi

if [[ -z "$version" ]]; then
  version="$(python3 -c 'import json; print(json.load(open("version.json"))["version"])')"
fi
if [[ -z "$archive" ]]; then
  if [[ -f "bin/${version}/Larder-darwin-arm64.zip" ]]; then
    archive="bin/${version}/Larder-darwin-arm64.zip"
  elif [[ -f "bin/Larder-darwin-arm64.zip" ]]; then
    archive="bin/Larder-darwin-arm64.zip"
  else
    echo "missing updater archive: bin/${version}/Larder-darwin-arm64.zip or bin/Larder-darwin-arm64.zip" >&2
    exit 1
  fi
fi

python3 - "$version" "$archive" "$manifest" <<'PY'
import base64
import hashlib
import json
import pathlib
import sys

version, archive, manifest_path = sys.argv[1], pathlib.Path(sys.argv[2]), pathlib.Path(sys.argv[3])
data = archive.read_bytes()
manifest = json.loads(manifest_path.read_text())
expected_url = f"https://github.com/599yongyang/larder/releases/download/v{version}/Larder-darwin-arm64.zip"
expected_digest = base64.b64encode(hashlib.sha256(data).digest()).decode()

assert manifest.get("schemaVersion") == 1, "unsupported schemaVersion"
assert manifest.get("version") == version, f"manifest version {manifest.get('version')} != {version}"
artifact = (manifest.get("artifacts") or [])[0]
assert artifact.get("url") == expected_url, f"artifact URL mismatch: {artifact.get('url')}"
assert artifact.get("size") == len(data), f"archive size mismatch: {artifact.get('size')} != {len(data)}"
assert artifact.get("digest") == expected_digest, f"archive digest mismatch: {artifact.get('digest')} != {expected_digest}"
assert artifact.get("platform") == "darwin" and artifact.get("arch") == "arm64", "wrong platform/arch"
print(f"manifest ok: {version}, {len(data)} bytes, sha256 {expected_digest}")
PY

if [[ "${VERIFY_REMOTE:-0}" == "1" ]]; then
  url="$(python3 -c 'import json; print(json.load(open("updates/macos-arm64.json"))["artifacts"][0]["url"])')"
  curl -fsSI "$url" >/dev/null
  echo "remote artifact ok"
fi

#!/usr/bin/env bash
set -euo pipefail

# Generate updates/macos-arm64.json from a built macOS updater archive.
#
# Usage:
#   ./scripts/write-update-manifest.sh [version] [archive]
#
# Defaults:
#   version  -> version.json version
#   archive  -> bin/<version>/Larder-darwin-arm64.zip
#               then bin/Larder-darwin-arm64.zip
#
# The manifest is consumed by the official updater endpoint provider. The
# archive digest is base64 SHA-256, as required by the update manifest schema.

cd "$(dirname "$0")/.."

version="${1:-}"
archive="${2:-}"

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

if [[ ! -f "$archive" ]]; then
  echo "missing updater archive: $archive" >&2
  exit 1
fi

notes="${UPDATE_NOTES:-Larder ${version}}"
mkdir -p updates
python3 - "$version" "$archive" "$notes" <<'PY'
import base64
import hashlib
import json
import pathlib
import sys
from datetime import datetime, timezone

version, archive, notes = sys.argv[1], pathlib.Path(sys.argv[2]), sys.argv[3]
data = archive.read_bytes()
url = f"https://github.com/599yongyang/larder/releases/download/v{version}/Larder-darwin-arm64.zip"
manifest = {
    "schemaVersion": 1,
    "version": version,
    "channel": "stable",
    "name": f"Larder {version}",
    "notes": notes,
    "publishedAt": datetime.now(timezone.utc).strftime("%Y-%m-%dT%H:%M:%SZ"),
    "artifacts": [{
        "url": url,
        "filename": "Larder-darwin-arm64.zip",
        "filetype": "zip",
        "size": len(data),
        "platform": "darwin",
        "arch": "arm64",
        "digestAlgo": "sha256",
        "digest": base64.b64encode(hashlib.sha256(data).digest()).decode(),
    }],
}
path = pathlib.Path("updates/macos-arm64.json")
path.write_text(json.dumps(manifest, indent=2) + "\n")
print(f"wrote {path}")
print(f"version: {version}")
print(f"size:    {len(data)}")
print(f"sha256:  {base64.b64encode(hashlib.sha256(data).digest()).decode()}")
PY

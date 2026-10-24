#!/usr/bin/env bash
set -euo pipefail
cd "$(dirname "$0")/.."
export PATH="$PWD/.tools/bin:$PATH"
export npm_config_cache="$PWD/frontend/.npm-cache"
exec wails3 dev -config ./build/config.yml -port "${LARDER_DEV_PORT:-9245}"

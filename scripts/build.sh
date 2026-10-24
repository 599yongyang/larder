#!/usr/bin/env bash
set -euo pipefail
cd "$(dirname "$0")/.."
export PATH="$PWD/.tools/bin:$PATH"
export npm_config_cache="$PWD/frontend/.npm-cache"
wails3 build
echo
echo "Built: $PWD/bin/larder"

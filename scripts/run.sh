#!/usr/bin/env bash
set -euo pipefail
cd "$(dirname "$0")/.."
if [[ ! -d bin/larder.dev.app ]]; then
  echo "No app bundle yet. Building first…"
  scripts/build.sh
fi
open bin/larder.dev.app

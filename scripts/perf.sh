#!/usr/bin/env bash
set -euo pipefail

# Quick resource check for a locally running Larder build (macOS).
cd "$(dirname "$0")/.."

pid=$(pgrep -f 'bin/larder.dev.app/Contents/MacOS/larder' | head -1 || true)
if [[ -z "${pid}" ]]; then
  echo "Larder is not running. Start it with ./scripts/run.sh first." >&2
  exit 1
fi

echo "== process =="
ps -o pid=,rss=,%cpu=,etime=,command= -p "${pid}"

data_dir="${LARDER_DATA_DIR:-${HOME}/Library/Application Support/Larder}"
echo "== data =="
if [[ -d "${data_dir}" ]]; then
  du -sh "${data_dir}"
else
  echo "missing ${data_dir}"
fi

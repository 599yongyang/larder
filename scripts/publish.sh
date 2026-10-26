#!/usr/bin/env bash
set -euo pipefail

# Publish a clean snapshot of local main to the public remote.
# Local main keeps development history; the public main gets one clean commit.

cd "$(dirname "$0")/.."

if [[ -n "$(git status --porcelain --untracked-files=normal)" ]]; then
  echo "Working tree is not clean. Commit or stash changes first." >&2
  exit 1
fi

git fetch origin

parent=$(git rev-parse origin/main)
tree=$(git rev-parse main^{tree})
message="${1:-Larder update}"

if git diff --quiet "$parent" "$tree"; then
  echo "No changes between local main and origin/main."
  exit 0
fi

commit=$(printf '%s\n' "$message" | git commit-tree "$tree" -p "$parent")
git branch -f open-source-0.0.4 "$commit"
git push origin open-source-0.0.4:main

echo "Published $message"
echo "Remote main: $(git rev-parse --short "$commit")"

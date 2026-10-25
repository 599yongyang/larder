#!/usr/bin/env bash
set -euo pipefail

VERSION="${1:?usage: scripts/set-version.sh VERSION}"
[[ "$VERSION" =~ ^[0-9]+\.[0-9]+\.[0-9]+$ ]] || { echo "version must be x.y.z" >&2; exit 1; }
cd "$(dirname "$0")/.."

export VERSION
perl -0pi -e 's/var Version = "[0-9]+\.[0-9]+\.[0-9]+"/var Version = "$ENV{VERSION}"/' cmd/larder/service.go
perl -0pi -e 's{larder/[0-9]+\.[0-9]+\.[0-9]+ update-check}{larder/$ENV{VERSION} update-check}' internal/platform/fetch.go
perl -0pi -e 's/(version: ")[0-9]+\.[0-9]+\.[0-9]+(")/$1$ENV{VERSION}$2/' build/config.yml
perl -0pi -e 's/(<string>)[0-9]+\.[0-9]+\.[0-9]+(<\/string>)/$1$ENV{VERSION}$2/g' build/darwin/Info.plist build/darwin/Info.dev.plist
perl -0pi -e 's/("file_version": ")[0-9]+\.[0-9]+\.[0-9]+(")/$1$ENV{VERSION}$2/; s/("ProductVersion": ")[0-9]+\.[0-9]+\.[0-9]+(")/$1$ENV{VERSION}$2/' build/windows/info.json
perl -0pi -e 's/(INFO_PRODUCTVERSION ")[0-9]+\.[0-9]+\.[0-9]+(")/$1$ENV{VERSION}$2/' build/windows/nsis/wails_tools.nsh
VERSION4="$VERSION.0"; export VERSION4
perl -0pi -e 's/(Version=")[0-9]+\.[0-9]+\.[0-9]+\.0(")/$1$ENV{VERSION4}$2/' build/windows/msix/app_manifest.xml build/windows/msix/template.xml
perl -0pi -e 's/(version=")[0-9]+\.[0-9]+\.[0-9]+(" processorArchitecture)/$1$ENV{VERSION}$2/' build/windows/wails.exe.manifest
perl -0pi -e 's/("version": ")[0-9]+\.[0-9]+\.[0-9]+(")/$1$ENV{VERSION}$2/' frontend/package.json
python3 - <<'PY'
import json
from pathlib import Path
p = Path('frontend/package-lock.json')
data = json.loads(p.read_text())
data['version'] = __import__('os').environ['VERSION']
data['packages']['']['version'] = __import__('os').environ['VERSION']
p.write_text(json.dumps(data, indent=2) + '\n')
PY
perl -0pi -e 's/`[0-9]+\.[0-9]+\.[0-9]+`/`$ENV{VERSION}`/g' README.md README.zh-CN.md
perl -0pi -e 's/var Version = "[0-9]+\.[0-9]+\.[0-9]+"/var Version = "$ENV{VERSION}"/' docs/RELEASE.md
perl -0pi -e 's/\.\/scripts\/publish\.sh "Larder [0-9]+\.[0-9]+\.[0-9]+"/.\/scripts\/publish.sh "Larder $ENV{VERSION}"/' docs/DEVELOPMENT.md
perl -0pi -e 's/open-source-[0-9]+\.[0-9]+\.[0-9]+/open-source-$ENV{VERSION}/g' scripts/publish.sh
chmod +x scripts/set-version.sh
echo "Version set to $VERSION"

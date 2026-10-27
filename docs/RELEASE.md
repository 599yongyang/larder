# Release

Larder releases are local-first desktop builds. Nothing is published automatically from a development checkout.

## Before building

```sh
cd frontend
npx tsc --noEmit
npm test
npm run build
cd ..
go test ./...
go vet ./...
```

## Build

Build the macOS app bundle and DMG:

```sh
./scripts/package.sh macos
```

Build a Windows installer from macOS (requires NSIS):

```sh
brew install makensis
./scripts/package.sh windows
```

Build both:

```sh
./scripts/package.sh all
```

The build must run on a machine with the platform toolchain available:

- macOS: Xcode command line tools
- Windows: WebView2 toolchain

## Version

The app version lives in `cmd/larder/service.go`:

```go
var Version = "0.0.6"
```

The public release tag and the in-app version must match.

Upload `checksums.txt` alongside `Larder.dmg` and the Windows installer. The in-app updater downloads the DMG/EXE, verifies it against this file, then opens the installer.

## macOS signing

Unsigned builds trigger Gatekeeper. A public binary release should be signed and notarized:

```text
Developer ID Application certificate
codesign --deep --force --options runtime --sign "Developer ID Application: ..." Larder.app
xcrun notarytool submit Larder.zip --wait
xcrun stapler staple Larder.app
```

Never publish a signed binary claiming notarization unless the notarization succeeded.

## Windows

Windows builds are unsigned for now. Mention SmartScreen in release notes instead of silently shipping an unsigned executable.

## Release notes

Include:

- what changed
- version
- supported platforms
- known limitations
- whether the build is signed
- rollback/snapshot behavior if write operations changed

## Do not

- Do not attach local `bin/`, `dist/`, `preview/`, `.tools/`, or node_modules contents.
- Do not publish a release from the development `main` history. Publish a clean snapshot.
- Do not claim Windows or another desktop platform is fully supported until its acceptance workflow is green.

# Larder

**Local provisions for your agents.**

[English](README.md) · [简体中文](README.zh-CN.md)

Larder is a local-first desktop app for developers who work with multiple AI agents. It shows what is actually installed on your machine — Skills, MCP servers, CLI tools, and instruction files — and lets you manage them safely.

![Skills overview](docs/screenshots/skills-cards-en.png)

## What it does

- **Skills** — scan every Agent directory and shared library, inspect sources, token cost and visibility, then install, update, uninstall, repair links or sync copies.
- **MCP servers** — inspect global and project configuration, add or edit servers, toggle them per Agent, and test whether a command or URL is reachable.
- **CLI tools** — see what is installed, which package manager owns it, duplicates and available updates. Update or uninstall through the package manager.
- **Instruction files** — scan `AGENTS.md`, `CLAUDE.md`, Cursor rules and Copilot instructions. Preview Markdown or raw content, edit existing files, or create new ones.
- **Projects** — see the Skills, MCP servers and instruction files that belong to a project in one place.
- **Language** — switch between English and Chinese in Settings; the app follows the system language on first launch.

![MCP overview](docs/screenshots/mcp-en.png)

## Safe by default

Larder reads local files before it writes anything.

- Every write has a visible action or plan.
- Configuration files are snapshotted before modification.
- Failures are shown instead of silently ignored.
- Local data stays local. No account, no file upload, no telemetry.

## Build from source

Requirements:

- macOS 13+ (current primary platform)
- Windows 10 1809+ / 11 (build and acceptance path included; WebView2)
- Go 1.26+
- Node.js 24+
- Wails v3 CLI

macOS is the current primary target. The Windows build and test path is already in the repository; more desktop platforms are planned.

```sh
git clone <repo-url>
cd larder
./scripts/run.sh
```

Useful scripts:

```sh
./scripts/run.sh    # build if needed, then open the app
./scripts/dev.sh    # development mode with frontend hot reload
./scripts/build.sh  # rebuild the app
```

There is no signed release build yet.

On macOS, a locally built app may require:

```sh
xattr -dr com.apple.quarantine /path/to/Larder.app
```

On Windows, an unsigned build may show a SmartScreen warning.

## Status

`0.0.1` — early development. The current focus is local truth, safe writes and the daily workflow around Skills and MCP.

## License

[MIT](LICENSE)

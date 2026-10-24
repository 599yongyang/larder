# Architecture

Larder is a local-first desktop application. The UI never reads user files directly; all filesystem and configuration access goes through the Go service.

## Layers

```text
main.go                  Wails shell, window and tray
cmd/larder/                 AppService: UI-facing API and state
internal/core/           scanning, inventory, operations, backups
internal/platform/       paths, filesystem, platform adapters
frontend/src/            React UI
```

### AppService

`cmd/larder` owns:

- startup scan and cache loading
- state events to the frontend
- plan generation and apply
- MCP and instruction file operations
- settings, backup and restore

It is the only bridge between the frontend and the machine.

### Core

`internal/core` is UI-independent and contains:

- Skill, CLI, MCP and instruction scanning
- update checks
- issue detection
- safe write plans
- snapshots and rollback metadata
- local SQLite cache
- catalog and registry clients

### Frontend

The frontend is React + Vite. It calls the generated Wails bindings through `frontend/src/backend.ts`.

Page structure:

```text
Skills
MCP
CLI
Instructions
Project context
History
Settings
```

## Data flow

```text
filesystem / agent configs
        ↓
   core scanners
        ↓
   snapshot + SQLite cache
        ↓
   AppService
        ↓
   Wails bindings
        ↓
   React UI
```

Writes use the reverse path:

```text
UI action
  → plan
  → explicit confirmation
  → snapshot
  → write
  → operation record
  → rescan
```

## Safety model

- Scans are read-only.
- Every write has a visible plan or an explicit action.
- Config files and Skill files are snapshotted before modification.
- Failures are surfaced; there is no silent fallback.
- Rollback is available for the operations that store enough snapshot data.
- Tests use injected temporary home/data roots; they never touch real user files.

## Local data

macOS:

```text
~/Library/Application Support/Larder/
```

Windows:

```text
%APPDATA%\Larder\
```

Contains:

- `ledger.sqlite`
- `snapshots/`
- `backups/`
- `instruction-backups/`
- `disabled-mcp.json`

## Build

```sh
./scripts/build.sh
```

Wails embeds the production frontend build. TypeScript bindings are generated with:

```sh
wails3 generate bindings -clean -ts
```


## Localized messages

User-facing text generated in Go is returned as `Message` values:

```go
type Message struct {
    Code     string
    Params   map[string]string
    Fallback string
}
```

The frontend resolves `Code + Params` through i18next. Plans use this for titles and warnings. New backend-facing copy should follow the same shape instead of returning a hard-coded sentence.

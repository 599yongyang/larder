# Security and privacy

Larder works with local files and Agent configuration. The security model is deliberately conservative.

## Principles

- Local-first: file contents are not uploaded.
- Read-only by default: scanning never modifies files.
- Explicit writes: installation, update, uninstall, linking, MCP changes and instruction edits are user-triggered.
- Snapshot before write: config files and Skill files are copied before modification.
- Visible failures: errors are surfaced in the UI instead of being silently ignored.

## What leaves the machine

Only network requests needed for product features:

- Skills catalog requests to `skills.sh`
- MCP Registry requests to `registry.modelcontextprotocol.io`
- Update metadata requests to package registries
- Skill source downloads when the user installs from a catalog
- Optional HTTP reachability checks when the user tests an MCP server

File contents are not sent as telemetry.

## Sensitive data

MCP environment variables and HTTP headers can contain tokens. The UI shows field names, not values. Disabled MCP entries and backups can contain secret values on disk; files are written with restricted permissions where possible, and the backup UI warns that backups are not encrypted.

## Writing

All write operations must:

1. Explain the target and impact.
2. Create a snapshot or record enough data to restore the previous state.
3. Report success or failure visibly.
4. Leave an operation record.

Tests must use temporary roots and must not modify real Agent directories.

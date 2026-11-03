# Changelog

## 0.0.10

### Hotfix

- Fix MCP servers launched from Finder failing with `env: node: No such file or directory` by giving child processes a usable PATH.
- Fix MCP status checks, CLI version probes, native CLI updates and package-manager checks in the same GUI environment.
- Wrap Windows `.cmd` / `.bat` shims through `cmd.exe`.

## 0.0.9

### MCP

- Add SDK-backed MCP inspection for tools, resources, resource templates and prompts.
- Keep live MCP sessions for tool calls, resource reads and prompt rendering, with capability caching and protocol fallback.
- Add tool argument forms, JSON mode for structured arguments, execution history, cancellation and JSON-RPC logs.
- Render prompt messages, resource text, images, binary downloads and resource links.
- Add remote MCP header authentication, OAuth status and authorization cleanup.
- Add official MCP Registry discovery, vendor search, pagination and runtime/auth badges.
- Add inline editing for launch settings, configuration fields, headers and agent targets, with snapshot and rollback support.
- Add support for adding an MCP server to a new agent configuration, including missing config files.

### Windows

- Support Windows directory junctions for link creation, unmounting, snapshots and rollback.
- Resolve MCP config paths from APPDATA and recognize Windows VS Code configuration files.
- Support Windows environment expansion, path separators and Claude session folder encoding.
- Compare Windows session paths case-insensitively.
- Add PowerShell resume commands and terminal launch support.
- Hide console windows for MCP child processes.

### UI

- Restore shared styles that were lost during the MCP iteration.
- Add missing common translation labels.

## 0.0.8

### Data and sync

- Add selectable data packages for settings, app state, MCP configs, instructions, commands and experimental Skills.
- Add manual WebDAV and S3-compatible remote sync with connection tests, upload, download and last-sync status.
- Encrypt remote packages with AES-256-GCM and keep WebDAV, S3 and encryption secrets in the macOS Keychain.
- Require HTTPS outside localhost, explicit plaintext confirmation, and detect remote package changes before overwriting.
- Map package paths onto the current home, data directory and matching project roots.
- Make imports transactional and roll back files when applying imported settings fails.

### Updates

- Replace the separate updater window with an in-app update dialog.
- Fall back to the static update manifest when GitHub is unavailable.
- Localize native menus and harden update checks.

### Desktop

- Add native app menus, single-instance behavior, reveal in Finder, autostart, clipboard support, folder drops, and a configurable global shortcut.
- Add session resume, quick actions and session detail sheets.

### Source resolution

- Resolve skill sources from agent lockfiles, optional library indexes, frontmatter and the bundled catalog.
- Propagate sources to identical copies, label local installs and allow manual source binding.

### UI

- Rebuild Settings as one scrolling page with a scroll-linked section navigation.
- Unify Select, Textarea, SegmentedControl, Checkbox, dialogs and border radii around the Larder design tokens.
- Remove the duplicated backup/restore flow now covered by data packages.

## 0.0.7

- Add the in-app data inspector, agent sessions, CLI metadata and update polish.

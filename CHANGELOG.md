# Changelog

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

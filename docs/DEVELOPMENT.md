# Development

## Requirements

- macOS 13+ (primary)
- Windows 10 1809+ / 11 (build and acceptance path)
- Go 1.26+
- Node.js 24+
- Wails v3 CLI

## Commands

```sh
./scripts/dev.sh    # Wails dev mode
./scripts/build.sh  # production build
./scripts/run.sh    # open the built app
```

Frontend:

```sh
cd frontend
npm ci
npm test
npm run build
npx tsc --noEmit
```

Go:

```sh
go test ./...
go vet ./...
```

Bindings:

```sh
wails3 generate bindings -clean -ts
```

## Test isolation

Never touch the real user home in tests. Use a temporary home and data directory.

The inventory tests show the expected pattern:

```sh
t.Setenv("LARDER_HOME", home)
```

## Frontend conventions

- Page toolbars use the shared `filterBar` / `filterLeft` / `filterRight` layout.
- Page search uses `AssetSearch`.
- Detail sheet actions use `xs` buttons.
- Dialog footer actions use `sm` buttons.
- Dialog inputs stay compact: 30px high, 11px text.
- See [UI_RULES.md](UI_RULES.md) before adding a new page.

## 发布

`main` 是本地开发分支，保留完整开发历史。

公开 Release 使用 **PR 合并** 到公开仓库的 `main`，不要用
`scripts/publish.sh` 创建 clean-snapshot 提交。发布 PR 标题使用：

```text
chore(release): Larder x.y.z
```

hotfix：

```text
chore(release): Larder x.y.z hotfix
```

合并 PR 后，在合并提交上打 `vX.Y.Z` tag 并上传构建产物。不要把
`Larder x.y.z` 作为公开 `main` 的提交信息。

`v0.0.11` 曾用快照流程发布，作为例外保留，不重写历史。

## 提交前

```sh
cd frontend && npx tsc --noEmit && npm test && npm run build
cd .. && go test ./... && go vet ./...
```

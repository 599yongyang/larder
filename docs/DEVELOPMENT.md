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

公开仓库的 `main` 使用干净快照：

```sh
./scripts/publish.sh "Larder 0.0.1"
```

脚本会：

- 检查工作区是否干净
- 以 `origin/main` 为父提交
- 用当前 `main` 的 tree 创建一个新 commit
- 推送到公开 `main`

公开历史不会包含本地开发过程。

## 提交前

```sh
cd frontend && npx tsc --noEmit && npm test && npm run build
cd .. && go test ./... && go vet ./...
```

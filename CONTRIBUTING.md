# Contributing

## 开发

```sh
./scripts/dev.sh
```

## 验证

提交前至少运行：

```sh
cd frontend
npx tsc --noEmit
npm test
npm run build

cd ..
go test ./...
go vet ./...
```

## UI

新增页面或组件前先阅读 [`docs/DEVELOPMENT.md`](docs/DEVELOPMENT.md) 与 [`docs/UI_RULES.md`](docs/UI_RULES.md)，保持工具栏、搜索、按钮和弹窗尺寸一致。

## Commit

- 一个 commit 只做一件事
- 修复优先附带回归测试
- 不在 commit 中提交本机路径、备份、数据库或缓存

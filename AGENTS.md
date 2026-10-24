# AGENTS.md

Larder 是 AI 与贡献者的工作入口。修改代码前先读这里，再读相关文档。

## 项目

本机 Agent 工具与配置管理桌面应用：

- 扫描 Skills、MCP、CLI、Agent 指令文件
- 展示来源、状态、可见性和依赖
- 所有写操作先生成快照

技术形态：

```text
Go 业务核心 + React 前端 + Wails v3 桌面壳
macOS 优先；Windows 构建与验收路径已接入，后续覆盖更多桌面平台
```

## 命令

```sh
# 开发
./scripts/dev.sh

# 构建
./scripts/build.sh

# 前端
cd frontend
npx tsc --noEmit
npm test
npm run build

# Go
go test ./...
go vet ./...

# 生成 Wails TypeScript bindings
wails3 generate bindings -clean -ts
```

`.tools/bin` 是仓库本地工具链；`scripts/*.sh` 会自动处理 PATH 和 npm 缓存。

## 架构边界

```text
main.go                 Wails 外壳，只做窗口和服务装配
cmd/larder/                AppService，连接 UI 与业务核心
internal/core/          扫描、操作、备份、更新、体检
internal/platform/      路径和文件系统适配
frontend/src/           React UI、feature 组件、样式
```

- `internal/core` 不依赖 Wails 或 React。
- 前端不直接操作文件系统或配置文件；只调用 AppService。
- 所有文件写入必须可解释、可回滚。
- 生成文件（bindings、dist）不手工编辑。

## 研发规则

1. 先读现有实现，再改代码。
2. 扫描和测试不能触碰真实用户文件；使用 `LARDER_HOME`/`LARDER_DATA_DIR` 注入临时根。
3. 任何写入操作必须有快照或操作记录。
4. 失败不能静默吞掉，必须有可见错误。
5. UI 先读 [docs/UI_RULES.md](docs/UI_RULES.md)。
6. 修复 bug 时补对应测试；没有测试的改动要说明原因。
7. 不提交个人信息、真实路径、数据库、缓存、备份或 `.env`。

## 相关文档

- [README](README.md)
- [架构](docs/ARCHITECTURE.md)
- [开发](docs/DEVELOPMENT.md)
- [UI 规则](docs/UI_RULES.md)
- [安全](docs/SECURITY.md)
- [领域模型](docs/DOMAIN.md)
- [术语表](docs/GLOSSARY.md)

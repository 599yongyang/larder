# Domain model

Larder 管理本机上的四类资源。它们来源不同，但都服务于 AI Agent 的工作环境。

## Skill

包含 `SKILL.md` 的目录，给 Agent 提供任务说明、脚本或参考文件。

一个 Skill 可以通过链接或副本出现在多个 Agent 目录中。来源、路径和内容哈希共同决定它是不是同一个 Skill。

## CLI

从 shell 调用、被 Skill 依赖或被用户关注的命令行工具。

CLI 有安装来源和管理者：Homebrew、npm、Bun、Cargo、Conda、pipx、uv、standalone 等。同名命令可能由多个来源安装。

## MCP Server

通过 stdio 或 HTTP 提供的 MCP 服务。配置可以存在于全局 Agent 配置，也可以存在于项目目录。

Larder 关注配置存在于哪些 Agent、传输方式、启动命令、环境变量字段和连通性，不负责实现 MCP Server。

## Instruction file

Agent 会读取的说明文件或规则目录：

```text
AGENTS.md
CLAUDE.md
GEMINI.md
.cursorrules
.cursor/rules/*.mdc
.github/copilot-instructions.md
```

作用范围可以是用户级、项目级或目录级。

## Relations

- Skill 可以通过 `bins` 声明依赖 CLI。
- MCP 的启动命令可能依赖 CLI。
- 项目聚合项目内 Skill、项目 MCP 和项目指令文件。
- 移除或更新一个 CLI 前，需要知道哪些 Skill 和 MCP 在使用它。

## Visibility

Skill 在某个 Agent 目录中的形态：

```text
linked       链接
copied       独立副本
missing      不存在
broken       链接失效
unavailable  无法读取
```

可见性表示 Agent 是否会发现它，不等同于运行时已经加载。

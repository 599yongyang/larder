# Glossary

UI 文案、代码命名和文档统一使用以下词汇。

## Skills

| 术语 | 英文 | 定义 |
|---|---|---|
| **Skill** | skill | 包含 `SKILL.md` 的目录，给 Agent 提供任务说明。 |
| **中央库** | central store | 集中存放 Skill 的目录，本机通常是 `~/.agents/skills`。 |
| **Agent 目录** | agent skills dir | Agent 实际读取 Skill 的路径。一个 Agent 可以有多个目录。 |
| **项目 Skill** | project skill | 位于项目工作区内、只对项目生效的 Skill。 |
| **链接形态** | link mode | Skill 在 Agent 目录中的存在方式：linked、copied、missing、broken。 |
| **副本** | copy | Agent 目录中独立复制的 Skill，可能与中央库内容不一致。 |
| **可见性** | visibility | Agent 是否按已知规则能发现某个 Skill。 |
| **漂移** | drift | 本地内容与安装基线或上游不一致。 |
| **坏链** | broken link | 指向不存在目标的链接。 |
| **来源** | source | Skill 的上游仓库，来自 lockfile 或本地记录。 |
| **Lockfile** | lockfile | Skills CLI 的安装记录，用于判断来源和基线。 |

## CLI

| 术语 | 英文 | 定义 |
|---|---|---|
| **CLI** | cli | 可从 shell 调用的命令。 |
| **owner** | owner | CLI 的实际管理者，例如 Homebrew、npm、uv、Cargo。 |
| **重复安装** | duplicate install | 同一命令由多个来源安装。 |
| **路径遮蔽** | PATH shadowing | 同名命令在 PATH 中先命中了非预期的来源。 |
| **standalone** | standalone | 由厂商自带更新器维护的独立二进制。 |

## Operations

| 术语 | 英文 | 定义 |
|---|---|---|
| **预演** | dry-run | 展示将执行的命令、影响面和 diff，但不落盘。 |
| **快照** | snapshot | 写操作前的文件备份，用于回滚。 |
| **影响面** | impact | 一次操作会波及的 Skill、Agent、CLI 或 MCP。 |
| **体检** | doctor | 坏链、重复安装、漂移、缺失依赖等规则的集合。 |
| **依赖图** | dependency graph | Skill、MCP 与 CLI 之间的依赖关系。 |
| **上下文成本** | context budget | 每轮对话为 Skill 名称和描述付出的 token。 |

## Technical

| 术语 | 英文 | 定义 |
|---|---|---|
| **core** | core | 纯 Go 业务核心，不依赖 UI 框架。 |
| **shell** | shell | Wails 桌面壳，负责窗口、托盘、绑定和生命周期。 |
| **platform adapter** | platform adapter | macOS / Windows 的路径、链接、进程和包管理器适配。 |
| **junction** | junction | Windows 目录联接，作为 symlink 的替代方案。 |
| **WebView2** | WebView2 | Windows 系统 WebView 运行时。 |

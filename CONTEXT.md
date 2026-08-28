# Skillsend — CONTEXT

Skillsend 是一个本地 Agent Skills 管理小工具：以一个可自定义路径的 git 仓库（**hub**）为唯一真源，通过 per-skill symlink 把技能「借给」`~/.agents/skills` 与 `~/.claude/skills`，提供丝滑、轻量、毫秒响应的 TUI。TUI 是**全功能管理面板**（Skills / Targets / Hub 三个视图），不只是选择器：浏览、切换、更新、收编、搜索、编辑入口、hub 仓库同步与过时检查都在面板内完成。

定位：**纯本地库管理器**。从网上安装 skills 是 `npx skills` / `gh skill` 已解决的问题，Skillsend 不做。

## 词汇表

输出中提到这些概念时，必须使用下面的术语，不要漂移到同义词。

- **Hub（真源）**：唯一存放 skill 实体文件夹的 git 仓库。路径可自定义（每台机器可在配置中指向不同路径，如新机器上 `git clone` 后指向新路径）。
- **Skill**：一个含 `SKILL.md` 的文件夹（Agent Skills 格式，见 agentskills.io）。
- **Target**：链接目的地目录。v1 写死两个：`~/.agents/skills` 与 `~/.claude/skills`。
- **Link（直链）**：`target/<skill> → hub/<skill>` 的单跳 symlink。**启用 = 链接存在；禁用 = 链接不存在**。不做链式链接、不做整目录链接。
- **外部条目（Foreign entry）**：target 目录中不指向 hub 的链接或实体目录（如 `deveco-cli`）。工具只显示、永不改动。
- **收编（Adopt）**：把外部实体 skill 目录移入 hub 并在原位建直链的**显式一次性操作**。
- **Upstream**：第三方 skill 的来源 git URL，记录在该 skill 的元数据中；更新时逐个 fast-forward pull。

## 不变量（invariants）

1. Skill 实体只存在于 hub；工具绝不把实体复制进 target。
2. 真实目录只通过**用户显式确认的操作**被改动（如收编时移动目录）；除此之外工具只增删 symlink，永不创建、删除、移动或修改真实目录。删除非本工具创建的链接必须逐个确认。
3. 不主动迁移、重写或「修复」既有链接（包括既有的链式链接 `claude → agents → my_dev`）；一切变更由用户显式触发。
4. 「已启用」的判定 = 链接解析（任意跳数）最终指向 hub 内部。
5. 更新操作只做 fast-forward；本地有改动的第三方 skill 跳过并提示，绝不 merge、绝不丢弃改动。

## 关键决策

见 `docs/adr/`：0001 范围、0002 技术栈、0003 链接模型、0004 真源与更新语义。

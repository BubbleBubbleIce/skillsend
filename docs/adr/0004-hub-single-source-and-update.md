# ADR-0004: hub 唯一真源，两级更新语义，显式收编

日期：2026-08-28 · 状态：已接受

## 背景

用户机器上 skill 实体分裂在两处：自己的 skills 在 `~/my_dev/skills`（git 仓库，远端 `BubbleBubbleIce/skills`）；Matt Pocock 的 ~15 个 skills 是真实目录直接住在 `~/.agents/skills`，不在任何版本控制下，上游更新无法跟进。用户选择 A 档（完整方案），并新增要求：**hub 路径可自定义**——换新电脑时可以新建文件夹作为真源。

## 决策

1. **Hub 是唯一真源**：所有 skill 实体只存在于 hub；target 目录里只允许出现链接（或与工具无关的外部条目）。
2. **更新 = 两级操作**：
   - (a) hub 本身一键 `git pull`（同步远端；symlink 是活的，pull 完所有 agent 立即生效）；
   - (b) 对元数据中记录了 upstream 的第三方 skill，逐个 `git pull`，**只 fast-forward**；本地有改动的跳过并提示。
3. **收编（adopt）是显式操作**：把外部实体 skill 目录移入 hub 并在原位建直链，逐个由用户确认触发；工具绝不批量擅自迁移用户文件。
4. **hub 路径可自定义**：单键配置文件（`~/.config/skillsend/config.toml` 的 `hub = "<路径>"`），首次运行 TUI 引导设置一次。新机器上 `git clone` 到任意路径后指向即可。

## 后果

「更新」从此有明确定义且对第三方 skill 闭环；真源分裂消除。收编作为显式操作保留了用户对自己环境的完全控制权。fast-forward-only 保证更新操作永不制造 merge 冲突、永不丢弃本地改动。

## 实现机制（2026-08-28 代码评审后固化）

- 第三方 skill 以**普通目录**存于 hub，由 hub 仓库版本化；收编时剥离嵌套 `.git`（否则 hub 只会记录 gitlink，破坏唯一真源）。
- `skillsend.toml` 每条记录含 `source` / `ref` / `path` / `synced` / `tree`：`synced` 是上游 commit sha（供 ahead/behind 统计），`tree` 是本地目录的**内容签名**（确定性哈希，忽略 `.git` 与 `.DS_Store`）。
- 「本地是否有改动」= 当前签名 ≠ 记录签名。同时覆盖已提交与未提交改动，且不依赖在 hub 仓库中 diff 上游对象（上游 sha 在 hub 的对象库里不存在，直接 diff 不可行）。
- 更新 = 把上游树物化到暂存目录 → 与当前目录原子交换（失败即回滚），绝不先删后建；完成后改动保持未提交，由用户审阅并提交。`path` 字段支持上游为集合仓库（skill 位于其子目录）。
- `e` 新增上游且无 `synced` 时，首次更新通过对比本地与上游树的签名建立基线：一致则采纳该上游 head 为基线，不一致视为本地改动拒绝。

# ADR-0004: hub 唯一真源，两级更新语义，显式收编

日期：2026-08-28 · 状态：已接受

## 背景

用户机器上 skill 实体分裂在两处：自己的 skills 在 `~/my_dev/skills`（git 仓库，远端 `rookie-oops/skills`）；Matt Pocock 的 ~15 个 skills 是真实目录直接住在 `~/.agents/skills`，不在任何版本控制下，上游更新无法跟进。用户选择 A 档（完整方案），并新增要求：**hub 路径可自定义**——换新电脑时可以新建文件夹作为真源。

## 决策

1. **Hub 是唯一真源**：所有 skill 实体只存在于 hub；target 目录里只允许出现链接（或与工具无关的外部条目）。
2. **更新 = 两级操作**：
   - (a) hub 本身一键 `git pull`（同步远端；symlink 是活的，pull 完所有 agent 立即生效）；
   - (b) 对元数据中记录了 upstream 的第三方 skill，逐个 `git pull`，**只 fast-forward**；本地有改动的跳过并提示。
3. **收编（adopt）是显式操作**：把外部实体 skill 目录移入 hub 并在原位建直链，逐个由用户确认触发；工具绝不批量擅自迁移用户文件。
4. **hub 路径可自定义**：单键配置文件（`~/.config/skillsend/config.toml` 的 `hub = "<路径>"`），首次运行 TUI 引导设置一次。新机器上 `git clone` 到任意路径后指向即可。

## 后果

「更新」从此有明确定义且对第三方 skill 闭环；真源分裂消除。收编作为显式操作保留了用户对自己环境的完全控制权。fast-forward-only 保证更新操作永不制造 merge 冲突、永不丢弃本地改动。

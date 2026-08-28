# ADR-0001: 只做本地库管理器，目标目录写死两个

日期：2026-08-28 · 状态：已接受

## 背景

Agent Skills 管理领域已有成熟工具：`npx skills`（vercel-labs，29.8k★，symlink + update，覆盖 78 个 agent 目录）、`gh skill`（GitHub CLI 官方，provenance + pin）。它们的空白在于：没有以「个人本地 skills 仓库为中心」的漂亮 TUI、没有按目标目录开关 symlink 的交互、不支持 ZCode。

用户的核心诉求是**切换小工具**：丝滑、流畅、响应快、轻量。

## 决策

- Skillsend 只做**本地库管理器**：TUI 浏览 + 开关 symlink + 更新 + 收编。
- 不做「从网上安装 skill」——需要装新 skill 时用现成工具或手工 clone 进 hub。
- 目标目录写死两个：`~/.agents/skills` 与 `~/.claude/skills`。不做配置化的多目标扩展（用户明示「暂时」；将来要加是小改动）。

## 后果

砍掉约一半复杂度（安装源、registry、manifest、多目标探测），TUI 可以专注于切换体验。范围收缩换来的简单性是这个工具能保持轻量的前提。

# ADR-0002: Go + Bubble Tea

日期：2026-08-28 · 状态：已接受

## 背景

核心非功能需求：丝滑、流畅、毫秒冷启动、轻量。候选：Go + Bubble Tea/Lipgloss、Rust + ratatui、TypeScript + Ink、Python + Textual。Node（Ink）和 Python（Textual）需要携带运行时，冷启动与分发形态不满足「轻量、单二进制、即时响应」，直接排除。

## 决策

**Go + Bubble Tea（TUI 框架）+ Lipgloss（样式）**。单二进制约 10MB，冷启动几十毫秒，Lipgloss 做「美观」的性价比高，开发速度快于 Rust。

## 后果

放弃 Rust 更极致的体积与内存占用（对个人工具无实际意义），换取开发迭代速度。Ratatui 若未来迁移，架构上的 hub/链接/更新模型与 UI 层解耦，迁移成本可控。

# ADR-0006: Hub 页支持已知 Git URL 的 Clone 导入

日期：2026-08-30 · 状态：已接受

## 背景

ADR-0001 将网络安装完全交给 `npx skills` / `gh skill`。用户现在希望在 Hub 页直接输入 GitHub 链接并执行 clone，让“已经知道仓库地址”的导入不必离开 TUI。

## 决策

- Hub 页新增 `g`：输入 GitHub 或其他 Git URL 后异步 clone。
- 只接受根目录含 `SKILL.md`，或直接子目录含 `SKILL.md` 的集合仓库，与 hub 现有两层扫描语义一致。
- clone 先落在 hub 内的暂存目录。检查通过后剥离内层 `.git`，再将普通目录导入 hub，避免 hub 只记录 gitlink。
- 为每个导入 Skill 记录 `source/path/synced/tree`，使其立即进入现有 Upstream 过时检查和更新流程。
- 仓库目录或 Skill 叶子名与 hub 现有内容冲突时拒绝导入，不覆盖。
- 这不扩展为 registry 客户端：不做搜索、包解析，也不调用 `npx skills` / `gh skill`。

## 后果

用户可以在 TUI 内完成明确来源的新 Skill 导入，同时继续保持 hub 唯一真源和无嵌套 Git 仓库的更新模型。

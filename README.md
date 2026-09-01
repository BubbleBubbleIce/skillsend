# Skillsend

一个本地 Agent Skills 管理面板：以一个 git 仓库（**hub**）为唯一真源，通过 per-skill symlink 把技能「借给」`~/.agents/skills` 与 `~/.claude/skills`。

Go + Bubble Tea 单二进制，冷启动几十毫秒，TUI 里完成浏览、开关、搜索、编辑、更新、反向审计与仓库同步。

## 它不做的事

**不接管 skill registry。** 搜索、版本解析和 `npx skills` / `gh skill` 的安装流程仍交给对应工具。Skillsend 可以在 Hub 页接收一个明确的 GitHub/Git URL，将其 clone 并导入 hub。

目标目录写死两个（`~/.agents/skills`、`~/.claude/skills`），不做配置化扩展（ADR-0001）。

## 安装

**方式一：下载预编译二进制**（推荐，不需要 Go）。从 [Releases](https://github.com/BubbleBubbleIce/skillsend/releases) 取对应平台的包：

| 平台 | 包 |
| --- | --- |
| macOS Apple Silicon | `skillsend_Darwin_arm64.tar.gz` |
| macOS Intel | `skillsend_Darwin_x86_64.tar.gz` |
| Linux x86_64 | `skillsend_Linux_x86_64.tar.gz` |
| Linux arm64 | `skillsend_Linux_arm64.tar.gz` |

```bash
tar xzf skillsend_Darwin_arm64.tar.gz
sudo install -m 755 skillsend /usr/local/bin/skillsend
skillsend --version
```

> **macOS 首次运行**：二进制未做开发者签名，Gatekeeper 会拦第一次。在访达里右键 `skillsend` → 打开 → 确认即可，之后就正常了。想跳过这一步也可以自己清掉隔离属性：`xattr -d com.apple.quarantine /usr/local/bin/skillsend`。

**方式二：用 Go 安装**：

```bash
go install github.com/BubbleBubbleIce/skillsend@latest
```

**方式三：从源码构建**（需要 Go 1.26+）：

```bash
git clone https://github.com/BubbleBubbleIce/skillsend.git
cd skillsend
go build -o skillsend .
./skillsend
```

不论哪种方式，运行时都依赖 **`git` 在 PATH 中** —— hub 的 pull / push / 提交以及 upstream 更新都是调 git 完成的。除此之外无其他依赖，二进制是静态链接的。

## 快速开始

首次运行会问一次 hub 路径，写入 `~/.config/skillsend/config.toml`：

```toml
hub = "/Users/you/my_dev/skills"
```

之后每次启动直接进面板。换机器时 `git clone` 到任意路径，改这一行即可。

## 模型

```
hub（git repo，唯一真源）          target 目录（只放链接）
├── grilling/SKILL.md     ──▶    ~/.agents/skills/grilling → hub/grilling
├── tdd/SKILL.md                 ~/.claude/skills/grilling  → hub/grilling
├── pdf/SKILL.md
└── skillsend.toml                ⚠ deveco-cli/  ← 外部条目：只显示，不改动
```

- **Hub**：唯一存放 skill 实体的 git 仓库，路径可自定义。
- **Skill**：一个含 `SKILL.md` 的文件夹（[Agent Skills 格式](https://agentskills.io)）。
- **Target**：链接目的地目录，v1 写死两个。目录不存在时不会自动创建。
- **Link（直链）**：`target/<skill> → hub/<skill>` 的单跳 symlink。**启用 = 链接存在，禁用 = 链接不存在。**「已启用」按链接解析（任意跳数）是否最终落在 hub 内判定，因此既有的链式链接也能被正确识别。
- **外部条目**：target 中不指向 hub skill 的链接或实体目录。工具只显示，永不改动。
- **收编（adopt）**：把外部实体 skill 目录移入 hub 并在原位建直链的**显式**操作。
- **Upstream**：第三方 skill 的来源 git URL，记录于 `skillsend.toml`，更新时逐个 fast-forward pull。

## TUI

三个视图（`1` / `2` / `3` 或 `tab` 切换）。

**Skills** — hub 全部 skill 列表 + 详情栏（SKILL.md 预览、upstream、各 target 链接状态、脏状态）。

**Targets** — 反向视图：列出每个 target 目录里的全部条目并标注类型。

| 标记 | 含义 |
| --- | --- |
| `●` | hub 直链（已启用） |
| `○` | 未启用 |
| `◌` / `▣` | 外部链接 / 外部实体目录 |
| `✕` | 断链 |
| `⚠` | 名称与 hub skill 冲突，或存在未提交改动 |

**Hub** — 未提交文件列表，以及 hub 仓库的 pull / push / commit / 过时检查。

### 键位

| 键 | 作用 | 视图 |
| --- | --- | --- |
| `1` `2` `3` / `tab` | 切换视图 | 全局 |
| `j` `k` / `↑` `↓` | 移动光标 | Skills / Targets |
| `r` | 重新扫描 | 全局 |
| `t` | 切换 Catppuccin 配色 | 全局 |
| `?` | 帮助 | 全局 |
| `q` / `ctrl+c` | 退出 | 全局 |
| `space` | 启用 / 禁用当前 target 的链接 | Skills |
| `h` `l` | 切换焦点 target | Skills |
| `/` | 过滤（enter 确认，esc 清空） | Skills |
| `o` | 用 `$VISUAL` / `$EDITOR` 打开（macOS 无则 Finder 打开） | Skills |
| `e` | 编辑 upstream 元数据（留空即清除） | Skills |
| `u` | 更新全部：hub pull + 所有 upstream fast-forward | Skills |
| `space` | 禁用该 hub 直链 | Targets |
| `a` | 收编外部实体目录（逐个确认） | Targets |
| `x` | 删除外部链接 / 断链（逐个确认，**只删链接**） | Targets |
| `u` / `p` | hub pull / push | Hub |
| `c` | 全量暂存并提交（输入 message） | Hub |
| `f` | 过时检查：逐个 fetch，显示 ahead/behind | Hub |
| `g` | 输入 GitHub/Git URL，clone 并导入 skill | Hub |

所有网络操作异步执行，失败内联提示，不会阻塞或崩溃面板；离线时浏览、切换、收编照常可用。

更新前后的差异会保持未提交状态，由你审阅后再 `c` 提交 —— 更新只负责把上游内容搬到位。

## 配色

Catppuccin。默认按终端背景自动选择（暗底 Mocha，亮底 Latte），可用环境变量固定，也可在面板里按 `t` 循环：

```bash
SKILLSEND_FLAVOR=macchiato skillsend   # latte | frappe | macchiato | mocha
```

背景保持透明，不覆盖你自己的终端底色。

## 数据落在哪

| 文件 | 位置 |
| --- | --- |
| 配置 | `~/.config/skillsend/config.toml` |
| upstream 元数据 | `<hub>/skillsend.toml` |
| 上游 bare clone 缓存 | `<用户缓存目录>/skillsend/upstreams`（macOS：`~/Library/Caches/skillsend/upstreams`） |

`skillsend.toml` 每条记录含 `source` / `ref` / `path` / `synced` / `tree`：`synced` 是上游 commit sha（用于统计 behind），`tree` 是本地目录内容签名（忽略 `.git` 与 `.DS_Store`），比对二者即可判断本地是否有改动 —— 不需要上游对象在本地可见。

## 安全边界

1. Skill 实体只存在于 hub，绝不复制进 target。
2. 只增删 symlink；真实目录除「收编」外永不创建、删除、移动或修改，且收编必须逐个确认。
3. 绝不主动迁移、重写或「修复」既有链接（包括既有的链式链接），一切变更由你显式触发。
4. 更新只做 fast-forward：本地有改动的第三方 skill 跳过并提示，绝不 merge、绝不丢弃改动。
5. 上游内容更新采用「物化到暂存目录 → 原子交换」，失败即回滚，绝不先删后建。
6. Git clone 先落到 hub 内的暂存目录；只导入根目录或直接子目录含 `SKILL.md` 的仓库，导入时剥离内层 `.git` 并记录 upstream。

## 开发

```bash
go build ./...
go test ./...
```

```
main.go       启动、首次运行引导、targets 常量
config.go     ~/.config/skillsend/config.toml 读写
core/         状态模型与全部变更操作（唯一测试接缝）
  scan.go     扫描 hub 与 target，产出 State
  links.go    Enable / Disable / RemoveLink
  adopt.go    收编外部实体目录
  git.go      pull / push / commit / 上游 fetch 与更新
  manifest.go skillsend.toml 读写
  tree.go     目录内容签名
tui/          Bubble Tea 界面（core 之上的薄壳）
```

`core` 与 UI 层解耦：所有状态变更都在 `core`，`tui` 只负责渲染与按键。

### 发版

产物由 [GoReleaser](https://goreleaser.com) 构建，推 tag 即触发（`.github/workflows/release.yml`）：

```bash
git tag v0.1.0
git push origin v0.1.0
```

会产出 macOS / Linux × amd64 / arm64 四个静态二进制、tar.gz 包与 `checksums.txt`，changelog 从两次 tag 之间的 commit 生成。

本地试跑整条流水线而不上传：

```bash
goreleaser check                                        # 校验配置
goreleaser release --snapshot --clean --skip=publish    # 产物落在 dist/
```

贡献前请读 [`CONTEXT.md`](CONTEXT.md)（术语与不变量）和 [`AGENTS.md`](AGENTS.md)（issue 追踪约定）。设计决策与取舍见 [`docs/adr/`](docs/adr/)。

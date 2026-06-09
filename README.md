# AI Novel Agent · v2.x

> Reasonix 底座 × 56 垂直 Skill × 本地 RAG 知识库 — 终端里的 AI 网文创作助手
>
> IDE 终端输入 `novel-agent` → 进入对话 → 写大纲、埋伏笔、续正文、查知识库

---

## 目录

- [是什么](#是什么)
- [安装](#安装)
- [快速开始](#快速开始)
- [架构](#架构)
- [Skill 系统](#skill-系统)
- [创作流程](#创作流程)
- [RAG 知识库](#rag-知识库)
- [/rag 命令](#rag-命令)
- [小说项目结构](#小说项目结构)
- [配置参考](#配置参考)
- [开发指南](#开发指南)
- [版本分支](#版本分支)

---

## 是什么

**AI Novel Agent** = [Reasonix](https://github.com/esengine/reasonix) 编程 Agent 底座 + 网文创作专用 Skill 系统 + 本地 RAG 参考知识库。

一个 Go 编译的单一二进制文件，不依赖外部数据库、不绑定特定平台、不需要 Docker。所有数据存储在你本地的 `.txt`/`.md` 文件中。

| 对比维度 | 传统 AI 写作工具 | AI Novel Agent |
|---------|----------------|---------------|
| 架构 | Web 服务 + 云端存储 | 本地单二进制 + 文件系统 |
| Skill | 单一通用提示词 | **56 个垂直 Skill**（6 类型 × 9 子技能 + 项目 + 续写） |
| 流程 | 自由对话 | **4 阶段流水线**（定型→大纲+钩子→正文→优化） |
| 状态 | 无记忆 | **伏笔台账 + 大纲版本 + 人物谱系** 持久化 |
| RAG | 无 | **本地 ragCore 目录检索** — 热门小说章节即查即用 |
| 模型 | 绑定单一模型 | **多提供者自由配置**（DeepSeek/MiMo/Anthropic/任意 OpenAI 兼容） |
| 分发 | 网页访问 | `npm install -g novel-agent-cli` |
| 工具 | 纯文本对话 | **完整 Agent 工具链**：文件读写、shell、搜索、ask 决策 |

---

## 安装

### 方式一：npm 全局安装（推荐）

```bash
npm install -g novel-agent-cli
```

```bash
novel-agent version
```

### 方式二：Go 源码编译

```bash
git clone https://gitee.com/penney-101/ai-novel-matrix-studio.git
cd ai-novel-matrix-studio
go build -o novel-agent.exe ./cmd/novel-agent/
```

### 方式三：手动下载二进制

在 [Releases](https://github.com/PeneyLove/ai-novel-matrix-studio/releases) 页面下载对应平台的二进制文件，放入 `$PATH`。

---

## 快速开始

```bash
# 1. 进入你想创作的小说目录
mkdir 我的修仙小说 && cd 我的修仙小说

# 2. 确保 API Key 已设置（以 DeepSeek 为例）
export DEEPSEEK_API_KEY=sk-xxx

# 3. 启动创作终端
novel-agent
```

进入 TUI 后：

```
> /novel-init                           # 初始化小说项目结构（创建目录 + 模板）
> /xuanhuan-genre_init                  # 定型：确认玄幻修仙赛道
> /xuanhuan-outline                     # 生成完整大纲
> /xuanhuan-hooks                       # 预埋伏笔台账
> /novel-continue                       # 续写下一章（自动读进度/大纲/伏笔→写入 chapters/）
> /xuanhuan-optimize-shuangdian         # 专项优化：强化爽点
```

所有 Skill 都可通过 Tab 补全，输入 `/xuanhuan` 然后按 Tab 即可看到该类型下的 9 个子技能。

---

## 架构

```
┌─────────────────────────────────────┐
│           novel-agent.exe            │  ← 单二进制
├─────────────────────────────────────┤
│           Agent 底座                │  ← agent 循环 / TUI / provider / tools
│  ├─ Bubble Tea TUI                  │     (reasonix 同款终端 UI)
│  ├─ config-driven (reasonix.toml)   │     提供者/工具/技能全部配置化
│  ├─ 20+ 内置工具                    │     write_file / read_file / bash / grep / glob / ask ...
│  └─ Skill 系统                      │     .reasonix/skills/novel/*.md
├─────────────────────────────────────┤
│  Novel 专用                         │
│  ├─ 56 个网文 Skill (Markdown)     │     6 类型 × 9 子技能 + init + continue
│  ├─ rag_search 工具                 │     远程 API / 本地 ragCore 目录双模式
│  └─ /rag 命令                       │     终端配置 RAG 知识库
└─────────────────────────────────────┘
```

---

## Skill 系统

### 56 个 Skill（6 类型 × 9 子技能 + 2）

Skills 定义在 `.reasonix/skills/novel/` 下，使用 Reasonix 标准的 Markdown + frontmatter 格式。

```
.reasonix/skills/novel/
├── init.md                  # 初始化小说项目结构
├── continue.md              # 续写下一章（自动读取进度/大纲/伏笔）
├── xuanhuan/                # 玄幻修仙
│   ├── genre_init.md        #   类型定型 & 初始化
│   ├── outline.md           #   大纲全链路搭建 & 迭代
│   ├── hooks.md             #   伏笔/爽点/钩子全维度埋置
│   ├── writing.md           #   正文定向续写（大纲绑定版）
│   ├── optimize-shuangdian.md  # 爽点强化
│   ├── optimize-fubi.md        # 伏笔回收
│   ├── optimize-jiezou.md      # 节奏优化
│   ├── optimize-renshe.md      # 人设优化
│   └── optimize-chongtu.md     # 冲突升级
├── dushi/  (都市) ...
├── guyan/  (古言) ...
├── xuanyi/ (悬疑) ...
├── kehuan/ (科幻) ...
└── tianchong/ (甜宠) ...
```

### 6 大网文类型

| 类型 | 代码 | 细分赛道 |
|------|------|---------|
| 玄幻修仙 | `xuanhuan` | 凡人流/逆袭流/宗门流/仙魔大战/重生修仙/系统修仙 |
| 都市网文 | `dushi` | 战神/神医/系统/赘婿/异能/职场逆袭/校园爽文 |
| 古言权谋 | `guyan` | 宫斗/宅斗/权谋朝堂/重生古言/穿越古言/王爷王妃 |
| 悬疑灵异 | `xuanyi` | 灵异探险/规则怪谈/刑侦悬疑/校园诡异/民间怪谈/无限悬疑 |
| 科幻无限 | `kehuan` | 无限流/末世生存/星际科幻/系统副本/赛博朋克 |
| 现言甜宠 | `tianchong` | 校园甜宠/都市言情/破镜重圆/先婚后爱/霸总甜宠/暗恋逆袭 |

### Skill 格式

```markdown
---
name: xuanhuan-genre_init
description: 玄幻修仙 — 类型定型&初始化（凡人流、逆袭流、宗门流…）
runAs: inline
---

你是专业垂直网文写作智能Agent，当前调用Skill：玄幻修仙-类型定型&初始化Skill。
你已锁定【玄幻修仙】赛道...确认用户写作细分赛道/核心套路/受众偏好...
```

直接编辑 `.md` 文件即可修改提示词，下次调用立即生效。

---

## 创作流程

```
Phase 1: genre_init       ← /{type}-genre_init
    ↓                       锁定赛道、初始化创作档案
Phase 2: outline + hooks  ← /{type}-outline  →  /{type}-hooks
    ↓                       大纲搭建 + 伏笔台账预埋（用户定稿前不启动正文）
Phase 3: writing          ← /novel-continue
    ↓                       每次自动读取：进度 → 大纲 → 伏笔台账 → 上一章 → 写入新章节
Phase 4: optimize_*       ← /{type}-optimize-{爽点/伏笔/节奏/人设/冲突}
                            随时触发专项优化
```

**每章自动植入**：1 个微爽点 + 1 个收尾钩子 + 1 处伏笔铺垫。伏笔台账自动更新。

---

## RAG 知识库

Agent 通过 `rag_search` 工具查询参考知识库，获取热门小说的写法、桥段、套路、节奏参考。

### 双模式

| 模式 | 配置方式 | 说明 |
|------|---------|------|
| **local** | `/rag init <path>` | 从本地 ragCore 目录读取 .txt 章节文件，关键词密度评分检索 |
| **remote** | `/rag remote <url>` | 调用远程向量数据库 API 做语义检索 |

### ragCore 标准目录结构

```
ragCore/
  xuanhuan/                   ← 类别：玄幻修仙
    1_斗破苍穹/                ← 排名_书名
      第一卷/                   ← 卷（可选）
        chapter1.txt           ← 第1章
        chapter2.txt           ← 第2章
        ...
      第二卷/
        chapter1.txt
        ...
    2_完美世界/
      chapter1.txt             ← 无卷时章节目录直接在书名下
      chapter2.txt
      ...
  dushi/                      ← 类别：都市网文
    1_都市之最强狂兵/
      ...
```

章节文件格式：纯文本 `.txt`，UTF-8 编码。支持 `chapter{N}.txt`、`第N章.txt`、`chapter N.txt` 等多种命名。

U 盘/外部硬盘上的 ragCore 目录同样支持，直接 `/rag init E:\ragCore\` 即可。

---

## /rag 命令

在 TUI 中用 `/rag` 管理知识库，无需编辑配置文件：

```
> /rag                        # 查看当前 RAG 状态和统计
RAG knowledge base
  status:  enabled
  mode:    local
  path:    D:\novels\ragCore
  3 genres · 12 novels · 48 volumes · 523 chapters

> /rag init D:\novels\ragCore  # 扫描并激活本地 ragCore
> /rag init E:\                # 也支持 U 盘路径

> /rag remote https://api.example.com --key-env RAG_KEY --index novels
                                # 配置远程 RAG 服务

> /rag enable                   # 启用
> /rag disable                  # 禁用
> /rag topk 10                  # 每次返回 10 条结果
> /rag config                   # 查看原始配置
```

Tab 补全支持 `/rag` 的所有子命令（`init`/`remote`/`enable`/`disable`/`topk`/`config`）。

---

## 小说项目结构

使用 `/novel-init` 自动创建标准化目录：

```
我的小说/
├── .novelAgent/              # AI 元数据（状态/大纲/人设/伏笔台账）
│   ├── state.json            # 创作进度（当前类型/阶段/章节号）
│   ├── config.yaml           # 项目级配置
│   ├── outline/              # 大纲版本
│   ├── characters/           # 人物谱系
│   └── hooks/                # 伏笔台账
├── outlines/                 # 人类可读大纲 (.txt)
│   └── main_outline.txt
├── characters/               # 人设 (.txt)
│   ├── protagonist.txt
│   └── supporting_cast.txt
├── chapters/                 # 正文章节
│   ├── 第1章/
│   │   └── chapter.txt       # Markdown 格式正文
│   ├── 第2章/
│   │   └── chapter.txt
│   └── ...
└── README.md                 # 小说简介
```

章节文件使用 UTF-8 + Markdown，方便排版。Agent 通过 `write_file` 直接写入、`read_file` 读取上下文。

---

## 配置参考

配置文件 `reasonix.toml`（项目根目录），完整语法参见 [reasonix.example.toml](./reasonix.example.toml)。

```toml
default_model = "deepseek-flash"

[agent]
max_steps   = 25
temperature = 0.0

[[providers]]
name        = "deepseek-flash"
kind        = "openai"
base_url    = "https://api.deepseek.com"
model       = "deepseek-v4-flash"
api_key_env = "DEEPSEEK_API_KEY"
context_window = 1000000

# RAG 知识库配置
[rag]
enabled     = true
mode        = "local"                  # "local" 或 "remote"
local_path  = "D:\\novels\\ragCore"    # local 模式下的 ragCore 根目录
top_k       = 5

# 远程模式（mode = "remote" 时生效）
# endpoint    = "https://your-rag-api.com/api"
# api_key_env = "RAG_API_KEY"
# index_name  = "novels"
```

**密钥安全**：API Key 全部通过环境变量注入（`${VAR_NAME}` 或 `api_key_env`），不写入 TOML 文件。

---

## 开发指南

### 项目结构

```
cmd/novel-agent/main.go          # 入口：调用 cli.Run() (Reasonix 底座)
internal/
  cli/                            # TUI 聊天界面 (Bubble Tea)
  agent/                          # Agent 循环 (reasoning / compaction / tool dispatch)
  boot/                           # 启动装配 (config → tool registry → controller)
  config/                         # 配置加载 + 编辑方法 (含 RAGConfig)
  tool/builtin/                   # 内置工具 (bash/write_file/read_file/grep/glob/rag_search/...)
  skill/                          # Skill 发现 + 索引 (Markdown 格式)
  provider/                       # 模型提供者 (openai / anthropic)
  plugin/                         # MCP 插件系统
  permission/                     # 权限门控
  sandbox/                        # 沙箱约束
  ... (40 个模块)
.reasonix/
  skills/novel/                   # 56 个网文 Skill (Markdown)
  commands/                       # 自定义斜杠命令模板
npm/                              # npm 发布脚本
```

### 编译

```bash
make build                      # → bin/novel-agent(.exe)
make cross                      # → dist/ (6 平台全量)
```

### 测试

```bash
go test ./...
```

### 添加新 Skill

在 `.reasonix/skills/novel/<genre>/` 下新建 `.md` 文件：

```markdown
---
name: my-custom-skill
description: 我的自定义创作技能
runAs: inline
---

你是一个专业网文写作助手，当前调用：我的自定义技能。
...
```

文件保存后立即可用：`/my-custom-skill` 或 Agent 自动发现并调用。

---

## 版本分支

| 分支 | 版本 | 基底 | 状态 |
|------|------|------|------|
| **v1.x** (当前) | 2.0.0-alpha | Reasonix harness | 活跃开发 |
| [v0.x](https://gitee.com/penney-101/ai-novel-matrix-studio/tree/v0.x/) | 0.9.0 | 自研 Go 引擎 | 冻结 — 仅修 bug |

---

## License

MIT

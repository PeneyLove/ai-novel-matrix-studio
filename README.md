# AI Novel Agent · v2.0

> 多 Agent 会话隔离 × 三级写作缓存 × SOP 全流程 Skill 体系 — 终端里的 AI 网文创作助手
>
> IDE 终端输入 `novel-agent` → 进入对话 → 立项筹备 / 设定生成 / 正文创作 / 质量审核

---

## 目录

- [是什么](#是什么)
- [安装](#安装)
- [快速开始](#快速开始)
- [架构](#架构)
- [多 Agent 系统](#多-agent-系统)
- [缓存引擎](#缓存引擎)
- [Skill 体系](#skill-体系)
- [创作流程（SOP）](#创作流程sop)
- [内置咨询引擎](#内置咨询引擎)
- [RAG 知识库](#rag-知识库)
- [小说项目结构](#小说项目结构)
- [配置参考](#配置参考)
- [开发指南](#开发指南)
- [版本分支](#版本分支)

---

## 是什么

**AI Novel Agent** = 多 Agent 协作底座 + 三级写作缓存 + 内置咨询引擎 + SOP 全流程 Skill 体系。

一个 Go 编译的单一二进制文件，不依赖外部数据库、不绑定特定平台、不需要 Docker。所有数据存本地文件。

| 对比维度 | 传统 AI 写作工具 | AI Novel Agent v2.0 |
|---------|----------------|---------------------|
| 架构 | Web 服务 + 云端存储 | 本地单二进制 + 文件系统 |
| Agent | 单一对话窗口 | **多 Agent 会话隔离**（策划/设定/大纲/质检独立沙箱，零交叉污染） |
| 缓存 | 无 | **三级写作缓存**（全局资产 + 语义片段 + 剧情摘要）自动注入 |
| 质量 | 依赖模型直觉 | **内置确定性咨询引擎**（大纲校验/人设一致性/伏笔回收/AI-slop 检测） |
| Skill | 单一通用提示词 | **SOP 全流程 Skill**（6 阶段 × 核心 + 6 体裁 × v1 兼容） |
| 预热 | 首轮冷启动 | **自动缓存预热**（首轮前最小请求填充 provider 缓存） |
| RAG | 无 | **本地 ragCore 目录检索** — 热门小说章节即查即用 |
| 模型 | 绑定单一模型 | **多提供者自由配置**（DeepSeek/MiMo/Anthropic/任意 OpenAI 兼容） |
| 分发 | 网页访问 | `npm install -g novel-agent-cli` |

---

## 安装

### 方式一：npm 全局安装（推荐）

```bash
npm install -g novel-agent-cli
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
> /novel-init                           # 初始化小说项目结构
> /xuanhuan-genre_init                  # 定型：确认玄幻修仙赛道
> /xuanhuan-outline                     # 生成完整大纲
> /novel-consult outline                # 内置引擎审核大纲完整性
> /xuanhuan-hooks                       # 预埋伏笔台账
> /novel-continue                       # 续写下一章
> /novel-consult full                   # 全维度质量检测
```

所有 Skill 都可通过 Tab 补全。输入 `/xuanhuan` 然后按 Tab 即可看到该类型下的 9 个子技能。

---

## 架构

```
┌──────────────────────────────────────────────────┐
│                  novel-agent.exe                  │
├──────────────────────────────────────────────────┤
│  Agent 底座（ai-reasonix 同款）                    │
│  ├─ Bubble Tea TUI                                │
│  ├─ config-driven (novel-agent.toml)              │
│  ├─ Provider 层 (OpenAI/Anthropic 兼容)           │
│  └─ Tool Registry + MCP Plugin Host               │
├──────────────────────────────────────────────────┤
│  多 Agent 会话隔离层                               │
│  ├─ SandboxManager (单例，生命周期管理)           │
│  ├─ AgentSandbox × N (独立会话 history)           │
│  └─ Orchestrator (任务分发 → 缓存写入 → 零侵入)   │
├──────────────────────────────────────────────────┤
│  三级写作缓存引擎                                   │
│  ├─ L1 AssetCache (全局资产：世界/人设/大纲)       │
│  ├─ L2 FragmentCache (语义片段：trope/genre 索引) │
│  └─ L3 SummaryCache (剧情摘要：滚动窗口 20 章)    │
├──────────────────────────────────────────────────┤
│  内置咨询引擎 (internal/consult/)                  │
│  ├─ OutlineValidator / CharacterAnalyzer          │
│  ├─ PlotStructureAnalyzer / PacingAnalyzer        │
│  ├─ ConsistencyStrategy / LogicStrategy           │
│  ├─ HookStrategy / StyleStrategy                  │
│  └─ MultiSourceEngine (多文件联合分析)             │
├──────────────────────────────────────────────────┤
│  Novel 专用                                        │
│  ├─ SOP Skill 体系 (novel-sop/)                   │
│  ├─ Genre v1 Skills (6 体裁 × 9)                  │
│  ├─ Cross-genre Skills (8 个分析/参考技能)        │
│  └─ novel_consult 工具 (确定性分析)                │
└──────────────────────────────────────────────────┘
```

---

## 多 Agent 系统

每个 Agent 角色运行在独立沙箱中，对话历史完全隔离，零交叉污染。数据共享仅通过缓存引擎。

| 角色 | 职责 | 沙箱策略 | 系统 Prompt 首行 |
|------|------|---------|-----------------|
| `writer` | 正文创作主 Agent | 常驻，保留全量对话 | `【角色锁定】你是网文写作主Agent` |
| `planner` | 赛道分析 / 对标拆解 | 按需创建，完成后可重置 | `【角色锁定】你是专业的网文赛道分析策划师` |
| `world_builder` | 世界观设定（6 维度）| 按需创建 | `【角色锁定】你是专业的网文世界观设定师` |
| `character_designer` | 人设卡设计（7 维度）| 按需创建 | `【角色锁定】你是专业的网文人设设计师` |
| `outliner` | 大纲架构（5 模块）| 按需创建 | `【角色锁定】你是专业的网文大纲架构师` |
| `reviewer` | 质量审核（5 维度）| 每次重置，不积累 | `【角色锁定】你是专业的网文质量审核员` |

**隔离保障三重机制**：
1. **内存隔离** — 每个沙箱独立 `[]provider.Message` 切片，深拷贝返回
2. **角色锁死** — 系统 Prompt 首行强制 `【角色锁定】` 指令
3. **单一数据通道** — 跨 Agent 数据仅通过 `AssetCache` 传递，不通过对话历史

**配置开关**：
```toml
[multi_agent]
enabled              = true   # 关闭后退化为单 Agent 模式
auto_reset_sandbox   = true   # 辅助任务后自动重置沙箱
cache_inject_enabled = true   # 缓存资产自动注入主 Agent
```

---

## 缓存引擎

### 三级缓存体系

| 级别 | 缓存 | 内容 | Token 节省机制 |
|------|------|------|---------------|
| L1 | `AssetCache` | 世界观 / 人设 / 金手指 / 大纲 / 伏笔 | 启动时注入系统 Prompt（cache-stable prefix），每轮 0 额外成本 |
| L2 | `FragmentCache` | 标准化桥段（打脸/升级/退婚等）| 按 `{genre}:{trope_type}` 召回，避免每次从零生成 |
| L3 | `SummaryCache` | 最近 20 章剧情摘要 | 滚动窗口，替代全文塞入上下文 |

### 缓存预热

首轮请求前自动发送 1-token 最小请求到 Provider，让异步缓存提前填充。首个真实请求即可享受部分 cache hit。

```toml
[cache]
warmup = true   # 默认开启
```

### Provider 端缓存优化

- **PrefixShape 诊断** — 每次请求对 system/tools/rewrite 三级哈希比对，精确定位缓存未命中原因
- **CacheAlignment** — 计算前缀与 DeepSeek 64-token 缓存块的对齐度，输出浪费百分比
- **Compact 参数调优** — `softCompactRatio=0.6` / `compactRatio=0.85` / `tailTokens=32K`
- **ReasoningContent 剥离** — 不向 API 回传 reasoning，避免被按 prompt 计费

---

## Skill 体系

### SOP 全流程 Skill（novel-sop/）

```
.novel-agent/skills/novel-sop/
├── sop-workflow.md                 # 七阶段全流程导航
├── sop-preparation/
│   └── benchmark_analysis.md       # 对标作品拆解
├── sop-setting/                    # （复用 genre v1 skills）
├── sop-outline/                    # （复用 genre v1 skills）
├── sop-writing/
│   └── plot_divergence.md          # 卡文推演（3 路径 × 5 维度评分）
├── sop-quality/
│   ├── consistency_check.md        # 人设一致性校验
│   ├── hook_recovery.md            # 伏笔回收校验
│   └── logic_debug.md              # 逻辑 bug 排查
└── sop-operations/                 # 规划中
```

### Genre v1 Skills（novel/）

6 个体裁 × 9 子技能 = 54 个 v1 兼容 Skill，全部保留不动。

```
.novel-agent/skills/novel/
├── init.md / continue.md           # 项目初始化 / 续写
├── xuanhuan/  dushi/  guyan/       # 玄幻 / 都市 / 古言
│   xuanyi/  kehuan/  tianchong/    # 悬疑 / 科幻 / 甜宠
│   ├── genre_init.md               # 类型定型
│   ├── outline.md                  # 大纲搭建
│   ├── hooks.md                    # 伏笔埋设
│   ├── writing.md                  # 正文创作
│   └── optimize-*.md               # 5 个优化子技能
```

### 跨类型 Skill

| Skill | 功能 |
|-------|------|
| `novel-consult` | 内置咨询引擎（7 维度 × 多源分析） |
| `novel-worldbuilding` | 世界观五维构建法 |
| `novel-characters` | 人物谱系管理 |
| `novel-plot-analyze` | 剧情健康度诊断（40 分制） |
| `novel-volume-plan` | 分卷规划 |
| `novel-style-analysis` | 写作风格分析 |
| `novel-trope-reference` | 网文套路/桥段速查 |
| `novel-rag-search` | RAG 知识库查询引导 |

---

## 创作流程（SOP）

```
Phase 1: 前期筹备          ← /sop-benchmark-analysis
    ↓                        赛道分析 / 对标拆解
Phase 2: 核心设定          ← /{type}-genre_init + /novel-worldbuilding + /novel-characters
    ↓                        世界观 / 人设 / 金手指 → 写入 L1 AssetCache
Phase 3: 大纲搭建          ← /{type}-outline + /novel-consult outline
    ↓                        粗纲 → 分卷细纲 → 内置引擎审核 → 写入缓存
Phase 4: 开篇打磨 + 正文   ← /novel-continue（每次自动读取缓存 + 进度）
    ↓                        缓存注入：全局设定 + 前文摘要 + 章细纲
Phase 5: 质量校验          ← /novel-consult full（7 维度联合分析）
    ↓                        人设 / 逻辑 / 伏笔 / 节奏 / 风格一次性诊断
Phase 6: 卡文急救          ← /sop-plot-divergence
    ↓                        3 条推进路径 × 5 维度评分 → 最优方案
Phase 7: 运营复盘          ← （规划中）
```

---

## 内置咨询引擎

`internal/consult/` 包提供了 8 个确定性分析策略，输出结构化评分报告。所有分析在 Go 端完成，不占用 LLM 推理 token。

### 可用策略

| 策略 | 命令 | 检查项 |
|------|------|--------|
| `outline-completeness` | `/novel-consult outline` | 模块完整性 / 关键词覆盖 / 篇幅密度 |
| `character-consistency` | `/novel-consult characters` | 角色名提取 / 章节出现 / OOC 风险 |
| `plot-structure` | `/novel-consult plot` | 核心冲突 / 分卷 / 结局 |
| `pacing-health` | `/novel-consult plot` | 爽点密度 / 高潮分布 |
| `logic-debug` | `/novel-consult plot` | 突破次数异常 / 时间线 / 战力 |
| `hook-recovery` | `/novel-consult hooks` | 回收率 / 逾期 / 缺计划 |
| `style-analysis` | `/novel-consult style` | AI-slop 检测 / 段落长度 |
| `all` | `/novel-consult full` | 全维度联合分析（多源） |

### 输出示例

```
═══ 创作咨询报告 ═══
分析对象：大纲审核
健康评分：75/100
总体评估：总体良好，个别细节可优化

共发现 3 个问题：

### 🔴 必须修复（阻塞项）
**缺少「核心设定」模块**
> 大纲中没有找到「核心设定」模块。
💡 建议：添加核心设定：境界体系、力量体系、世界观地图
可信度：████░ 95%

### 🟡 建议修复（警告）
...
═══ 报告结束 ═══
```

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
    1_斗破苍穹/               ← 排名_书名
      第一卷/
        chapter1.txt          ← 第1章
        ...
  dushi/                      ← 类别：都市网文
    1_都市之最强狂兵/
      ...
```

章节文件格式：纯文本 `.txt`，UTF-8 编码。支持 `chapter{N}.txt`、`第N章.txt` 等多种命名。

---

## 小说项目结构

使用 `/novel-init` 自动创建标准化目录：

```
我的小说/
├── .novelAgent/              # AI 元数据（状态/大纲/人设/伏笔台账）
│   ├── state.json            # 创作进度
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
│   ├── 第1章/chapter.txt
│   ├── 第2章/chapter.txt
│   └── ...
└── README.md                 # 小说简介
```

---

## 配置参考

配置文件 `novel-agent.toml`（项目根目录），完整语法参见 [novel-agent.example.toml](./novel-agent.example.toml)。

```toml
default_model = "deepseek-flash"

[agent]
max_steps   = 25
temperature = 0.0

[cache]
warmup = true

[multi_agent]
enabled = true

[[providers]]
name        = "deepseek-flash"
kind        = "openai"
base_url    = "https://api.deepseek.com"
model       = "deepseek-v4-flash"
api_key_env = "DEEPSEEK_API_KEY"
context_window = 1000000

[rag]
enabled     = true
mode        = "local"
local_path  = "D:\\novels\\ragCore"
top_k       = 5
```

**密钥安全**：API Key 全部通过环境变量注入，不写入 TOML 文件。

---

## 开发指南

### 项目结构

```
cmd/novel-agent/main.go              # 入口
internal/
  agent/
    agent.go                          # Agent 循环
    sandbox/                          # 多 Agent 会话沙箱 (v2.0)
      sandbox.go / manager.go / prompts.go
    orchestrator/                     # 任务调度器 (v2.0)
    cache_optimizer.go                # 缓存对齐诊断 (v2.0)
    cache_warming.go                  # 缓存预热 (v2.0)
    cache_shape.go                    # PrefixShape 诊断
    compact.go                        # 上下文压缩
  cache/novel/                        # 三级写作缓存 (v2.0)
  consult/                            # 内置咨询引擎 (v2.0)
    consult.go / outline.go / analysis.go
    analysis_multi.go / report.go
  boot/                               # 启动装配
  config/                             # 配置（含 MultiAgentConfig / CacheConfig）
  tool/builtin/                       # 内置工具（含 novel_consult）
  skill/                              # Skill 系统
  provider/                           # 模型提供者
  plugin/                             # MCP 插件
  ...
.novel-agent/
  skills/
    novel/                            # 6 体裁 v1 Skills (54 个)
    novel-sop/                        # SOP 全流程 Skills (8 个, v2.0)
    novel-*.md                        # 跨类型 Skills (8 个)
  commands/
npm/                                  # npm 发布脚本
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

---

## 版本分支

| 分支 | 版本 | 内容 | 状态 |
|------|------|------|------|
| **master** | 2.0 | 多 Agent 隔离 + 三级缓存 + 咨询引擎 + SOP Skill | 稳定 |
| v1.2 | 1.2 | 缓存优化 + 咨询引擎 + SOP Skill (无多 Agent) | 冻结 |
| v1.x | 1.1 | 56 Skill + RAG | 冻结 |

---

## License

MIT

# AI Novel Agent · v3.0.1

> 世界知识图谱 × 角色人格Agent × 时间线合成Agent × 三级写作缓存 × 47 个内置 Skill（含改文修复） × 配置驱动质检引擎 — 终端里的 AI 网文创作助手
>
> IDE 终端输入 `novel-agent` → 进入对话 → 图谱初始化 / 角色反应 / 时间线合成 / 正文创作 / 质量审核

---

## 目录

- [是什么](#是什么)
- [安装](#安装)
- [快速开始](#快速开始)
- [架构](#架构)
- [多 Agent 系统](#多-agent-系统)
- [章节质量管理](#章节质量管理)
- [Skill 体系（46 个内置）](#skill-体系46-个内置)
- [配置驱动](#配置驱动)
- [缓存引擎](#缓存引擎)
- [内置咨询引擎](#内置咨询引擎)
- [RAG 知识库](#rag-知识库)
- [小说项目结构](#小说项目结构)
- [配置参考](#配置参考)
- [开发指南](#开发指南)

---

## 是什么

**AI Novel Agent** = 多 Agent 协作底座 + 三级写作缓存 + 47 个内置 Skill（含 review_repair 改文修复） + 配置驱动 CheckAgent 质检引擎。

单个 Go 二进制文件，不依赖外部数据库、不绑定特定平台。所有数据存本地文件。

| 对比维度 | 传统 AI 写作工具 | AI Novel Agent v3.0 |
|---------|----------------|---------------------|
| 架构 | Web 服务 + 云端存储 | 本地单二进制 + 文件系统 |
| Agent | 单一对话窗口 | **双轨 Agent**（功能角色 × 角色人格，两层隔离） |
| 世界状态 | 全靠模型记忆 | **世界知识图谱** — 5 类节点 + 12 种关系边，子图快照控制上下文 |
| 角色一致性 | 同质化严重 | **角色人格Agent × N** — 独立性格约束，互不感知，防脑补塌陷 |
| 节奏控制 | 手动调整 | **时间线合成Agent** — 4 拍节奏自动排序 + 冲突检测解决 |
| 质量 | 依赖模型直觉 | **check_chapter 10 项量化打分**（满分 100，≥90 通过）+ **fix_chapter 自动修复** |
| 缓存 | 无 | **三级写作缓存**（全局资产 + 语义片段 + 剧情摘要）自动注入 |
| Skill | 单一通用提示词 | **46 个内置 Skill + 5 个 V3 工具**（编译进二进制） |
| 长篇一致性 | 全靠模型记忆 | **图谱反馈循环**（每章图谱更新 → 下次只读快照，无需全文历史） |
| RAG | 无 | **本地 ragCore 目录检索** — 热门小说章节即查即用 |
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
> /novel-init                           # 初始化项目
> /xuanhuan-init                        # 定型赛道 + 生成大纲
> /novel-worldbuilding                  # 世界观五维构建
> /write-chapter                        # 逐章写作（自动质检+修复）
> /novel-consult full                   # 全维度质量检测
```

所有 46 个 Skill 都可通过 Tab 补全。输入 `/xuanhuan` 然后按 Tab 即可看到该类型的 3 个子技能。

---

## 架构

```
┌──────────────────────────────────────────────────┐
│                  novel-agent.exe (v3.0)           │
├──────────────────────────────────────────────────┤
│        ╔══ V3.0 时间线生成引擎 ═══════╗           │
│        ║  世界知识图谱 (Story Bible)  ║           │
│        ║  角色人格 Agent × N         ║           │
│        ║  时间线合成 Agent            ║           │
│        ╚════════════════════════════╝           │
├──────────────────────────────────────────────────┤
│  Agent 底座（Bubble Tea TUI + Provider + MCP）    │
├──────────────────────────────────────────────────┤
│  多 Agent 会话隔离层 (功能角色)                    │
│  ├─ SandboxManager + AgentSandbox × N            │
│  └─ Orchestrator (任务分发 → 缓存写入)            │
├──────────────────────────────────────────────────┤
│  三级写作缓存引擎                                   │
│  ├─ L1 AssetCache (全局资产)                      │
│  ├─ L2 FragmentCache (语义片段)                   │
│  └─ L3 SummaryCache (剧情摘要滚动窗口)             │
├──────────────────────────────────────────────────┤
│  47 个内置 Skill + 5 个 V3 工具 + review_repair     │
│  ├─ V3: storybible_init / snapshot / apply       │
│  ├─ V3: character_react / timeline_synthesize    │
│  ├─ 6 赛道 × 3 (init/writing/optimize) = 18      │
│  ├─ 小说核心 12 (worldbuilding/…/rewrite-fix)      │
│  ├─ SOP 工作流 11 (char-all/sop-vol1/write-…)     │
│  └─ 基础工具 6 (explore/research/review/…)        │
├──────────────────────────────────────────────────┤
│  配置驱动质检引擎                                   │
│  ├─ check_chapter — 10 项量化打分（满分 100）      │
│  ├─ fix_chapter — 机械性问题自动修复               │
│  ├─ review_repair — 8 维改文诊断 + 针对性改写      │
│  ├─ batch_scan — 批量扫描 + 严重度排序             │
│  └─ novel_consult — 多源创作咨询                   │
└──────────────────────────────────────────────────┘
```

---

## 多 Agent 系统

每个 Agent 角色运行在独立沙箱中，对话历史完全隔离，数据仅通过缓存共享。

| 角色 | 职责 | 沙箱策略 |
|------|------|---------|
| `writer` | 正文创作主 Agent | 常驻，保留全量对话 |
| `planner` | 赛道分析 / 对标拆解 | 按需创建 |
| `world_builder` | 世界观设定 | 按需创建 |
| `character_designer` | 人设卡设计 | 按需创建 |
| `outliner` | 大纲架构 | 按需创建 |
| `reviewer` | 质量审核 | 每次重置 |

配置开关：
```toml
[multi_agent]
enabled = true
```

---

## V3.0 时间线生成引擎

v3.0 在 v2.0 的功能角色 Agent 之上，新增**叙事角色 Agent 系统**：

### 世界知识图谱（Story Bible Graph）

持久化世界状态的结构化存储，替代 markdown 文件。

| 节点类型 | 关键属性 |
|---------|---------|
| 角色 | 身份、势力归属、当前实力/状态、当前目标、性格标签、底线 |
| 势力/组织 | 势力范围、与其他势力的关系（敌对/从属/联盟/中立） |
| 地点 | 归属势力、当前状态（安全/沦陷/争夺中） |
| 物品/功法 | 当前持有者、来源、稀有度/价值 |
| 事件 | 参与角色、结果、影响了哪些关系边 |

- **12 种动态关系边**：盟友/敌对/从属/持有/师徒/血缘/恋人/竞争对手/背叛者等
- **子图快照**：每次生成只拉取相关局部子图（种子角色 + 一层关系），控制上下文长度
- **更新指令可审计**：每条变更都有原因+章节号，可回放/回溯

### 角色人格 Agent × N

每个主要角色 = 一个独立 Agent。四个输入驱动：

1. 图谱中该角色的当前状态快照
2. 性格约束模板（说话风格、行为准则、绝不会做的事）
3. 压缩后的最近记忆摘要（近 1-3 章关键事件）
4. 本次触发事件的描述

输出：**内心反应/判断** + **行动倾向（多个候选，优先级排序）** + **关系变化**

核心原则：角色 Agent 之间**互不感知彼此的完整输出**，只感知图谱中已确认的客观事实。

### 时间线合成 Agent

1. **收集**各相关角色 Agent 的反应候选
2. **排序整合**：按 4 拍网文节奏 — 铺垫 → 冲突升级 → 高潮/爽点 → 悬念收尾
3. **冲突协调**：存在冲突时优先满足主角剧情线，但不破坏配角的性格底线
4. **输出**：场景节拍 + **图谱更新指令**（结构化变更列表）

### 反馈循环

每章完成 → 图谱更新指令写回 Story Bible Graph → 下次生成只需最新快照 + 最近几章摘要 → 长篇一致性 O(1) 成本

配置开关：
```toml
[story_bible]
enabled = true

[character_agent]
enabled = true

[timeline]
enabled = true
```

---

## 章节质量管理

每章写完后自动执行 **修复 → 打分 → 重生成** 闭环，不通过门槛绝不持久化。

### 质量工具

| 工具 | 作用 | 输出 |
|------|------|------|
| `check_chapter` | 10 项量化打分（四段完整/比例合规/重复动作/重复结尾/钩子/段落/对话/标点/句子长度/伏笔） | 满分 100，≥90 PASS |
| `fix_chapter` | 自动修复（段间空行/问句句号/标牌删除/AI套话/重复动作替换/四段标记/结尾检测） | 修复文本 + 变更明细 |
| `batch_scan` | 多章并行审查，按严重度排序 | 汇总表 + 修复建议 |

### 单章流水线（write-chapter Skill 自动执行）

```
写正文
  → fix_chapter (自动修复机械问题)
  → check_chapter (10 项打分)
  → 得分 < 90 → 针对性修复 → 回到 fix_chapter
  → 2 轮仍不通过 → 停止，等待用户指示
  → 通过 → 持久化
```

### 长篇一致性：上下文注入

百万字级别长篇的关键技术：**不加载全文，只注入结构化摘要**。

每章写作时自动注入：
- 人物状态清单（当前状态/能力/位置）
- 物品清单（携带物/状态）
- 时间线（第几天/期限剩余）
- 禁忌清单（禁止重复的动作/结尾类型）

通过 `anchor-sync` Skill 每章更新，替代全文上下文加载。

---

## Skill 体系（46 个内置）

所有 Skill 编译进二进制，npm 用户直接可用。项目中的同名 Markdown 文件可覆盖内置版本。

### 赛道专项（6 赛道 × 3）

| 赛道 | Skill |
|------|-------|
| 玄幻修仙 `xuanhuan` | init（定型+大纲）/ writing（续写+伏笔）/ optimize（人设/节奏/爽点/冲突/伏笔） |
| 都市网文 `dushi` | init / writing / optimize |
| 古言权谋 `guyan` | init / writing / optimize |
| 科幻无限 `kehuan` | init / writing / optimize |
| 现言甜宠 `tianchong` | init / writing / optimize |
| 悬疑灵异 `xuanyi` | init / writing / optimize |

### 小说核心（11 个）

`global-encoding` `novel-init` `novel-worldbuilding` `novel-characters` `novel-consult` `novel-style-analysis` `novel-plot-analyze` `novel-trope-reference` `novel-volume-plan` `novel-continue` `novel-rag-search`

### SOP 工作流（11 个）

`char-all` `sop-vol1` `write-chapter` `review-chapter` `anchor-sync` `sop-workflow` `sop-benchmark-analysis` `sop-consistency-check` `sop-hook-recovery` `sop-logic-debug` `sop-plot-divergence`

### 基础工具（6 个）

`explore` `research` `review` `security-review` `test` `init`

---

## 配置驱动

所有质检工具通过项目配置文件驱动，不硬编码任何具体小说的设定。

### 项目配置文件

`.novel-agent/novel-config.json` — 定义本小说的规则：

```json
{
  "pass_score": 90,
  "quality_check_list": [
    {"item": "段落不过长", "score": 10},
    {"item": "对话分段正确", "score": 10},
    ...
  ],
  "repeat_control": {
    "high_frequency_forbid_list": ["打坐行气", "拔匕首插回", "摸柴刀"],
    "replace_map": {
      "打坐行气": "气息沉下",
      "摸刀": "指节微紧"
    }
  },
  "ending_hook_rules": {
    "cycle_type": ["声音钩子", "动作钩子", "视线钩子", "压迫钩子"],
    "forbid_same_type_two_chapters_in_row": true
  }
}
```

工作流：`read_file .novel-agent/novel-config.json` → 传入工具 `config_json` 参数 → 工具按规则运行。

---

## 缓存引擎

### 三级缓存体系

| 级别 | 缓存 | 内容 | 效果 |
|------|------|------|------|
| L1 | `AssetCache` | 世界观/人设/大纲/伏笔 | 系统 Prompt 注入，每轮 0 额外 token |
| L2 | `FragmentCache` | 标准化桥段 | 按 `{genre}:{trope_type}` 召回 |
| L3 | `SummaryCache` | 最近 20 章剧情摘要 | 滚动窗口，替代全文上下文 |

首轮自动缓存预热，首个请求即享 cache hit。

---

## 内置咨询引擎

`internal/consult/` 包提供 8 个确定性分析策略（Go 端完成，不占 LLM token）。

| 策略 | 命令 | 检查项 |
|------|------|--------|
| `outline-completeness` | `/novel-consult outline` | 模块完整性/关键词覆盖 |
| `character-consistency` | `/novel-consult characters` | OOC 风险/人设一致性 |
| `plot-structure` | `/novel-consult plot` | 核心冲突/分卷/结局 |
| `pacing-health` | `/novel-consult plot` | 爽点密度/高潮分布 |
| `logic-debug` | `/novel-consult plot` | 时间线/战力/设定冲突 |
| `hook-recovery` | `/novel-consult hooks` | 回收率/逾期/悬空 |
| `style-analysis` | `/novel-consult style` | AI-slop/段落长度 |
| `all` | `/novel-consult full` | 全维度联合分析 |

---

## RAG 知识库

双模式：`/rag init <path>` 本地 ragCore 目录，`/rag remote <url>` 远程向量库。

```
ragCore/
  xuanhuan/1_斗破苍穹/第一卷/chapter1.txt
  dushi/1_都市之最强狂兵/chapter1.txt
  ...
```

---

## 小说项目结构

```
我的小说/
├── .novel-agent/
│   ├── state.json              # 创作进度
│   ├── novel-config.json       # 项目配置（质检规则/禁止动作/结尾轮循）
│   ├── hooks/ledger.yaml       # 伏笔台账
│   └── skills/                 # 用户自定义 Skill（覆盖内置）
├── memory/
│   ├── anchor-state.md         # 上下文注入摘要（人物/物品/时间线/禁忌）
│   ├── world-building.md
│   └── protagonist.md
├── outlines/main_outline.txt
├── characters/
├── chapters/第N章/chapter.txt
└── README.md
```

---

## 配置参考

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
```

完整语法见 [novel-agent.example.toml](./novel-agent.example.toml)。

---

## 开发指南

### 关键目录

```
internal/
  skill/
    builtins.go                     # 基础 6 个内置 Skill
    sop_builtins.go                 # SOP 工作流 11 个内置 Skill
    novel_builtins.go               # 小说核心 11 个内置 Skill
    genre_builtins.go               # 6 赛道 × 3 = 18 个内置 Skill
  tool/builtin/
    check_chapter.go                # 10 项量化打分
    fix_chapter.go                  # 自动修复
    batch_scan.go                   # 批量扫描
    review_chapter.go               # 旧版审查（兼容）
    novelconsult.go                 # 创作咨询引擎
```

### 编译

```bash
make build                      # → bin/novel-agent(.exe)
make cross                      # → dist/ (6 平台)
go test ./...
```

---

## 版本

| 分支 | 版本 | 内容 |
|------|------|------|
| **master** | 3.0.1 | 世界知识图谱 + 角色人格Agent + 时间线合成Agent + 47 Skill + review_repair + 质检引擎 |
| v1.x | 1.2 | 56 Skill + RAG + 咨询引擎 |

---

## License

MIT
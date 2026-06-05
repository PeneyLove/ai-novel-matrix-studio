# AI Novel Agent · v2.x

> 单二进制 Harness + 可插拔 YAML Skill — 聚焦商业网文全流程 AI 辅助创作。
>
> `npm install -g novelAgent` → `novel-agent` → 终端对话开始创作

---

## 目录

- [是什么](#是什么)
- [安装](#安装)
- [快速开始](#快速开始)
- [核心理念](#核心理念)
- [.novelAgent/ 目录](#novelagent-目录)
- [Skill 系统](#skill-系统)
- [创作流程（4 阶段流水线）](#创作流程4-阶段流水线)
- [模型路由](#模型路由)
- [全局规则](#全局规则)
- [联网权限](#联网权限)
- [提示词迭代](#提示词迭代)
- [CLI 命令参考](#cli-命令参考)
- [配置参考](#配置参考)
- [安全模型](#安全模型)
- [属性测试](#属性测试)
- [开发指南](#开发指南)
- [版本分支](#版本分支)

---

## 是什么

**AI Novel Agent** 是一套服务于网文创作者的本地化 AI 工具链。它不依赖外部数据库、不绑定特定平台、不需要 Docker 编排——一个 Go 编译的二进制文件 + 一个 `.novelAgent/` 本地目录即可完整运转。

核心差异点：

| 对比维度 | 传统 AI 写作工具 | AI Novel Agent |
|---------|----------------|---------------|
| 架构 | Web 服务 + 云端存储 | 本地单二进制 + 文件系统 |
| Skill | 单一通用提示词 | **6 类型 × 9 子技能 = 54 个垂直 Skill** |
| 流程 | 自由对话 | **4 阶段强制流水线**（定型→大纲+钩子→正文→优化） |
| 状态 | 无记忆 | **伏笔台账 + 大纲版本 + 人物谱系** 持久化 |
| 模型 | 绑定单一模型 | **4 个国产大模型按阶段分工** |
| 提示词 | 写死不可变 | **3 种迭代方式**（手动编辑 / AI 自动优化 / 版本快照回滚） |
| 分发 | 网页访问 | `npm install -g novelAgent` |

---

## 安装

### 方式一：npm 全局安装（推荐）

```bash
npm install -g novelAgent
```

安装完成后验证：

```bash
novel-agent version   # 2.0.0-alpha

安装脚本自动检测你的操作系统和 CPU 架构（macOS Intel/Apple Silicon、Linux x64、Windows x64），下载对应的 Go 预编译二进制到全局 `PATH`。

安装完成后验证：

```bash
novel-agent version   # 2.0.0-alpha
novel-agent help
```

### 方式二：Go 源码编译（开发者）

```bash
go build -o novel-agent.exe ./cmd/novel-agent/
```

### 方式三：手动下载二进制

在 [Releases](https://github.com/PeneyLove/ai-novel-matrix-studio/releases) 页面下载对应平台的二进制文件，放入 `$PATH` 即可。

---

## 快速开始

```bash
# 1. 初始化项目
novel-agent init
cd my-novel-project/

# 2. 编辑 API Key（至少配一个模型）
#    打开 .novelAgent/config.yaml，填入你的 API Key
#    或设置环境变量：export DEEPSEEK_API_KEY=sk-xxx

# 3. 查看已安装的 Skill
novel-agent skill list
# 输出：xuanhuan_genre_init, xuanhuan_outline, xuanhuan_hooks, ...
#       共 54 个 Skill（6 类型 × 9 子技能）

# 4. 启动创作流水线（以玄幻修仙为例）
novel-agent pipeline \
  --skill xuanhuan_genre_init \
  --trend-data "凡人流修仙，开局被废灵根，意外获得上古传承"

# 5. 导出为 Word 文档
novel-agent export --task-id task-1733000000 --format txt
```

---

## 核心理念

> AI 是工具，不是「代笔」。人类的创意、审美与版权意识，才是长久运营的核心。

本系统的核心竞争力在于**用 AI 放大人类创意**——以标准化 4 阶段流水线为基础，以 54 个垂直 Skill 为效率支撑，以伏笔台账和版权留存哈希为底线。

```
人类主导环节        AI 辅助环节
─────────────────────────────────────
确认赛道/套路  →   [Skill] 初始化创作档案
审核大纲/定稿  →   [Skill] 生成大纲 + 伏笔埋置
逐章修改去 AI 味 →  [Skill] 定向续写（大纲绑定）
专项优化节奏  →   [Skill] 爽点强化 / 伏笔回收 / 人设优化
```

---

## .novelAgent/ 目录

所有数据存储在本地，无需 MySQL、MongoDB、Redis 等任何外部服务。

```
my-novel-project/
└── .novelAgent/
    ├── config.yaml          # 模型 API Key、全局规则、网络权限
    ├── skills/              # 已安装的 Skill 定义（YAML 格式）
    │   ├── xuanhuan_genre_init.yaml
    │   ├── xuanhuan_outline.yaml
    │   ├── ...
    │   └── tianchong_optimize_chongtu.yaml
    ├── prompts_history/     # 提示词版本快照（每次修改自动保存）
    │   └── xuanhuan_outline/outline_generation/
    │       ├── v001-20250101-120000.yaml
    │       └── v002-20250102-090000.yaml
    ├── state/               # 创作状态持久化（伏笔台账 + 大纲版本 + 人物谱系）
    │   └── task-1733000000.json
    ├── corpus/              # 本地语料缓存（可选，按题材分文件）
    ├── outputs/             # 生成内容（按 task_id 分目录）
    │   └── task-1733000000/
    │       ├── genre_init.txt
    │       ├── outline_generation.txt
    │       ├── hooks_placement.txt
    │       └── content_generation.txt
    └── traces/              # 版权留存哈希记录（JSONL 格式）
        └── task-1733000000.jsonl
```

---

## Skill 系统

### 6 大类型 × 9 子技能 = 54 个垂直 Skill

每个网文类型包含 4 个核心阶段 Skill（必须按顺序执行）和 5 个专项优化 Skill（随时调用）：

| 类型 | 代码 | 细分赛道 | 大纲模型 | 正文模型 |
|------|------|---------|---------|---------|
| 玄幻修仙 | `xuanhuan` | 凡人流/逆袭流/宗门流/仙魔大战/系统修仙 | 豆包 | 通义千问 |
| 都市网文 | `dushi` | 战神/神医/系统/赘婿/异能/职场逆袭 | 豆包 | 通义千问 |
| 古言权谋 | `guyan` | 宫斗/宅斗/权谋朝堂/重生古言/王爷王妃 | 豆包 | 通义千问 |
| 悬疑灵异 | `xuanyi` | 灵异探险/规则怪谈/刑侦悬疑/无限悬疑 | 豆包 | 通义千问 |
| 科幻无限 | `kehuan` | 无限流/末世生存/星际科幻/赛博朋克 | 豆包 | 通义千问 |
| 现言甜宠 | `tianchong` | 校园甜宠/都市言情/破镜重圆/霸总甜宠 | 豆包 | **豆包** |

**9 个子 Skill（每类型）：**

| Phase | 子 Skill | 说明 | 强制顺序 |
|-------|---------|------|---------|
| 1 | `genre_init` | 类型定型 & 初始化 — 锁定赛道、确认细分方向、初始化创作档案 | ✅ 必须先执行 |
| 2 | `outline` | 大纲全链路搭建 & 迭代 — 核心设定/人物谱系/主线剧情/爽点节点 | ✅ genre_init 后 |
| 2 | `hooks` | 伏笔/爽点/钩子全维度埋置 — 长线伏笔/阶段性爽点/章节钩子台账 | ✅ outline 后 |
| 3 | `writing` | 正文定向续写（大纲绑定版）— 每章植入 1 微爽点+1 钩子+1 伏笔 | ✅ outline+hooks 后 |
| 4 | `optimize/shuangdian` | 爽点强化 — 提升打脸/升级/逆袭密度和强度 | 随时 |
| 4 | `optimize/fubi` | 伏笔回收 — 按台账精准回收伏笔、设计反转 | 随时 |
| 4 | `optimize/jiezou` | 节奏优化 — 去水字数、压缩过渡、密集高光节点 | 随时 |
| 4 | `optimize/renshe` | 人设优化 — 迭代人物弧光、防人设崩塌 | 随时 |
| 4 | `optimize/chongtu` | 冲突升级 — 升级主支线矛盾、引入新冲突维度 | 随时 |

### Skill YAML 定义

每个 Skill 是一个独立的 YAML 文件，你可以直接编辑它来调整提示词：

```yaml
# .novelAgent/skills/xuanhuan_writing.yaml
name: xuanhuan_writing
version: "2.0.0"
genre: xuanhuan
type: core
phase: 3
sub_skill: writing
prerequisites: [outline_generation, hooks_placement]
stages: [content_generation]
model_bindings:
  content_generation: qwen
prompts:
  content_generation: |
    你是专业垂直网文写作智能Agent...
    核心规则：严格绑定已定稿大纲...
    每章必须植入：1个微爽点+1个收尾钩子+1处伏笔铺垫...
    当前：第{{.ChapterNo}}章，累计{{.TotalChapters}}章
    伏笔台账：{{.HookSummary}}
    全局规则：{{.GlobalRules}}
output_header: "当前调用Skill：玄幻修仙-正文定向续写Skill"
requires_network: false
```

`{{.Variable}}` 模板变量在运行时由流水线自动填充。

---

## 创作流程（4 阶段流水线）

```
Phase 1: genre_init      ← 用户输入：想写什么类型？（凡人流修仙 / 都市战神 / 宫斗权谋...）
    ↓                      输出：锁定赛道确认 + 创作档案初始化
Phase 2: outline          ← 基于初始设定生成大纲（核心设定+人物+剧情+爽点）
    ↓  hooks               ← 同步埋置伏笔台账（≥8 条长线伏笔 + 章节钩子）
    ↓  用户审核通过 → 定稿（未定稿前绝对不启动正文）
Phase 3: writing           ← 逐章续写，每章读取伏笔台账（{{.HookSummary}}）
    ↓                      每章植入：1 微爽点 + 1 钩子 + 1 伏笔铺垫
Phase 4: optimize_*        ← 随时触发专项优化（爽点/伏笔/节奏/人设/冲突）
```

**强制规则**（Hard-coded in pipeline）：
- Phase 2 大纲未定稿 → Phase 3 写作直接拒绝（返回 prerequisite error）
- Phase 3 每次续写注入前文最后 300 字上下文
- Phase 3 写完一章自动更新伏笔台账（已回收的标记为 `resolved`）
- 每阶段输出必须以 `当前调用Skill：XXX` 开头

---

## 模型路由

每个 Skill 的 YAML 文件声明每个阶段使用哪个模型。运行时如果主模型不可用（未配置 API Key 或调用失败），自动降级到 `config.yaml` 中指定的 `fallback` 模型（默认 Qwen）。

| 阶段 | 默认模型 | 优势 | 参考成本 |
|------|---------|------|---------|
| `genre_init` | **MiniMax** (海螺AI) | 创意生成、热点分析 | ¥0.01/千 tokens |
| `outline_generation` | **豆包** (字节跳动) | 中文理解、逻辑严谨 | ¥0.008/千 tokens |
| `hooks_placement` | **豆包** (字节跳动) | 长文本结构化 | ¥0.008/千 tokens |
| `content_generation` | **通义千问** (阿里) | 长文本生成、网文风格 | ¥0.006/千 tokens |
| `optimize_*` | **DeepSeek** (深度求索) | 语言润色、去 AI 味 | ¥0.001/千 tokens |
| *所有阶段 fallback* | **通义千问** | 万能替补 | — |

**每个模型独立原生适配**，不共用"最低共同分母"：
- MiniMax 使用 `sender_type` 消息格式 + `reply_constraints`
- 豆包支持 `endpoint_id` 直接路由（Volcengine Ark）
- 通义千问双模式：DashScope 原生 + OpenAI 兼容自动切换
- DeepSeek 原生支持 `frequency_penalty` 防重复输出

---

## 全局规则

`novel-agent init` 会在 `config.yaml` 中自动写入 8 条全局规则。**每次调用 AI 模型时，系统自动将这 8 条规则注入到 system prompt 最前面**：

```yaml
# .novelAgent/config.yaml
global_rules:
  language: zh-CN
  rules:
    - "全程使用简体中文输出，包括所有说明、描述、对话、叙述"
    - "专有名词、技术术语可保留原文（如 API、SDK、GDP、CEO 等）"
    - "人名、地名、品牌名等专有名称可保留英文或拼音原文"
    - "网络热梗、流行语、meme 可以使用，但涉及实时信息时需申请联网权限"
    - "代码块、命令行示例保持英文原样"
    - "禁止输出繁体中文"
    - "数字使用阿拉伯数字"
    - "标点符号使用全角中文标点"
  network:
    enabled: false
    ask_permission: true
```

你可以随时编辑 `config.yaml` 增删规则——下次调用立即生效，无需重启。

---

## 联网权限

部分 Skill 可能需要联网获取实时信息（如搜索网络热梗、查询热搜数据）。这些 Skill 在 YAML 中声明 `requires_network: true`。

**默认行为**：网络功能关闭，当执行一个需要联网的 Skill 时，系统会：

- **CLI 模式**：打印提示并等待用户输入 `y/N`
- **API 模式**（`novel-agent serve`）：返回 `HTTP 403`，Flutter GUI 弹出权限对话框

**授权方式**：

```bash
# CLI：在提示时输入 y
# API：通过 Flutter GUI 设置页一键开启

# 或直接编辑 config.yaml
global_rules:
  network:
    enabled: true
    ask_permission: false
```

---

## 提示词迭代

每个 Skill 的提示词不是一成不变的。系统提供 **3 种迭代方式**：

### 1. 手动编辑（`prompt edit`）

```bash
novel-agent prompt edit --skill xuanhuan_outline --stage outline_generation
# 打开系统默认编辑器（Windows: 记事本, macOS/Linux: $EDITOR）
# 保存后自动验证 + 热加载 + 版本快照
```

### 2. AI 自动优化（`prompt optimize`）

```bash
novel-agent prompt optimize \
  --skill xuanhuan_writing \
  --stage content_generation \
  --feedback "节奏太慢，每章前300字要出现一个小爽点，对话要更简洁"
# AI 分析你的反馈，自动改写 prompt 模板并写回
```

### 3. 版本快照（`prompt history` / `prompt diff` / `prompt rollback`）

```bash
# 查看历史版本
novel-agent prompt history --skill xuanhuan_outline --stage outline_generation

# 对比当前版本与上一个快照
novel-agent prompt diff --skill xuanhuan_outline --stage outline_generation

# 回滚到指定版本
novel-agent prompt rollback --skill xuanhuan_outline --stage outline_generation \
  --to v001-20250101-120000.yaml
```

所有快照存储在 `.novelAgent/prompts_history/` 下，按 Skill/Stage 分级。

---

## CLI 命令参考

### 项目管理

| 命令 | 说明 |
|------|------|
| `novel-agent init [--force]` | 初始化 `.novelAgent/` 目录，写入默认配置和 54 个内置 Skill |
| `novel-agent version` | 输出版本号 |

### 创作执行

| 命令 | 说明 |
|------|------|
| `novel-agent run --skill <name> --stage <stage> --input <json>` | 执行单个 Skill 阶段 |
| `novel-agent pipeline --skill <name> --trend-data <text>` | 执行完整 4 阶段流水线 |
| `novel-agent export --task-id <id> --format txt` | 导出生成内容为文件 |

### Skill 管理

| 命令 | 说明 |
|------|------|
| `novel-agent skill list` | 列出已安装的 Skill |
| `novel-agent skill validate <name>` | 校验 Skill YAML 定义 |
| `novel-agent skill remove <name>` | 移除一个 Skill |

### 提示词管理

| 命令 | 说明 |
|------|------|
| `novel-agent prompt edit --skill <name> --stage <stage>` | 编辑 Skill 提示词模板 |
| `novel-agent prompt history --skill <name> --stage <stage>` | 查看提示词版本快照 |
| `novel-agent prompt diff --skill <name> --stage <stage>` | 对比当前版本与上次快照 |
| `novel-agent prompt optimize --skill <name> --stage <stage> --feedback <text>` | AI 自动优化提示词 |
| `novel-agent prompt rollback --skill <name> --stage <stage> --to <file>` | 回滚到指定版本 |

### 服务

| 命令 | 说明 |
|------|------|
| `novel-agent serve [--port 9876]` | 启动本地 HTTP API（供 Flutter GUI 连接） |

---

## 配置参考

```yaml
# .novelAgent/config.yaml

# ── 模型配置 ────────────────────────────────────────────
minimax:
  api_key: "${MINIMAX_API_KEY}"                      # 支持环境变量占位符 ${VAR}
  api_endpoint: "https://api.minimax.chat/v1/text/chatcompletion_v2"
  model_name: "abab6.5s-chat"
  max_tokens: 4096
  temperature: 0.8
  timeout: 60
  retry_times: 3
  group_id: ""                                        # MiniMax 专属字段

doubao:
  api_key: "${DOUBAO_API_KEY}"
  api_endpoint: "https://ark.cn-beijing.volces.com/api/v3/chat/completions"
  model_name: "doubao-pro-32k"
  endpoint_id: ""                                     # Volcengine Ark 端点 ID
  max_tokens: 8192
  temperature: 0.7
  timeout: 90
  retry_times: 3

qwen:
  api_key: "${QWEN_API_KEY}"
  api_endpoint: "https://dashscope.aliyuncs.com/compatible-mode/v1/chat/completions"
  model_name: "qwen-long"
  compatible_mode: true                               # true=OpenAI兼容 / false=DashScope原生
  max_tokens: 6000
  temperature: 0.75
  timeout: 120
  retry_times: 3

deepseek:
  api_key: "${DEEPSEEK_API_KEY}"
  api_endpoint: "https://api.deepseek.com/v1/chat/completions"
  model_name: "deepseek-chat"
  max_tokens: 4096
  temperature: 0.6
  timeout: 60
  retry_times: 3

# ── 阶段路由 ────────────────────────────────────────────
stage_routing:
  genre_init:          minimax
  outline_generation:  doubao
  hooks_placement:     doubao
  content_generation:  qwen
  optimize_shuangdian: deepseek
  optimize_fubi:       deepseek
  optimize_jiezou:     deepseek
  optimize_renshe:     deepseek
  optimize_chongtu:    deepseek
  fallback:            qwen

# ── 全局规则 ────────────────────────────────────────────
global_rules:
  language: zh-CN
  rules:
    - "全程使用简体中文输出..."
    # ... 8 条规则
  network:
    enabled: false
    ask_permission: true
```

**环境变量注入**：配置文件中 `${VAR_NAME}` 会在启动时从环境变量读取实际值。API Key 不在 YAML 中明文存储。

---

## 安全模型

### 密钥保护

```
config.yaml → ${ENV_VAR} 占位符 → 运行时从环境变量注入
                                ↓
                            内存持有，不写入任何日志或输出文件
                                ↓
                    encrypt 子命令：AES-256-GCM + 机器指纹派生密钥
```

```bash
# 加密 config.yaml 中的敏感字段
novel-agent config encrypt
# 解密（同一台机器）
novel-agent config decrypt
```

### 沙箱约束

- 所有文件读写限制在 `.novelAgent/` 目录内
- 路径穿越（`../etc/passwd`）被拒绝
- 模型 API endpoint 仅允许 `config.yaml` 中声明的白名单地址

### 版权留存

每次章节生成自动记录 SHA256 哈希到 `.novelAgent/traces/`：

```json
{"task_id":"task-001","stage":"content_generation","prompt_hash":"abc123...","draft_hash":"def456...","timestamp":"2025-01-01T12:00:00Z"}
```

- `prompt_hash`：提示词 SHA256
- `draft_hash`：AI 初稿 SHA256
- `final_hash`：人工修改定稿 SHA256（待后续更新）

---

## 属性测试

系统使用 8 条正确性属性（Property-Based Testing）验证核心模块：

| 属性 | 说明 | 覆盖包 |
|------|------|--------|
| P1 | Skill Schema 校验一致性 — 合法 YAML 通过 Validate，非法被拒绝 | `skill_test.go` |
| P2 | 模型路由完备性 — 所有 stage 均有 fallback | `model_test.go` |
| P3 | Skill 阶段覆盖 — SupportsStage 正确性 | `orchestrator_test.go` |
| P4 | 流水线幂等性 — TaskExists 防止重复执行 | `orchestrator_test.go` |
| P5 | 全局规则注入 — 8 条规则写入每个 prompt 前缀 | `rules_test.go` |
| P6 | 版权留存完整性 — prompt_hash + draft_hash 非空 | `orchestrator_test.go` |
| P7 | 联网权限阻断 — requires_network + disabled → 返回 PermissionRequest | `rules_test.go` |
| P8 | 加密 roundtrip — Encrypt/Decrypt 对称性 + 跨机器认证拒绝 | `crypto_test.go` |

运行：

```bash
go test ./internal/... -v -cover
```

---

## 开发指南

### 项目结构

```
cmd/novel-agent/          # CLI 入口 (cobra)
internal/
  harness/                # 顶层运行时：skill + router + pipeline 绑定
  skill/                  # Skill Schema + Loader + Manager
  model/                  # 4 个国产模型客户端 + Router + 指数退避重试
  pipeline/               # 流水线编排器（phase 强制 + state 注入）
  state/                  # 创作状态持久化（伏笔台账 + 大纲版本 + 人物谱系）
  storage/                # .novelAgent/ 本地 IO + 沙箱路径约束
  global/                 # 全局规则 + 联网权限
  prompt/                 # 提示词生命周期管理（快照/对比/回滚/AI 优化）
  secure/                 # AES-256-GCM 加密
  api/                    # HTTP API server (127.0.0.1:9876)
skills/                   # 54 个内置 Skill YAML（6 类型 × 9 子技能）
gui/                      # Flutter Web/PWA GUI
npm/                      # npm 包安装脚本
```

### 编译

```bash
# 单平台构建
go build -o novel-agent ./cmd/novel-agent/

# 全平台交叉编译
make build
# 输出：dist/novel-agent_darwin_amd64
#      dist/novel-agent_darwin_arm64
#      dist/novel-agent_linux_amd64
#      dist/novel-agent_windows_amd64.exe
```

### 测试

```bash
go test ./internal/... -v -cover
```

---

## 版本分支

| 分支 | 版本 | 技术栈 | 状态 |
|------|------|-------|------|
| **main** (v2.x) | 2.0.0-alpha | Go Harness + 54 Skills + Flutter GUI | 活跃开发 |
| [v0.x](https://gitee.com/penney-101/ai-novel-matrix-studio/tree/v0.x/) | 0.9.0 | Python/FastAPI/Celery/PyQt6 | 冻结 — 仅修 bug |

---

## License

MIT

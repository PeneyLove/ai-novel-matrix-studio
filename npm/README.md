# AI Novel Agent · v2.0

> 多 Agent 会话隔离 × 三级写作缓存 × SOP 全流程 Skill 体系 — 终端里的 AI 网文创作助手
>
> `npm install -g novel-agent-cli` → `novel-agent` → 立项筹备 / 设定生成 / 正文创作 / 质量审核

---

## 是什么

**AI Novel Agent** = 多 Agent 协作底座 + 三级写作缓存 + 内置咨询引擎 + SOP 全流程 Skill 体系。

一个 Go 编译的单一二进制文件，不依赖外部数据库、不绑定特定平台、不需要 Docker。

| 对比维度 | 传统 AI 写作工具 | AI Novel Agent v2.0 |
|---------|----------------|---------------------|
| 架构 | Web 服务 + 云端存储 | 本地单二进制 + 文件系统 |
| Agent | 单一对话窗口 | **多 Agent 会话隔离**（策划/设定/大纲/质检独立沙箱） |
| 缓存 | 无 | **三级写作缓存**（全局资产 + 语义片段 + 剧情摘要）自动注入 |
| 质量 | 依赖模型直觉 | **内置确定性咨询引擎**（大纲校验/人设/伏笔/AI-slop） |
| 预热 | 首轮冷启动 | **自动缓存预热** |
| Skill | 单一提示词 | **SOP 全流程**（6 阶段）+ 6 体裁 v1 兼容 |
| RAG | 无 | **本地 ragCore 目录检索** |
| 分发 | 网页访问 | `npm install -g novel-agent-cli` |

---

## 安装

```bash
npm install -g novel-agent-cli
novel-agent version
```

---

## 快速开始

```bash
mkdir 我的修仙小说 && cd 我的修仙小说
export DEEPSEEK_API_KEY=sk-xxx
novel-agent
```

进入 TUI 后：

```
> /novel-init                           # 初始化项目结构
> /xuanhuan-genre_init                  # 定型赛道
> /xuanhuan-outline                     # 生成大纲
> /novel-consult outline                # 内置引擎审核纲完整性
> /xuanhuan-hooks                       # 预埋伏笔台账
> /novel-continue                       # 续写正文
> /novel-consult full                   # 全维度质量检测
```

Tab 补全所有 Skill。

---

## 核心能力

### 多 Agent 会话隔离

6 个 Agent 角色运行在独立沙箱中，对话历史完全隔离，数据仅通过缓存共享：

| 角色 | 职责 |
|------|------|
| `writer` | 正文创作主 Agent（常驻） |
| `planner` | 赛道分析 / 对标拆解 |
| `world_builder` | 世界观设定（6 维度） |
| `character_designer` | 人设卡设计（7 维度） |
| `outliner` | 大纲架构（5 模块） |
| `reviewer` | 质量审核（5 维度诊断） |

配置开关：`[multi_agent] enabled = true`，关闭后退化为单 Agent 模式。

### 三级写作缓存

| 级别 | 内容 | 效果 |
|------|------|------|
| L1 AssetCache | 世界观 / 人设 / 金手指 / 大纲 | 启动时注入系统 Prompt，每轮 0 额外 token |
| L2 FragmentCache | 标准化桥段（打脸/升级/退婚等）| 按 `{genre}:{trope_type}` 召回 |
| L3 SummaryCache | 最近 20 章剧情摘要 | 滚动窗口，替代全文塞入上下文 |

首轮自动缓存预热（1-token 最小请求填充 Provider 缓存）。

### 内置咨询引擎

8 个确定性分析策略（Go 端完成，不占 LLM 推理 token）：

```
/novel-consult outline   → 大纲完整性检查
/novel-consult full      → 全维度联合分析（大纲+人设+伏笔+节奏+风格）
```

输出结构化评分报告（100 分制 + 严重级标注 + 置信度进度条）。

### SOP 全流程

```
前期筹备 → 核心设定 → 大纲搭建 → 正文创作 → 质量校验 → 卡文急救 → 运营复盘
```

每阶段关联对应 Skill，通过 `/sop-workflow` 查看导航。

---

## 6 大网文类型

| 类型 | 代码 | 细分赛道 |
|------|------|---------|
| 玄幻修仙 | `xuanhuan` | 凡人流 / 逆袭流 / 宗门流 / 重生修仙 / 系统修仙 |
| 都市网文 | `dushi` | 战神 / 神医 / 赘婿 / 异能 / 校园爽文 |
| 古言权谋 | `guyan` | 宫斗 / 宅斗 / 权谋朝堂 / 重生古言 |
| 悬疑灵异 | `xuanyi` | 规则怪谈 / 刑侦悬疑 / 无限悬疑 |
| 科幻无限 | `kehuan` | 无限流 / 末世 / 星际科幻 / 赛博朋克 |
| 现言甜宠 | `tianchong` | 校园甜宠 / 都市言情 / 破镜重圆 / 先婚后爱 |

---

## RAG 知识库

双模式：`/rag init <path>` 本地 ragCore 目录模式，`/rag remote <url>` 远程向量库模式。

```
ragCore/
  xuanhuan/
    1_斗破苍穹/第一卷/chapter1.txt
    1_斗破苍穹/第一卷/chapter2.txt
    ...
  dushi/
    1_都市之最强狂兵/chapter1.txt
    ...
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

完整语法见 [novel-agent.example.toml](https://gitee.com/penney-101/ai-novel-matrix-studio/blob/master/novel-agent.example.toml)。

---

## 版本

| 分支 | 版本 | 状态 |
|------|------|------|
| **master** | 2.0 | 稳定 |
| v0.x | 0.9.0 | 冻结 |

---

## License

MIT

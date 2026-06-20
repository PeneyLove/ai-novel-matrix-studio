# AI Novel Agent · v2.0

> 多 Agent 会话隔离 × 46 个内置 Skill × 配置驱动质检引擎 — 终端里的 AI 网文创作助手
>
> `npm install -g novel-agent-cli` → `novel-agent` → 立项 / 设定 / 创作 / 质检

---

## 是什么

**AI Novel Agent** = 单二进制文件。所有 46 个 Skill 和质检工具编译进二进制，npm 安装即用。

| 能力 | 说明 |
|------|------|
| 多 Agent | 6 角色独立沙箱（策划/设定/大纲/创作/质检），零交叉污染 |
| 章节质检 | `check_chapter` 10 项量化打分（满分 100，≥90 通过）+ `fix_chapter` 自动修复 |
| 长篇一致性 | 配置驱动上下文注入**（每章只注摘要，百万字不爆上下文） |
| 三级缓存 | 全局资产 + 语义片段 + 剧情摘要，自动注入系统 Prompt |
| 内置咨询 | 8 个确定性分析策略（Go 端完成，不占 LLM token） |
| RAG | 本地 ragCore 目录检索热门小说桥段 |

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

```
> /novel-init                    # 初始化项目
> /xuanhuan-init                 # 定型赛道 + 生成大纲
> /write-chapter                 # 逐章写作（自动质检+修复）
```

---

## 内置 Skill（46 个）

| 分类 | 数量 | 说明 |
|------|------|------|
| 赛道专项 | 18 | 玄幻/都市/古言/科幻/甜宠/悬疑 × init/writing/optimize |
| 小说核心 | 11 | 世界观/人设/咨询/风格/剧情/套路/分卷/续写/RAG |
| SOP 工作流 | 11 | 写作引擎/质检引擎/规则引擎/锚点同步/伏笔回收/卡文推演 |
| 基础工具 | 6 | explore/research/review/security-review/test/init |

全部 Tab 补全。项目中的同名 Markdown 文件可覆盖内置版本。

---

## 章节质检流水线

每章自动执行，不通过门槛不持久化：

```
写正文 → fix_chapter (自动修复) → check_chapter (10项打分)
  → 得分 < 90 → 修复 → 重检 → 2轮不通过则停止
  → 通过 → 持久化
```

---

## 6 大网文类型

| 类型 | 赛道 |
|------|------|
| 玄幻修仙 `xuanhuan` | 凡人流/逆袭流/宗门流/重生/系统 |
| 都市网文 `dushi` | 战神/神医/赘婿/异能/校园 |
| 古言权谋 `guyan` | 宫斗/宅斗/权谋/重生古言 |
| 悬疑灵异 `xuanyi` | 规则怪谈/刑侦/无限悬疑 |
| 科幻无限 `kehuan` | 无限流/末世/星际/赛博朋克 |
| 现言甜宠 `tianchong` | 校园/都市/破镜重圆/先婚后爱 |

---

## 配置驱动

项目级配置 `.novel-agent/novel-config.json` 定义质检规则，工具通过 `config_json` 参数读取：

```json
{
  "pass_score": 90,
  "repeat_control": {
    "high_frequency_forbid_list": ["打坐行气", "拔匕首插回"],
    "replace_map": { "打坐行气": "气息沉下" }
  },
  "ending_hook_rules": {
    "cycle_type": ["声音钩子", "动作钩子", "视线钩子", "压迫钩子"]
  }
}
```

---

## RAG 知识库

```bash
/rag init ragCore
```

```
ragCore/xuanhuan/1_斗破苍穹/第一卷/chapter1.txt
ragCore/dushi/1_都市之最强狂兵/chapter1.txt
```

---

## 配置参考

```toml
[[providers]]
name        = "deepseek-flash"
kind        = "openai"
base_url    = "https://api.deepseek.com"
model       = "deepseek-v4-flash"
api_key_env = "DEEPSEEK_API_KEY"
```

---

## License

MIT

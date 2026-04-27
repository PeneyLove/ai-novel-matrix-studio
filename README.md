# AI 小说矩阵工作室

> 以"人类创意主导 + AI 高效执行"为核心，搭建可复制、高产出、低风险的 AI 小说创作矩阵。

---

## 目录

- [项目简介](#项目简介)
- [核心目标](#核心目标)
- [系统架构](#系统架构)
- [技术栈](#技术栈)
- [Windows 桌面客户端](#windows-桌面客户端)
- [快速开始](#快速开始)
- [项目结构](#项目结构)
- [模块说明](#模块说明)
- [AI 模型分工](#ai-模型分工)
- [账号矩阵布局](#账号矩阵布局)
- [创作流程 SOP](#创作流程-sop)
- [变现矩阵](#变现矩阵)
- [版权合规](#版权合规)
- [配置说明](#配置说明)
- [API 接口](#api-接口)
- [属性测试](#属性测试)
- [阶段目标](#阶段目标)

---

## 项目简介

本项目是一套完整的 AI 小说矩阵工作室系统，聚焦商业网文赛道，实现"多账号、多题材、多变现"的规模化运营。

系统通过爬虫自动采集优质网文语料，经清洗分类后注入专项智能体，驱动从选题到发布的全自动创作流水线，同时内置版权合规留存机制，保障内容安全。

**核心特点：**

- 多模型分工协作（MiniMax / 豆包 / Qwen / DeepSeek 各司其职）
- 智能爬虫系统（自动采集、清洗、分类语料）
- 专项智能体（针对不同题材的定制化创作引擎）
- 完整数据流（爬虫 → 分类 → 语料库 → 智能体 → 创作流水线）
- 版权合规机制（创作痕迹留存、人工审核）
- **Windows 桌面 GUI**（PyQt6 深色主题，可视化管理全部业务）

---

## 核心目标

| 阶段 | 时间 | 目标 |
|------|------|------|
| 初期 | 1-3 个月 | 跑通单账号/单题材变现流程，月均收益 1000-3000 元 |
| 中期 | 4-6 个月 | 搭建 3-5 个差异化账号矩阵，月均总收益 8000-15000 元 |
| 长期 | 7-12 个月 | 规模化运营，月均总收益 20000 元以上，打造 1-2 个优质 IP |

---

## 系统架构

```
数据采集层          数据处理层          AI 调度层           创作流水线
┌──────────┐      ┌──────────┐      ┌──────────┐      ┌──────────────┐
│ 爬虫调度器 │─────▶│ 内容清洗  │─────▶│ 模型路由器 │─────▶│ 选题生成      │
│ 番茄爬虫  │      │ 内容分类  │      │ MiniMax  │      │ 大纲生成      │
│ 七猫爬虫  │      │ 语料库    │      │ 豆包     │      │ 正文生成      │
│ 知乎爬虫  │      └──────────┘      │ Qwen     │      │ 内容润色      │
└──────────┘                        │ DeepSeek │      │ 人工审核      │
                                    └──────────┘      │ 发布管理      │
                                                       └──────────────┘
```

---

## 技术栈

| 类别 | 技术 |
|------|------|
| 后端框架 | Python 3.11+, FastAPI, Celery |
| 桌面 GUI | PyQt6（Windows 原生桌面应用）|
| 数据库 | MySQL 8.0（结构化数据）, MongoDB 7.0（语料/章节）, Redis 7.2（队列/缓存）|
| 爬虫 | Playwright（动态渲染）, httpx + BeautifulSoup（静态页面）, APScheduler（调度）|
| AI 模型 | MiniMax API, 豆包 API, Qwen API, DeepSeek API |
| NLP | jieba（中文分词）, scikit-learn（TF-IDF 分类）|
| 测试 | pytest, Hypothesis（属性测试）|
| 部署 | Docker, docker-compose |

---

## Windows 桌面客户端

系统提供基于 PyQt6 的 Windows 原生桌面应用，深色主题，无需浏览器，直连本地 MySQL 即可使用。

### 界面预览

```
┌─────────────────────────────────────────────────────────────┐
│  ✍ AI小说工作室          数据看板                  [统计周期▼] │
│  矩阵创作管理系统  ├──────────────────────────────────────────┤
│                   │  总账号数   活跃账号   总任务数   待审核    │
│  📊  数据看板  ◀  │   4         3          12         2       │
│  📋  创作任务     │──────────────────────────────────────────│
│  👤  账号矩阵     │  账号       平台    题材   章节  字数  收益 │
│  📚  语料库       │  重生大佬   番茄    女频重生  6  9,200  ¥46 │
│  🔔  系统告警     │  都市无敌   七猫    男频爽文  4  6,800  ¥34 │
│  ⚙️  系统设置     │  悬疑推理社 知乎    悬疑短篇  2  8,500  ¥85 │
└─────────────────────────────────────────────────────────────┘
```

### 功能页面

| 页面 | 功能说明 |
|------|---------|
| 📊 数据看板 | 统计卡片（账号数/任务数/字数/收益）+ 账号发布明细，支持日/周/月切换 |
| 📋 创作任务 | 任务列表（按阶段筛选）、新建任务、查看详情、审核通过/拒绝 |
| 👤 账号矩阵 | 账号增删改查，设置平台/题材/每日配额/状态 |
| 📚 语料库 | 语料统计卡片 + 列表，按题材/类型/质量分筛选 |
| 🔔 系统告警 | 告警列表，标记处理，筛选未处理/已处理 |
| ⚙️ 系统设置 | 修改 MySQL 连接配置 + 各模型 API Key，一键测试连接 |

### 启动桌面客户端

**方式一：双击启动（推荐）**

```
双击项目根目录的 启动GUI.bat
```

脚本会自动检查并安装依赖，然后启动应用。

**方式二：手动启动**

```bash
# 安装 GUI 依赖
pip install PyQt6 SQLAlchemy PyMySQL cryptography

# 启动
python -m ai_novel_studio.gui.app
```

### 数据库连接

默认连接配置：

| 参数 | 默认值 |
|------|--------|
| 主机 | localhost |
| 端口 | 3306 |
| 用户名 | root |
| 密码 | root |
| 数据库 | ai_novel_studio |

> 可在「系统设置 → 数据库连接」页面修改，修改后重启生效。

### GUI 专用依赖

```
PyQt6>=6.6.0
SQLAlchemy>=2.0.29
PyMySQL>=1.1.0
cryptography>=42.0.0
```

---

### 前置要求

- Docker & Docker Compose
- 各 AI 模型的 API Key（MiniMax / 豆包 / Qwen / DeepSeek）

### 1. 克隆项目

```bash
git clone <repo-url>
cd ai_novel_studio
```

### 2. 配置环境变量

```bash
cp .env.example .env
# 编辑 .env，填入各 AI 模型 API Key
```

`.env` 关键配置项：

```env
# 数据库
MYSQL_URL=mysql+aiomysql://root:password@mysql:3306/ai_novel_studio
MONGODB_URL=mongodb://mongodb:27017
REDIS_URL=redis://redis:6379/0

# AI 模型 API Keys
MINIMAX_API_KEY=your_key_here
DOUBAO_API_KEY=your_key_here
QWEN_API_KEY=your_key_here
DEEPSEEK_API_KEY=your_key_here
```

### 3. 启动服务

```bash
docker compose up -d
```

服务启动后：
- API 文档：http://localhost:8000/docs
- 健康检查：http://localhost:8000/health

### 4. 本地开发

```bash
pip install -r requirements.txt
playwright install chromium --with-deps
uvicorn ai_novel_studio.api.main:app --reload --port 8000
```

---

## 项目结构

```
ai_novel_studio/
├── config/
│   ├── models.yaml          # AI 模型 API 配置 + 阶段路由
│   ├── agents.yaml          # 各智能体系统提示词模板
│   └── spiders.yaml         # 爬虫目标站点与调度配置
├── crawler/
│   ├── spiders/             # 番茄 / 七猫 / 知乎 爬虫
│   ├── cleaner.py           # 内容清洗器
│   ├── classifier.py        # TF-IDF 分类器 + 关键词分类器
│   ├── pipeline.py          # 去重写入管道
│   └── scheduler.py         # APScheduler 调度器
├── models/
│   ├── config.py            # ModelProvider / CreationStage 枚举
│   ├── base.py              # 抽象基类（指数退避重试）
│   ├── minimax/doubao/qwen/deepseek.py
│   └── router.py            # 模型路由 + fallback 降级
├── agents/
│   ├── corpus_loader.py     # 语料加载器
│   ├── base.py              # NovelAgent 基类
│   ├── female_rebirth/male_power/suspense/romance.py
│   └── registry.py          # AgentRegistry 单例
├── pipeline/
│   ├── copyright_tracer.py  # 版权留存
│   ├── states.py            # TaskStore 状态机
│   ├── tasks.py             # Celery 流水线
│   └── publisher.py         # 多平台发布 + 配额管理
├── api/
│   ├── main.py              # FastAPI 入口
│   ├── review.py            # 人工审核接口
│   ├── accounts.py          # 账号矩阵管理
│   ├── dashboard.py         # 数据看板
│   └── corpus.py            # 语料统计查询
├── gui/                     # ★ Windows 桌面客户端
│   ├── app.py               # 应用入口
│   ├── main_window.py       # 主窗口（侧边导航）
│   ├── db.py                # 同步 MySQL 连接层
│   ├── styles.py            # 深色主题样式表
│   ├── requirements_gui.txt # GUI 专用依赖
│   └── pages/
│       ├── dashboard.py     # 数据看板页
│       ├── tasks.py         # 创作任务页
│       ├── accounts.py      # 账号矩阵页
│       ├── corpus.py        # 语料库页
│       ├── alerts.py        # 系统告警页
│       └── settings.py      # 系统设置页
├── storage/
│   ├── mysql.py             # SQLAlchemy ORM（异步，供后端使用）
│   ├── mongo.py             # Motor 异步客户端
│   └── redis_client.py      # Redis 连接池
├── tests/
│   ├── test_classifier.py   # P1 / P5 / P7 属性测试
│   ├── test_model_router.py # P2 属性测试
│   ├── test_agents.py       # P3 属性测试
│   └── test_pipeline.py     # P4 / P6 / P8 属性测试
├── database/schema.sql      # MySQL 完整建表脚本
├── Dockerfile
├── Dockerfile.celery
├── docker-compose.yml
├── requirements.txt         # 后端完整依赖
└── .env.example
启动GUI.bat                  # ★ Windows 一键启动脚本
```

---

## 模块说明

### 爬虫采集

从番茄小说、七猫小说、知乎盐选自动采集免费内容，支持：

- Playwright 模拟浏览器处理动态渲染页面（番茄）
- User-Agent 轮换 + 请求延迟（≥2秒）规避反爬
- MD5 内容哈希去重，防止重复写入
- APScheduler 按 cron 表达式定时调度

### 内容清洗与分类

- `ContentCleaner`：HTML 清洗 → 广告移除 → 空白规范化 → 特殊字符过滤
- `ContentClassifier`：jieba 分词 + TF-IDF + 朴素贝叶斯，支持四个题材分类
- `KeywordBasedClassifier`：关键词命中补充分类

**支持题材：**

| 分类 | 说明 | 关键词示例 |
|------|------|-----------|
| `female_rebirth` | 女频重生 | 重生、穿越、虐渣、马甲、打脸 |
| `male_power` | 男频爽文 | 都市、异能、系统、签到、无敌 |
| `suspense` | 悬疑短篇 | 悬疑、推理、侦探、反转、密室 |
| `romance` | 甜宠 | 甜宠、恋爱、暖文、校园、青梅竹马 |

### 专项智能体

每个智能体绑定一个题材，持有对应语料上下文，通过 `agents.yaml` 配置系统提示词：

```
语料库(MongoDB)
    ↓
CorpusLoader.load_for_agent(agent_type)  # 加载高质量语料（quality_score ≥ 0.8）
    ↓
NovelAgent.build_system_prompt()          # 将语料嵌入 system prompt
    ↓
模型 API 调用（选题 / 大纲 / 正文 / 润色）
```

### 创作流水线

基于 Celery 的异步任务链，支持：

- 串行编排：选题 → 大纲 → 正文 → 润色 → 人工审核 → 发布
- 失败自动重试（指数退避，最多 3 次）
- 幂等性保护（相同 task_id 不重复创建）
- 任务状态全程追踪

---

## AI 模型分工

| 模型 | 负责环节 | 核心优势 | 参考成本 |
|------|---------|---------|---------|
| **MiniMax** | 选题生成、灵感激发 | 创意生成、热点分析 | ¥0.01/千 tokens |
| **豆包** | 大纲生成、结构设计 | 中文理解、逻辑严谨 | ¥0.008/千 tokens |
| **Qwen** | 正文生成、细节填充 | 长文本生成、网文风格 | ¥0.006/千 tokens |
| **DeepSeek** | 内容润色、去 AI 味 | 语言润色、风格优化 | ¥0.001/千 tokens |

模型路由配置（`config/models.yaml`）：

```yaml
stage_routing:
  topic_generation:    minimax
  outline_generation:  doubao
  content_generation:  qwen
  polish:              deepseek
  fallback:            qwen     # 主模型失败时自动降级
```

---

## 账号矩阵布局

| 账号类型 | 核心题材 | 目标平台 | 变现方式 |
|---------|---------|---------|---------|
| 主账号1（流量款）| 女频·重生虐渣+马甲 | 番茄小说 | 保底签约+全勤+分成 |
| 主账号2（变现款）| 男频·都市异能+爽文 | 七猫小说 | 保底签约+全勤+渠道分成 |
| 辅账号1（快变现）| 短篇·悬疑反转 | 知乎盐选 | 短篇买断+付费解锁 |
| 辅账号2（引流款）| 甜宠短篇/剧情节选 | 小红书/抖音 | 引流至核心平台+私域变现 |

---

## 创作流程 SOP

```
选题（1天）→ 大纲（1-2天）→ 正文（日均6000-10000字）→ 审核（1小时）→ 发布（30分钟）→ 复盘（每周）
```

**正文生成策略：** 将单章拆分为 5 个模块分别生成，每个模块生成后人工修改，确保内容有"人味"：

```
场景铺垫 → 对话冲突 → 爽点打脸 → 结尾钩子
```

---

## 变现矩阵

**初期（1-3 个月）：快速回款**
- 保底/全勤签约：新人千字 10-50 元 + 全勤奖金 600-2000 元/月
- 短篇买断：知乎盐选，千字 50-200 元，过稿即结

**中期（4-6 个月）：拓展渠道**
- 平台分成（广告分成、付费阅读分成）
- 私域变现（付费合集、会员社群）

**长期（7-12 个月）：衍生增值**
- 短剧剧本、有声书改编
- IP 版权授权

---

## 版权合规

系统内置三层版权保护机制：

**1. 创作痕迹留存**

每次章节生成自动记录到 `copyright_traces` 表：

```
prompt_hash（提示词 MD5）+ draft_hash（初稿 MD5）+ final_hash（定稿 MD5）+ trace_time（不可修改）
```

**2. 内容去重**

所有语料通过 `content_hash`（MD5）唯一索引防止重复写入，疑似侵权内容自动标记，等待人工确认。

**3. 人工审核**

润色完成后进入 `human_review` 队列，审核通过才触发发布，拒绝时保留原因记录。

---

## 配置说明

### 模型配置（`config/models.yaml`）

```yaml
minimax:
  api_key: "${MINIMAX_API_KEY}"    # 从环境变量读取，不硬编码
  api_endpoint: "https://api.minimax.chat/v1/text/chatcompletion_v2"
  model_name: "abab6.5s-chat"
  max_tokens: 4096
  temperature: 0.8
  retry_times: 3
```

### 智能体配置（`config/agents.yaml`）

```yaml
female_rebirth:
  system_prompt_template: |
    你是专注于女频重生题材的网文创作专家。
    高频爽点关键词：{hot_keywords}
    风格参考段落：
    {corpus_samples}
    # ... 写作要求
```

### 爬虫配置（`config/spiders.yaml`）

```yaml
spiders:
  fanqie:
    enabled: true
    schedule: "0 2,14 * * *"   # 每日2次
    delay_seconds: 2
    use_playwright: true
```

> 所有配置文件支持热更新，修改后无需重启服务。

---

## API 接口

启动后访问 http://localhost:8000/docs 查看完整 Swagger 文档。

| 接口 | 方法 | 说明 |
|------|------|------|
| `/review/tasks/pending_review` | GET | 获取待审核任务列表 |
| `/review/decide` | POST | 提交审核决策（通过/拒绝）|
| `/accounts/` | GET/POST | 账号列表 / 创建账号 |
| `/accounts/{id}` | GET/PATCH/DELETE | 账号详情 / 更新 / 删除 |
| `/dashboard/summary` | GET | 数据看板（支持日/周/月聚合）|
| `/corpus/stats` | GET | 语料统计查询 |
| `/health` | GET | 服务健康检查 |

---

## 属性测试

系统使用 [Hypothesis](https://hypothesis.readthedocs.io/) 框架验证 8 条核心正确性属性：

| 属性 | 说明 | 对应需求 |
|------|------|---------|
| P1 分类器一致性 | 关键词命中 ≥3 时，两个分类器结果必须一致 | 需求 3.4 |
| P2 模型路由完备性 | 任意 CreationStage 均能返回有效客户端 | 需求 4.2 |
| P3 语料注入非空性 | 语料库非空时，style_samples 长度 ≥1 | 需求 5.2 |
| P4 流水线幂等性 | 相同 task_id 不重复创建任务 | 需求 6.3 |
| P5 内容清洗无损性 | 清洗后中文字符数 ≥ 原始的 80% | 需求 2.4 |
| P6 版权留存完整性 | 章节生成后 prompt_hash/draft_hash 不为空 | 需求 9.1 |
| P7 哈希去重性 | 相同内容哈希值必须相同 | 需求 10.4 |
| P8 发布配额约束性 | 达到 daily_quota 后拒绝新发布请求 | 需求 8.3 |

运行测试：

```bash
pytest ai_novel_studio/tests/ -v
```

---

## 阶段目标

### 初期（1-3 个月）：启动期

- [ ] 完成团队搭建（2-3 人）
- [ ] 搭建 2 个核心账号（番茄女频 + 七猫男频）
- [ ] 跑通选题→创作→审核→发布→变现全流程
- [ ] 实现日均产出 6000 字以上
- [ ] 完成至少 1 本小说平台签约

### 中期（4-6 个月）：拓展期

- [ ] 新增知乎短篇账号 + 小红书引流账号
- [ ] 日均产出提升至 10000 字以上
- [ ] 积累私域粉丝 500-1000 人
- [ ] 完成核心作品版权存证

### 长期（7-12 个月）：规模化

- [ ] 团队扩容至 5-8 人
- [ ] 搭建 5-6 个账号矩阵，覆盖 3-4 个热门题材
- [ ] 拓展短剧、有声书改编渠道
- [ ] 打造 1-2 个优质原创 IP

---

## 核心理念

> AI 是工具，不是"代笔"。人类的创意、审美与版权意识，才是工作室长久运营的核心。

系统的核心竞争力在于"用 AI 放大人类创意"——以标准化流程为基础，以 AI 工具为效率支撑，以版权合规为底线，实现"低成本、高产出、稳变现"的规模化运营。

---

## License

MIT

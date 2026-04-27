# 需求文档：AI小说矩阵工作室系统

## 简介

AI小说矩阵工作室系统是一个基于多模型协作的自动化小说创作平台。系统通过爬虫采集优质网文语料，经清洗分类后注入专项智能体，驱动"选题→大纲→正文→润色→人工审核→发布"的完整创作流水线，支持多账号矩阵管理与版权合规留存，目标是实现低成本、高产出、合规可控的规模化网文创作运营。

---

## 术语表

| 术语 | 说明 |
|------|------|
| **Crawler** | 爬虫模块，负责从目标网站采集小说内容 |
| **ContentCleaner** | 内容清洗器，负责去除HTML、广告、特殊字符 |
| **ContentClassifier** | 内容分类器，基于TF-IDF与关键词对语料进行题材分类 |
| **KeywordClassifier** | 关键词分类器，作为ContentClassifier的补充分类手段 |
| **CorpusLoader** | 语料加载器，从MongoDB按题材加载训练语料 |
| **NovelAgent** | 专项创作智能体，绑定特定题材语料与模型路由 |
| **AgentRegistry** | 智能体注册表，管理所有NovelAgent实例 |
| **ModelRouter** | 模型路由器，根据创作环节分配对应AI模型客户端 |
| **ModelClient** | AI模型客户端，封装对各模型API的调用 |
| **Pipeline** | 创作流水线，基于Celery编排的自动化创作任务链 |
| **TaskStore** | 任务存储，管理创作任务的状态与数据 |
| **ReviewAPI** | 人工审核接口，供审核人员对流水线产出内容进行审批 |
| **Publisher** | 发布模块，负责将定稿内容适配并发布到目标平台 |
| **CopyrightTracer** | 版权留存模块，记录提示词哈希、初稿哈希、定稿哈希 |
| **AccountManager** | 账号矩阵管理模块，管理各平台账号及发布配额 |
| **AgentType** | 智能体类型枚举（female_rebirth / male_power / suspense / romance） |
| **CreationStage** | 创作环节枚举（topic_generation / outline_generation / content_generation / polish） |
| **NovelCategory** | 小说题材分类枚举，与AgentType一一对应 |
| **TaskStage** | 任务阶段枚举（pending / topic_generating / outline_generating / content_generating / polishing / human_review / publishing / done / rejected） |

---

## 需求列表

---

### 需求 1：爬虫采集

**用户故事：** 作为AI运营师，我希望系统能自动从番茄小说、七猫小说、知乎盐选等平台采集免费小说内容，以便持续补充语料库，降低人工收集成本。

#### 验收标准

1. THE Crawler SHALL 支持番茄小说、七猫小说、知乎盐选三个目标网站的内容采集。
2. WHEN Crawler 向目标网站发起请求时，THE Crawler SHALL 轮换 User-Agent 并设置请求延迟（≥2秒），以规避反爬检测。
3. WHEN 目标页面为动态渲染内容时，THE Crawler SHALL 使用 Playwright 模拟浏览器加载页面后再提取内容。
4. WHEN Crawler 成功采集到章节内容时，THE Crawler SHALL 计算内容的 MD5 哈希值并存储，用于后续去重。
5. IF 同一内容哈希已存在于语料库中，THEN THE Crawler SHALL 跳过该内容，不重复写入。
6. THE Crawler SHALL 按照配置文件中指定的调度频率（每日或每周）自动执行采集任务。
7. WHEN 采集任务执行完成时，THE Crawler SHALL 将采集结果（来源、书名、章节标题、正文、采集时间）写入 MongoDB 原始语料集合。

---

### 需求 2：内容清洗

**用户故事：** 作为AI运营师，我希望系统能自动清洗爬取的原始内容，去除广告、HTML标签和无效字符，以便获得干净可用的语料。

#### 验收标准

1. WHEN ContentCleaner 接收到含 HTML 标签的原始内容时，THE ContentCleaner SHALL 去除所有 HTML 标签，仅保留纯文本。
2. WHEN ContentCleaner 处理内容时，THE ContentCleaner SHALL 识别并移除广告模式文本（包括"本章未完，点击下一页"、"下载APP阅读全文"等常见广告语）。
3. WHEN ContentCleaner 处理内容时，THE ContentCleaner SHALL 规范化空白字符，将连续空白合并为单个空格，段落间距统一为双换行。
4. FOR ALL 含中文字符的原始内容，ContentCleaner.clean() 处理后，中文字符数量 SHALL 不少于原始内容中文字符数量的 80%。（对应属性 P5）
5. WHEN ContentCleaner 完成清洗后，THE ContentCleaner SHALL 验证内容有效性：内容长度不少于 500 字符，且中文字符占比不低于 70%。
6. IF 内容未通过有效性验证，THEN THE ContentCleaner SHALL 将该内容标记为无效，不写入训练语料库。

---

### 需求 3：内容分类

**用户故事：** 作为AI运营师，我希望系统能自动将清洗后的语料按题材分类（女频重生、男频爽文、悬疑短篇、甜宠），以便各专项智能体能加载对应语料。

#### 验收标准

1. THE ContentClassifier SHALL 支持将文本分类到以下四个题材之一：female_rebirth（女频重生）、male_power（男频爽文）、suspense（悬疑短篇）、romance（甜宠）。
2. WHEN ContentClassifier 对文本进行分类时，THE ContentClassifier SHALL 使用基于 jieba 分词的 TF-IDF 向量化方法提取特征。
3. THE KeywordClassifier SHALL 基于预定义关键词列表对文本进行分类，作为 ContentClassifier 的补充手段。
4. WHEN 同一文本的关键词命中数量 ≥ 3 时，THE ContentClassifier 与 THE KeywordClassifier 的分类结果 SHALL 一致。（对应属性 P1）
5. FOR ALL 有效文本输入，ContentClassifier.predict() SHALL 返回一个有效的 NovelCategory 枚举值，不得抛出异常。
6. WHEN ContentClassifier 完成分类后，THE System SHALL 将分类结果与语料一同写入 MongoDB，并更新 PostgreSQL 中的结构化分类记录。

---

### 需求 4：模型配置与路由

**用户故事：** 作为项目负责人，我希望系统能根据创作环节自动选择对应的AI模型（MiniMax选题、豆包大纲、Qwen正文、DeepSeek润色），以便充分发挥各模型优势，降低API成本。

#### 验收标准

1. THE ModelRouter SHALL 维护创作环节到模型提供商的映射关系：topic_generation → MiniMax，outline_generation → 豆包，content_generation → Qwen，polish → DeepSeek。
2. FOR ALL CreationStage 枚举值，ModelRouter.get_client_for_stage() SHALL 返回一个有效的 ModelClient 实例，不得抛出 KeyError 或返回 None。（对应属性 P2）
3. WHERE 主模型调用失败，THE ModelRouter SHALL 切换到配置的 fallback_model（默认为 Qwen）重试。
4. WHEN ModelClient 调用模型 API 失败时，THE ModelClient SHALL 按指数退避策略（2^n 秒）自动重试，最多重试 3 次。
5. IF 重试次数耗尽后仍失败，THEN THE ModelClient SHALL 抛出异常并记录错误日志，不得静默失败。
6. THE ModelConfig SHALL 支持通过 YAML 配置文件设置各模型的 api_key、api_endpoint、model_name、max_tokens、temperature、top_p、timeout、retry_times 参数。
7. WHEN 配置文件中的 api_key 为环境变量占位符时，THE System SHALL 在启动时从环境变量读取实际值，不得将密钥硬编码在代码中。

---

### 需求 5：专项智能体管理

**用户故事：** 作为AI运营师，我希望系统为每个题材维护一个专项智能体，智能体能自动加载对应语料并生成符合题材风格的内容，以便提升创作质量和效率。

#### 验收标准

1. THE System SHALL 维护四个专项智能体：女频重生智能体、男频爽文智能体、悬疑短篇智能体、甜宠智能体，每个智能体绑定对应的 AgentType。
2. WHEN NovelAgent 初始化时，THE CorpusLoader SHALL 从 MongoDB 按 AgentType 加载对应题材的语料，返回的 style_samples 列表长度 SHALL ≥ 1（语料库非空时）。（对应属性 P3）
3. WHEN NovelAgent.build_system_prompt() 被调用时，THE NovelAgent SHALL 将语料样本和热词嵌入系统提示词模板，返回的提示词字符串 SHALL 包含语料样本内容。
4. THE AgentRegistry SHALL 以单例模式管理所有 NovelAgent 实例，支持通过 AgentType 获取对应智能体。
5. WHEN 通过 AgentRegistry 请求不存在的 AgentType 时，THE AgentRegistry SHALL 抛出明确的错误，不得返回 None。
6. THE NovelAgent SHALL 提供以下创作接口：generate_topic（选题生成）、generate_outline（大纲生成）、generate_chapter（章节正文生成）、polish_content（内容润色）。
7. WHEN generate_chapter 被调用时，THE NovelAgent SHALL 接受上一章摘要作为上下文输入，以保持人设和情节连贯性。

---

### 需求 6：创作流水线

**用户故事：** 作为AI运营师，我希望系统能自动按顺序执行选题→大纲→正文→润色的创作流程，并在完成后进入人工审核队列，以便减少人工干预，提升产出效率。

#### 验收标准

1. THE Pipeline SHALL 按以下顺序串行执行创作任务：task_generate_topic → task_generate_outline → task_generate_chapters → task_polish。
2. WHEN 流水线中任意任务失败时，THE Pipeline SHALL 自动重试（最多 3 次），重试间隔按指数退避策略计算。
3. FOR ALL task_id，重复调用 start_creation_pipeline() 时，THE Pipeline SHALL 不创建重复任务，而是返回已有任务的当前状态。（对应属性 P4）
4. WHEN task_polish 完成时，THE Pipeline SHALL 将任务状态更新为 human_review，并通知审核人员。
5. THE TaskStore SHALL 记录每个任务的完整状态历史，包括各阶段的开始时间、完成时间和产出内容。
6. WHEN 任务状态发生变更时，THE TaskStore SHALL 同步更新 PostgreSQL 中 creation_tasks 表的 stage 和 updated_at 字段。
7. THE Pipeline SHALL 支持通过 Celery 任务队列（Redis broker）进行异步调度，不阻塞 API 主进程。

---

### 需求 7：人工审核

**用户故事：** 作为内容审核员，我希望通过管理接口查看待审核内容并做出通过/拒绝决定，以便在发布前保障内容质量和合规性。

#### 验收标准

1. THE ReviewAPI SHALL 提供 GET /tasks/pending_review 接口，返回所有处于 human_review 阶段的任务列表。
2. THE ReviewAPI SHALL 提供 POST /review/decide 接口，接受 task_id、approved（布尔值）、comments（可选文字说明）参数。
3. WHEN 审核人员提交 approved=true 时，THE ReviewAPI SHALL 将任务状态更新为 publishing，并触发发布任务。
4. WHEN 审核人员提交 approved=false 时，THE ReviewAPI SHALL 将任务状态更新为 rejected，记录拒绝原因，并将任务重新加入 pending 队列。
5. IF 提交审核决定时 task_id 不存在，THEN THE ReviewAPI SHALL 返回 HTTP 404 错误，并附带明确的错误描述。
6. WHEN 任务被拒绝后重新进入 pending 状态时，THE System SHALL 保留原有的拒绝原因记录，供后续创作参考。

---

### 需求 8：发布管理

**用户故事：** 作为AI运营师，我希望系统能将审核通过的内容按平台要求格式化后发布，并记录发布数据，以便追踪各账号的产出与收益。

#### 验收标准

1. THE Publisher SHALL 支持将内容发布到番茄小说、七猫小说、知乎盐选三个平台，并按各平台要求适配标题格式和章节字数（番茄：1500-2000字/章，知乎：3000-5000字/篇）。
2. THE AccountManager SHALL 为每个账号维护 daily_quota（每日发布章节数上限），默认值为 3。
3. WHEN 账号当日已发布章节数达到 daily_quota 时，THE Publisher SHALL 拒绝新的发布请求，并返回配额已满的提示。（对应属性 P8）
4. FOR ALL 发布操作，THE System SHALL 在 PostgreSQL publish_records 表中创建发布记录，包含 task_id、account_id、platform、chapter_no、word_count、published_at 字段。
5. WHEN 发布成功后，THE System SHALL 更新对应发布记录的 read_count 和 revenue 字段（初始值为 0，后续通过数据同步更新）。
6. THE AccountManager SHALL 支持通过 FastAPI 接口对账号进行增删改查操作，包括设置账号状态（active/inactive）。

---

### 需求 9：版权合规留存

**用户故事：** 作为项目负责人，我希望系统自动记录每次创作的提示词哈希、AI初稿哈希和定稿哈希，以便在版权纠纷时提供可追溯的创作痕迹。

#### 验收标准

1. WHEN 章节内容生成完成时，THE CopyrightTracer SHALL 在 PostgreSQL copyright_traces 表中创建一条记录，包含对应的 task_id。（对应属性 P6）
2. THE CopyrightTracer SHALL 计算并存储以下哈希值：prompt_hash（提示词 MD5）、draft_hash（AI 初稿 MD5），两者均不得为空或 NULL。（对应属性 P6）
3. WHEN 内容经人工修改定稿后，THE CopyrightTracer SHALL 更新对应记录的 final_hash（定稿 MD5）。
4. FOR ALL 已完成章节，copyright_traces 表中 SHALL 存在对应 task_id 的记录，且 prompt_hash 与 draft_hash 均不为空。（对应属性 P6）
5. THE System SHALL 确保 copyright_traces 记录的 timestamp 与章节生成时间一致，不得事后篡改。
6. IF copyright_traces 写入失败，THEN THE System SHALL 记录错误日志并触发告警，不得静默忽略版权留存失败。

---

### 需求 10：语料库管理

**用户故事：** 作为AI运营师，我希望系统能管理原始语料和训练语料，支持按题材查询和质量筛选，以便为智能体提供高质量的语料输入。

#### 验收标准

1. THE System SHALL 在 MongoDB 中维护两个集合：原始语料集合（存储爬取清洗后的内容）和训练语料集合（存储质量评分 ≥ 0.8 的高质量语料）。
2. THE CorpusLoader SHALL 支持按 AgentType 从训练语料集合中加载语料，并按 quality_score 降序排列。
3. WHEN 语料质量评分低于 0.8 时，THE System SHALL 不将该语料加入训练语料集合。
4. THE System SHALL 为每条语料计算并存储 content_hash，WHEN 新语料的 content_hash 已存在时，THE System SHALL 跳过写入，防止重复。（对应属性 P7）
5. THE System SHALL 提供 FastAPI 接口，支持按 category、quality_score 范围、crawl_time 范围查询语料统计信息。

---

### 需求 11：数据分析看板

**用户故事：** 作为项目负责人，我希望通过管理看板查看各账号的阅读量、收藏量、发布量和收益数据，以便进行数据复盘和资源优化。

#### 验收标准

1. THE System SHALL 提供 FastAPI 数据看板接口，返回各账号的发布章节总数、累计字数、累计阅读量、累计收益。
2. THE System SHALL 支持按时间范围（日/周/月）聚合统计各账号的发布数据。
3. WHEN 查询数据看板时，THE System SHALL 从 PostgreSQL publish_records 表聚合计算，响应时间 SHALL 不超过 3 秒。
4. THE System SHALL 支持按 platform 和 agent_type 维度过滤统计数据。

---

### 需求 12：系统配置与部署

**用户故事：** 作为项目负责人，我希望系统能通过配置文件管理所有模型参数、爬虫目标和智能体提示词，并支持 Docker 容器化部署，以便降低运维成本。

#### 验收标准

1. THE System SHALL 通过 models.yaml 配置文件管理各模型的 API 参数，支持通过环境变量注入 API Key，不得硬编码密钥。
2. THE System SHALL 通过 agents.yaml 配置文件管理各智能体的系统提示词模板，支持热更新（无需重启服务）。
3. THE System SHALL 通过 spiders.yaml 配置文件管理爬虫目标站点、调度频率和反爬参数。
4. THE System SHALL 提供 docker-compose.yml，包含 FastAPI 服务、Celery Worker、PostgreSQL、MongoDB、Redis 的完整编排配置。
5. WHEN 任意依赖服务（PostgreSQL/MongoDB/Redis）不可用时，THE System SHALL 在启动时输出明确的错误信息，不得静默启动后在运行时崩溃。

---

### 需求 13：非功能性需求

**用户故事：** 作为项目负责人，我希望系统具备基本的性能、可靠性和安全性保障，以便支撑规模化运营。

#### 验收标准

1. THE Pipeline SHALL 支持并发处理至少 5 个创作任务，不同任务之间不得相互阻塞。
2. THE ReviewAPI SHALL 在正常负载下，接口响应时间 SHALL 不超过 500ms（P95）。
3. WHEN 模型 API 出现限流（HTTP 429）时，THE ModelClient SHALL 等待至少 60 秒后重试，不得立即重试导致进一步限流。
4. THE System SHALL 对所有 FastAPI 接口进行输入参数校验，IF 参数不合法，THEN THE System SHALL 返回 HTTP 422 错误，并附带字段级错误描述。
5. THE System SHALL 不在日志或接口响应中暴露模型 API Key 或数据库连接字符串等敏感信息。
6. WHEN 爬虫采集到的内容与已知版权作品的相似度超过阈值时，THE System SHALL 标记该内容为疑似侵权，不自动加入训练语料库，等待人工确认。

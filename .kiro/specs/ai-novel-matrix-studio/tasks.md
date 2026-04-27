# 实现计划：AI小说矩阵工作室系统

## 概述

按照"基础设施→数据采集→内容处理→模型层→智能体→流水线→API→部署"的依赖顺序，将系统拆解为可增量交付的编码任务。每个任务均引用对应需求条款，属性测试任务标注对应属性编号（P1-P8）。

---

## 任务列表

- [x] 1. 项目基础结构与数据库层
  - [x] 1.1 执行 MySQL 数据库初始化脚本
    - 执行 `ai_novel_studio/database/schema.sql`，创建数据库及全部表、视图、索引
    - 验证 16 张表/视图均创建成功，检查外键约束与唯一索引是否生效
    - _需求：1.4、6.6、8.4、9.1、10.4、11.1_

  - [x] 1.2 初始化项目目录结构与依赖配置
    - 创建 `ai_novel_studio/` 目录骨架（config/crawler/models/agents/pipeline/api/storage/tests）
    - 编写 `requirements.txt`，包含 FastAPI、Celery、Scrapy、Playwright、SQLAlchemy、Motor、Redis、Hypothesis、jieba、scikit-learn、pydantic、httpx 等依赖
    - _需求：12.1_

  - [x] 1.3 实现 MySQL ORM 模型
    - 在 `storage/mysql.py` 中配置 SQLAlchemy 异步引擎与 Session（连接 MySQL）
    - 定义 ORM 模型：`CreationTask`、`Account`、`PublishRecord`、`CopyrightTrace`、`Chapter`，字段与 schema.sql 一致
    - _需求：6.6、8.4、9.1、9.2_

  - [x] 1.4 实现 MongoDB 连接与集合操作
    - 在 `storage/mongo.py` 中配置 Motor 异步客户端
    - 封装原始语料集合、训练语料集合、章节内容集合的 CRUD 操作
    - _需求：1.7、10.1_

  - [x] 1.5 实现 Redis 连接客户端
    - 在 `storage/redis_client.py` 中封装 Redis 连接池与常用操作（get/set/incr/expire）
    - _需求：6.7_

  - [x] 1.6 实现启动时依赖服务健康检查
    - 在 `api/main.py` 启动事件中检查 MySQL、MongoDB、Redis 连通性
    - 任意服务不可用时输出明确错误信息并退出，不静默启动
    - _需求：12.5_

- [x] 2. 配置管理模块
  - [x] 2.1 实现模型配置数据结构与 YAML 加载
    - 在 `models/config.py` 中定义 `ModelProvider`、`CreationStage`、`ModelConfig`、`ModelRouter` Pydantic 模型
    - 编写 `config/models.yaml` 示例配置（含环境变量占位符 `${API_KEY}`）
    - 实现配置加载器，从环境变量注入 API Key，不硬编码密钥
    - _需求：4.6、4.7、12.1_

  - [x] 2.2 实现智能体与爬虫配置文件
    - 编写 `config/agents.yaml`（各智能体系统提示词模板，支持热更新）
    - 编写 `config/spiders.yaml`（目标站点、调度频率、反爬参数）
    - 实现配置热更新机制（文件变更时重新加载，无需重启）
    - _需求：12.2、12.3_

- [x] 3. 爬虫采集模块
  - [x] 3.1 实现爬虫基类 `NovelSpider`
    - 在 `crawler/spiders/base.py` 中实现 User-Agent 轮换、请求延迟（≥2秒）、MD5 内容哈希计算
    - _需求：1.2、1.4_

  - [x] 3.2 实现番茄小说爬虫（Playwright 动态渲染）
    - 在 `crawler/spiders/fanqie.py` 中实现 `FanqieSpider`
    - 使用 Playwright 模拟浏览器加载动态页面后提取内容
    - _需求：1.1、1.3_

  - [x] 3.3 实现七猫小说爬虫与知乎盐选爬虫
    - 在 `crawler/spiders/qimao.py` 和 `crawler/spiders/zhihu.py` 中分别实现对应爬虫
    - _需求：1.1_

  - [x] 3.4 实现爬虫去重与 MongoDB 写入
    - 在爬虫 pipeline 中检查 `content_hash` 是否已存在，重复则跳过
    - 将采集结果（来源、书名、章节标题、正文、采集时间）写入 MongoDB 原始语料集合
    - _需求：1.5、1.7_

  - [x] 3.5 实现爬虫调度器
    - 在 `crawler/scheduler.py` 中使用 APScheduler 按 `spiders.yaml` 配置的频率调度各爬虫
    - _需求：1.6_

- [x] 4. 内容清洗与分类模块
  - [x] 4.1 实现 `ContentCleaner`
    - 在 `crawler/cleaner.py` 中实现 HTML 清洗、广告移除、空白规范化、特殊字符过滤
    - 实现 `validate_content()`：长度 ≥ 500 字符且中文占比 ≥ 70%
    - _需求：2.1、2.2、2.3、2.5、2.6_

  - [x] 4.2 编写属性测试：内容清洗无损性（P5）
    - **属性 P5：ContentCleaner.clean() 处理后，中文字符数量不少于原始内容中文字符数量的 80%**
    - **验证需求：2.4**
    - 在 `tests/test_classifier.py` 中使用 Hypothesis `@given` 生成含中文字符的随机文本进行验证

  - [x] 4.3 实现 `ContentClassifier`（TF-IDF + 朴素贝叶斯）
    - 在 `crawler/classifier.py` 中实现基于 jieba 分词的 TF-IDF 向量化与 MultinomialNB 分类器
    - 实现 `train()`、`predict()`、`predict_proba()`、`save()`、`load()` 方法
    - `predict()` 对所有有效输入必须返回合法 `NovelCategory` 枚举值，不得抛出异常
    - _需求：3.1、3.2、3.5_

  - [x] 4.4 实现 `KeywordBasedClassifier`
    - 在 `crawler/classifier.py` 中实现基于预定义关键词列表的补充分类器
    - _需求：3.3_

  - [x] 4.5 编写属性测试：分类器一致性（P1）
    - **属性 P1：对同一文本，当关键词命中数 ≥ 3 时，ContentClassifier 与 KeywordBasedClassifier 的分类结果必须一致**
    - **验证需求：3.4**
    - 在 `tests/test_classifier.py` 中使用 Hypothesis 生成含高频关键词的文本进行验证

  - [x] 4.6 实现分类结果写入 MongoDB 与 PostgreSQL
    - 分类完成后将结果与语料写入 MongoDB，并更新 PostgreSQL 结构化分类记录
    - _需求：3.6_

- [x] 5. 模型客户端与路由模块
  - [x] 5.1 实现 `BaseModelClient` 抽象基类与指数退避重试
    - 在 `models/base.py` 中实现 `generate()` 抽象方法与 `generate_with_retry()`（指数退避 2^n 秒，最多重试 3 次）
    - HTTP 429 限流时等待至少 60 秒后重试
    - 重试耗尽后抛出异常并记录错误日志，不静默失败
    - _需求：4.4、4.5、13.3_

  - [x] 5.2 实现四个模型客户端
    - 分别在 `models/minimax.py`、`models/doubao.py`、`models/qwen.py`、`models/deepseek.py` 中实现各 API 调用逻辑
    - _需求：4.1_

  - [x] 5.3 实现 `ModelRouter` 与 `ModelClientFactory`
    - 在 `models/router.py` 中实现 `get_client_for_stage()` 方法，维护 `CreationStage → ModelProvider` 映射
    - 主模型失败时切换到 `fallback_model`（默认 Qwen）重试
    - _需求：4.1、4.3_

  - [x] 5.4 编写属性测试：模型路由完备性（P2）
    - **属性 P2：对任意 CreationStage 枚举值，ModelRouter.get_client_for_stage() 必须返回有效的 BaseModelClient，不得抛出 KeyError**
    - **验证需求：4.2**
    - 在 `tests/test_model_router.py` 中使用 Hypothesis `st.sampled_from(list(CreationStage))` 验证

- [x] 6. 语料加载与专项智能体模块
  - [x] 6.1 实现 `CorpusLoader`
    - 在 `agents/corpus_loader.py` 中实现按 `AgentType` 从 MongoDB 训练语料集合加载语料，按 `quality_score` 降序排列
    - 语料质量评分 < 0.8 的不加入训练语料集合
    - _需求：5.2、10.2、10.3_

  - [x] 6.2 编写属性测试：语料注入非空性（P3）
    - **属性 P3：对任意 AgentType，CorpusLoader.load_for_agent() 在语料库非空时，返回的 style_samples 列表长度必须 ≥ 1**
    - **验证需求：5.2**
    - 在 `tests/test_agents.py` 中使用 Hypothesis `st.sampled_from(list(AgentType))` 验证

  - [x] 6.3 实现 `NovelAgent` 基类与四个专项智能体
    - 在 `agents/base.py` 中实现 `build_system_prompt()`（将语料样本和热词嵌入提示词模板）
    - 实现 `generate_topic()`、`generate_outline()`、`generate_chapter()`（接受上一章摘要）、`polish_content()` 四个创作接口
    - 分别在 `agents/female_rebirth.py`、`agents/male_power.py`、`agents/suspense.py`、`agents/romance.py` 中实现各智能体
    - _需求：5.1、5.3、5.6、5.7_

  - [x] 6.4 实现 `AgentRegistry` 单例管理
    - 在 `agents/registry.py` 中以单例模式管理所有 `NovelAgent` 实例
    - 请求不存在的 `AgentType` 时抛出明确错误，不返回 None
    - _需求：5.4、5.5_

- [x] 7. 版权留存模块
  - [x] 7.1 实现 `CopyrightTracer`
    - 在 `pipeline/` 中实现 `CopyrightTracer`，章节生成完成后在 `copyright_traces` 表写入 `task_id`、`prompt_hash`（提示词 MD5）、`draft_hash`（初稿 MD5）
    - 写入失败时记录错误日志并触发告警，不静默忽略
    - 实现 `update_final_hash()` 方法，人工定稿后更新 `final_hash`
    - _需求：9.1、9.2、9.3、9.5、9.6_

  - [x] 7.2 编写属性测试：版权留存完整性（P6）
    - **属性 P6：每次章节生成完成后，copyright_traces 表中必须存在对应 task_id 的记录，且 prompt_hash、draft_hash 均不为空**
    - **验证需求：9.1、9.4**
    - 在 `tests/test_pipeline.py` 中使用 Hypothesis 生成随机 task_id 验证写入完整性

- [x] 8. 语料库去重与管理
  - [x] 8.1 实现语料 content_hash 去重写入
    - 在语料写入逻辑中检查 `content_hash` 唯一性，重复时跳过写入
    - _需求：1.5、10.4_

  - [x] 8.2 编写属性测试：内容哈希去重性（P7）
    - **属性 P7：对任意两条 content_hash 相同的语料，系统不得允许重复写入，第二次写入必须被跳过**
    - **验证需求：1.5、10.4**
    - 在 `tests/test_classifier.py` 中使用 Hypothesis 生成相同哈希的语料对验证去重逻辑

  - [x] 8.3 实现语料库查询 FastAPI 接口
    - 在 `api/` 中实现按 `category`、`quality_score` 范围、`crawl_time` 范围查询语料统计信息的接口
    - _需求：10.5_

- [x] 9. Celery 创作流水线
  - [x] 9.1 实现 `TaskStore` 任务状态管理
    - 在 `pipeline/states.py` 中实现 `TaskStore`，管理任务状态机（pending → topic_generating → ... → done/rejected）
    - 状态变更时同步更新 PostgreSQL `creation_tasks` 表的 `stage` 和 `updated_at` 字段
    - 记录每个任务各阶段的开始时间、完成时间和产出内容
    - _需求：6.5、6.6_

  - [x] 9.2 实现 Celery 流水线任务链
    - 在 `pipeline/tasks.py` 中实现 `task_generate_topic`、`task_generate_outline`、`task_generate_chapters`、`task_polish` 四个 Celery 任务
    - 使用 `chain` 串行编排，任务失败时指数退避重试（最多 3 次）
    - `task_polish` 完成后将状态更新为 `human_review`
    - _需求：6.1、6.2、6.4、6.7_

  - [x] 9.3 实现流水线幂等性保护
    - 在 `start_creation_pipeline()` 中检查 `task_id` 是否已存在，重复调用时返回已有任务状态，不创建重复任务
    - _需求：6.3_

  - [x] 9.4 编写属性测试：流水线幂等性（P4）
    - **属性 P4：对同一 task_id，重复调用 start_creation_pipeline() 不得创建重复任务，必须返回已有任务状态**
    - **验证需求：6.3**
    - 在 `tests/test_pipeline.py` 中使用 Hypothesis 生成随机 task_id 并重复调用验证幂等性

- [x] 10. 发布管理模块
  - [x] 10.1 实现 `Publisher` 多平台发布适配
    - 在 `pipeline/publisher.py` 中实现番茄（1500-2000字/章）、七猫、知乎盐选（3000-5000字/篇）的标题格式与字数适配
    - 发布成功后在 `publish_records` 表创建记录（task_id、account_id、platform、chapter_no、word_count、published_at）
    - _需求：8.1、8.4、8.5_

  - [x] 10.2 实现 `AccountManager` 账号配额管理
    - 实现 `daily_quota` 检查：当日发布数达到上限时拒绝新发布请求并返回配额已满提示
    - _需求：8.2、8.3_

  - [x] 10.3 编写属性测试：发布配额约束性（P8）
    - **属性 P8：对任意账号，当日发布记录数达到 daily_quota 后，后续发布请求必须被拒绝，不得超额写入 publish_records**
    - **验证需求：8.3**
    - 在 `tests/test_pipeline.py` 中使用 Hypothesis 生成随机配额值和发布次数验证约束逻辑

- [x] 11. FastAPI 管理接口
  - [x] 11.1 实现人工审核接口
    - 在 `api/review.py` 中实现 `GET /tasks/pending_review`（返回 human_review 阶段任务列表）
    - 实现 `POST /review/decide`（接受 task_id、approved、comments）
    - approved=true 时更新状态为 publishing 并触发发布任务；approved=false 时更新为 rejected 并保留拒绝原因
    - task_id 不存在时返回 HTTP 404
    - _需求：7.1、7.2、7.3、7.4、7.5、7.6_

  - [x] 11.2 实现账号矩阵管理接口
    - 在 `api/accounts.py` 中实现账号增删改查接口，支持设置账号状态（active/inactive）
    - _需求：8.6_

  - [x] 11.3 实现数据分析看板接口
    - 在 `api/dashboard.py` 中实现各账号发布章节总数、累计字数、累计阅读量、累计收益的聚合查询
    - 支持按日/周/月时间范围、platform、agent_type 维度过滤
    - 响应时间不超过 3 秒
    - _需求：11.1、11.2、11.3、11.4_

  - [x] 11.4 实现全局输入校验与安全防护
    - 为所有 FastAPI 接口添加 Pydantic 参数校验，非法参数返回 HTTP 422 并附带字段级错误描述
    - 确保日志和接口响应中不暴露 API Key 或数据库连接字符串
    - _需求：13.4、13.5_

- [x] 12. Docker 部署配置
  - [x] 12.1 编写 `docker-compose.yml`
    - 包含 FastAPI 服务、Celery Worker、PostgreSQL、MongoDB、Redis 的完整编排配置
    - 配置服务健康检查（healthcheck）与依赖启动顺序
    - _需求：12.4、12.5_

  - [x] 12.2 编写 `Dockerfile` 与环境变量配置
    - 编写 FastAPI 服务与 Celery Worker 的 Dockerfile
    - 提供 `.env.example` 文件，列出所有必需的环境变量（API Key、数据库连接串等）
    - _需求：12.1、4.7_

---

## 备注

- 标注 `*` 的子任务为属性测试任务（可选，建议在对应实现任务完成后执行）
- 每个任务均引用具体需求条款，便于追溯
- 属性测试（P1-P8）使用 Hypothesis 框架，覆盖：
  - P1 分类器一致性、P2 模型路由完备性、P3 语料注入非空性、P4 流水线幂等性
  - P5 内容清洗无损性、P6 版权留存完整性、P7 哈希去重性、P8 发布配额约束性

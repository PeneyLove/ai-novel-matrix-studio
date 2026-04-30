-- ============================================================
-- AI小说矩阵工作室系统 — MySQL 数据库设计
-- 版本：1.0.0
-- 字符集：utf8mb4（支持中文及 emoji）
-- 说明：覆盖创作任务、账号矩阵、发布记录、版权留存、
--       爬虫任务、语料元数据、系统日志等全部业务模块
-- ============================================================

CREATE DATABASE IF NOT EXISTS ai_novel_studio
  DEFAULT CHARACTER SET utf8mb4
  DEFAULT COLLATE utf8mb4_unicode_ci;

USE ai_novel_studio;

-- ============================================================
-- 1. 账号矩阵表 accounts
--    管理各平台账号及每日发布配额（需求 8.2、8.6）
-- ============================================================
CREATE TABLE accounts (
    id           CHAR(36)     NOT NULL COMMENT '账号UUID',
    platform     VARCHAR(32)  NOT NULL COMMENT '平台标识: fanqie/qimao/zhihu/xiaohongshu/douyin',
    agent_type   VARCHAR(32)  NOT NULL COMMENT '绑定智能体: female_rebirth/male_power/suspense/romance',
    username     VARCHAR(128) NOT NULL COMMENT '平台用户名',
    display_name VARCHAR(128)          COMMENT '账号昵称/笔名',
    status       VARCHAR(16)  NOT NULL DEFAULT 'active' COMMENT '状态: active/inactive/banned',
    daily_quota  INT          NOT NULL DEFAULT 3        COMMENT '每日发布章节数上限（需求 8.2）',
    total_published INT       NOT NULL DEFAULT 0        COMMENT '累计发布章节数',
    created_at   DATETIME     NOT NULL DEFAULT CURRENT_TIMESTAMP,
    updated_at   DATETIME     NOT NULL DEFAULT CURRENT_TIMESTAMP ON UPDATE CURRENT_TIMESTAMP,
    PRIMARY KEY (id),
    INDEX idx_platform       (platform),
    INDEX idx_agent_type     (agent_type),
    INDEX idx_status         (status),
    CONSTRAINT chk_platform  CHECK (platform  IN ('fanqie','qimao','zhihu','xiaohongshu','douyin')),
    CONSTRAINT chk_agent     CHECK (agent_type IN ('female_rebirth','male_power','suspense','romance')),
    CONSTRAINT chk_status    CHECK (status    IN ('active','inactive','banned')),
    CONSTRAINT chk_quota     CHECK (daily_quota > 0)
) ENGINE=InnoDB DEFAULT CHARSET=utf8mb4 COLLATE=utf8mb4_unicode_ci
  COMMENT='账号矩阵管理表';


-- ============================================================
-- 2. 创作任务表 creation_tasks
--    记录每个创作任务的全生命周期（需求 6.5、6.6）
-- ============================================================
CREATE TABLE creation_tasks (
    id              CHAR(36)     NOT NULL COMMENT '任务UUID（幂等键，需求 6.3）',
    agent_type      VARCHAR(32)  NOT NULL COMMENT '使用的智能体类型',
    stage           VARCHAR(32)  NOT NULL DEFAULT 'pending'
                                          COMMENT '当前阶段: pending/topic_generating/outline_generating/content_generating/polishing/human_review/publishing/done/rejected',
    topic           TEXT                  COMMENT '生成的选题内容',
    outline         JSON                  COMMENT '分卷大纲（JSON结构）',
    word_count      INT          NOT NULL DEFAULT 0 COMMENT '当前累计字数',
    retry_count     TINYINT      NOT NULL DEFAULT 0 COMMENT '当前阶段重试次数',
    reject_reason   TEXT                  COMMENT '审核拒绝原因（需求 7.6）',
    trend_data      TEXT                  COMMENT '触发本任务的热榜数据快照',
    topic_at        DATETIME              COMMENT '选题完成时间',
    outline_at      DATETIME              COMMENT '大纲完成时间',
    content_at      DATETIME              COMMENT '正文完成时间',
    polish_at       DATETIME              COMMENT '润色完成时间',
    review_at       DATETIME              COMMENT '进入审核时间',
    publish_at      DATETIME              COMMENT '发布完成时间',
    created_at      DATETIME     NOT NULL DEFAULT CURRENT_TIMESTAMP,
    updated_at      DATETIME     NOT NULL DEFAULT CURRENT_TIMESTAMP ON UPDATE CURRENT_TIMESTAMP,
    PRIMARY KEY (id),
    INDEX idx_stage      (stage),
    INDEX idx_agent_type (agent_type),
    INDEX idx_created_at (created_at),
    CONSTRAINT chk_task_stage CHECK (stage IN (
        'pending','topic_generating','outline_generating',
        'content_generating','polishing','human_review',
        'publishing','done','rejected'
    )),
    CONSTRAINT chk_task_agent CHECK (agent_type IN ('female_rebirth','male_power','suspense','romance'))
) ENGINE=InnoDB DEFAULT CHARSET=utf8mb4 COLLATE=utf8mb4_unicode_ci
  COMMENT='创作任务主表';


-- ============================================================
-- 3. 任务状态历史表 task_stage_history
--    记录任务每次状态变更（需求 6.5）
-- ============================================================
CREATE TABLE task_stage_history (
    id          BIGINT       NOT NULL AUTO_INCREMENT,
    task_id     CHAR(36)     NOT NULL COMMENT '关联任务ID',
    from_stage  VARCHAR(32)           COMMENT '变更前阶段',
    to_stage    VARCHAR(32)  NOT NULL COMMENT '变更后阶段',
    operator    VARCHAR(64)           COMMENT '操作人（system/用户名）',
    remark      VARCHAR(512)          COMMENT '备注',
    created_at  DATETIME     NOT NULL DEFAULT CURRENT_TIMESTAMP,
    PRIMARY KEY (id),
    INDEX idx_task_id   (task_id),
    INDEX idx_created_at (created_at),
    CONSTRAINT fk_history_task FOREIGN KEY (task_id)
        REFERENCES creation_tasks(id) ON DELETE CASCADE
) ENGINE=InnoDB DEFAULT CHARSET=utf8mb4 COLLATE=utf8mb4_unicode_ci
  COMMENT='任务状态变更历史';


-- ============================================================
-- 4. 章节内容表 chapters
--    存储每章的初稿、润色稿、定稿（需求 9.1-9.4）
-- ============================================================
CREATE TABLE chapters (
    id               CHAR(36)    NOT NULL COMMENT '章节UUID',
    task_id          CHAR(36)    NOT NULL COMMENT '所属任务ID',
    chapter_no       SMALLINT    NOT NULL COMMENT '章节序号（从1开始）',
    chapter_title    VARCHAR(256)         COMMENT '章节标题',
    raw_content      MEDIUMTEXT           COMMENT 'AI初稿正文',
    polished_content MEDIUMTEXT           COMMENT '润色后正文',
    final_content    MEDIUMTEXT           COMMENT '人工定稿正文',
    word_count       INT         NOT NULL DEFAULT 0 COMMENT '定稿字数',
    status           VARCHAR(16) NOT NULL DEFAULT 'draft'
                                          COMMENT '状态: draft/polished/finalized/published',
    created_at       DATETIME    NOT NULL DEFAULT CURRENT_TIMESTAMP,
    updated_at       DATETIME    NOT NULL DEFAULT CURRENT_TIMESTAMP ON UPDATE CURRENT_TIMESTAMP,
    PRIMARY KEY (id),
    UNIQUE KEY uk_task_chapter (task_id, chapter_no),
    INDEX idx_task_id (task_id),
    INDEX idx_status  (status),
    CONSTRAINT fk_chapter_task FOREIGN KEY (task_id)
        REFERENCES creation_tasks(id) ON DELETE CASCADE,
    CONSTRAINT chk_chapter_status CHECK (status IN ('draft','polished','finalized','published'))
) ENGINE=InnoDB DEFAULT CHARSET=utf8mb4 COLLATE=utf8mb4_unicode_ci
  COMMENT='章节内容表';


-- ============================================================
-- 5. 版权留存表 copyright_traces
--    记录提示词哈希、初稿哈希、定稿哈希（需求 9.1-9.6，属性 P6）
-- ============================================================
CREATE TABLE copyright_traces (
    id           CHAR(36)    NOT NULL COMMENT '记录UUID',
    task_id      CHAR(36)    NOT NULL COMMENT '关联任务ID',
    chapter_id   CHAR(36)             COMMENT '关联章节ID（可为空，任务级留存时为空）',
    prompt_hash  CHAR(32)    NOT NULL COMMENT '提示词MD5（需求 9.2）',
    draft_hash   CHAR(32)    NOT NULL COMMENT 'AI初稿MD5（需求 9.2）',
    final_hash   CHAR(32)             COMMENT '定稿MD5（人工修改后更新，需求 9.3）',
    prompt_text  TEXT                 COMMENT '提示词原文（可选，用于人工核查）',
    trace_time   DATETIME    NOT NULL DEFAULT CURRENT_TIMESTAMP COMMENT '留存时间戳（不可修改，需求 9.5）',
    PRIMARY KEY (id),
    INDEX idx_task_id    (task_id),
    INDEX idx_chapter_id (chapter_id),
    INDEX idx_trace_time (trace_time),
    CONSTRAINT fk_trace_task    FOREIGN KEY (task_id)
        REFERENCES creation_tasks(id) ON DELETE RESTRICT,
    CONSTRAINT fk_trace_chapter FOREIGN KEY (chapter_id)
        REFERENCES chapters(id) ON DELETE SET NULL,
    -- prompt_hash 和 draft_hash 不允许为空（需求 9.2）
    CONSTRAINT chk_prompt_hash CHECK (prompt_hash IS NOT NULL AND prompt_hash != ''),
    CONSTRAINT chk_draft_hash  CHECK (draft_hash  IS NOT NULL AND draft_hash  != '')
) ENGINE=InnoDB DEFAULT CHARSET=utf8mb4 COLLATE=utf8mb4_unicode_ci
  COMMENT='版权合规留存表';


-- ============================================================
-- 6. 发布记录表 publish_records
--    记录每次发布操作及数据（需求 8.4、8.5、11.1）
-- ============================================================
CREATE TABLE publish_records (
    id           CHAR(36)       NOT NULL COMMENT '发布记录UUID',
    task_id      CHAR(36)       NOT NULL COMMENT '关联任务ID',
    chapter_id   CHAR(36)                COMMENT '关联章节ID',
    account_id   CHAR(36)       NOT NULL COMMENT '发布账号ID',
    platform     VARCHAR(32)    NOT NULL COMMENT '发布平台',
    chapter_no   SMALLINT       NOT NULL COMMENT '章节序号',
    chapter_title VARCHAR(256)           COMMENT '发布时的章节标题',
    word_count   INT            NOT NULL DEFAULT 0 COMMENT '发布字数',
    published_at DATETIME       NOT NULL DEFAULT CURRENT_TIMESTAMP COMMENT '发布时间',
    read_count   INT            NOT NULL DEFAULT 0 COMMENT '阅读量（定期同步）',
    collect_count INT           NOT NULL DEFAULT 0 COMMENT '收藏量',
    comment_count INT           NOT NULL DEFAULT 0 COMMENT '评论数',
    revenue      DECIMAL(10,2)  NOT NULL DEFAULT 0.00 COMMENT '收益（元）',
    data_synced_at DATETIME              COMMENT '数据最后同步时间',
    PRIMARY KEY (id),
    INDEX idx_task_id      (task_id),
    INDEX idx_account_id   (account_id),
    INDEX idx_platform     (platform),
    INDEX idx_published_at (published_at),
    INDEX idx_account_date (account_id, published_at),  -- 用于配额查询（需求 8.3，属性 P8）
    CONSTRAINT fk_pub_task    FOREIGN KEY (task_id)
        REFERENCES creation_tasks(id) ON DELETE RESTRICT,
    CONSTRAINT fk_pub_chapter FOREIGN KEY (chapter_id)
        REFERENCES chapters(id) ON DELETE SET NULL,
    CONSTRAINT fk_pub_account FOREIGN KEY (account_id)
        REFERENCES accounts(id) ON DELETE RESTRICT,
    CONSTRAINT chk_pub_platform CHECK (platform IN ('fanqie','qimao','zhihu','xiaohongshu','douyin'))
) ENGINE=InnoDB DEFAULT CHARSET=utf8mb4 COLLATE=utf8mb4_unicode_ci
  COMMENT='发布记录表';


-- ============================================================
-- 7. 爬虫任务表 crawl_jobs
--    记录每次爬虫调度执行情况（需求 1.6）
-- ============================================================
CREATE TABLE crawl_jobs (
    id            BIGINT       NOT NULL AUTO_INCREMENT,
    spider_name   VARCHAR(64)  NOT NULL COMMENT '爬虫名称: fanqie/qimao/zhihu',
    target_url    VARCHAR(512)          COMMENT '本次爬取目标URL',
    status        VARCHAR(16)  NOT NULL DEFAULT 'pending'
                                        COMMENT '状态: pending/running/success/failed/skipped',
    items_crawled INT          NOT NULL DEFAULT 0 COMMENT '本次采集条数',
    items_skipped INT          NOT NULL DEFAULT 0 COMMENT '本次跳过（重复）条数',
    error_msg     TEXT                  COMMENT '失败原因',
    started_at    DATETIME              COMMENT '开始时间',
    finished_at   DATETIME              COMMENT '完成时间',
    created_at    DATETIME     NOT NULL DEFAULT CURRENT_TIMESTAMP,
    PRIMARY KEY (id),
    INDEX idx_spider_name (spider_name),
    INDEX idx_status      (status),
    INDEX idx_created_at  (created_at),
    CONSTRAINT chk_crawl_status CHECK (status IN ('pending','running','success','failed','skipped'))
) ENGINE=InnoDB DEFAULT CHARSET=utf8mb4 COLLATE=utf8mb4_unicode_ci
  COMMENT='爬虫调度任务记录表';


-- ============================================================
-- 8. 语料元数据表 corpus_meta
--    MySQL侧记录语料的结构化元信息，正文存MongoDB（需求 10.1-10.5）
-- ============================================================
CREATE TABLE corpus_meta (
    id             CHAR(36)      NOT NULL COMMENT '语料UUID（与MongoDB _id对应）',
    mongo_id       VARCHAR(64)            COMMENT 'MongoDB ObjectId字符串',
    source         VARCHAR(32)   NOT NULL COMMENT '来源: fanqie/qimao/zhihu/qidian',
    category       VARCHAR(32)   NOT NULL COMMENT '题材分类',
    corpus_type    VARCHAR(16)   NOT NULL DEFAULT 'raw'
                                          COMMENT '语料类型: raw（原始）/training（训练）',
    book_title     VARCHAR(256)           COMMENT '书名',
    chapter_title  VARCHAR(256)           COMMENT '章节标题',
    word_count     INT           NOT NULL DEFAULT 0 COMMENT '字数',
    quality_score  DECIMAL(4,3)  NOT NULL DEFAULT 0.000 COMMENT '质量评分 0.000-1.000（需求 10.3）',
    content_hash   CHAR(32)      NOT NULL COMMENT '内容MD5（去重用，需求 10.4，属性 P7）',
    is_valid       TINYINT(1)    NOT NULL DEFAULT 1 COMMENT '是否有效（清洗通过）',
    is_copyright_suspect TINYINT(1) NOT NULL DEFAULT 0 COMMENT '是否疑似侵权（需求 13.6）',
    crawl_job_id   BIGINT                 COMMENT '关联爬虫任务ID',
    crawl_time     DATETIME      NOT NULL DEFAULT CURRENT_TIMESTAMP COMMENT '采集时间',
    selected_at    DATETIME               COMMENT '入训练集时间',
    PRIMARY KEY (id),
    UNIQUE KEY uk_content_hash (content_hash),   -- 全局去重（属性 P7）
    INDEX idx_category      (category),
    INDEX idx_corpus_type   (corpus_type),
    INDEX idx_quality_score (quality_score),
    INDEX idx_crawl_time    (crawl_time),
    INDEX idx_is_valid      (is_valid),
    INDEX idx_copyright     (is_copyright_suspect),
    CONSTRAINT chk_corpus_category CHECK (category IN ('female_rebirth','male_power','suspense','romance')),
    CONSTRAINT chk_corpus_type     CHECK (corpus_type IN ('raw','training')),
    CONSTRAINT chk_quality_range   CHECK (quality_score BETWEEN 0.000 AND 1.000)
) ENGINE=InnoDB DEFAULT CHARSET=utf8mb4 COLLATE=utf8mb4_unicode_ci
  COMMENT='语料元数据表（正文存MongoDB）';


-- ============================================================
-- 9. 模型调用日志表 model_call_logs
--    记录每次AI模型API调用（用于成本统计与故障排查）
-- ============================================================
CREATE TABLE model_call_logs (
    id             BIGINT        NOT NULL AUTO_INCREMENT,
    task_id        CHAR(36)               COMMENT '关联任务ID',
    provider       VARCHAR(32)   NOT NULL COMMENT '模型提供商: minimax/doubao/qwen/deepseek',
    model_name     VARCHAR(64)   NOT NULL COMMENT '具体模型名称',
    stage          VARCHAR(32)   NOT NULL COMMENT '创作环节',
    prompt_tokens  INT           NOT NULL DEFAULT 0 COMMENT '输入token数',
    output_tokens  INT           NOT NULL DEFAULT 0 COMMENT '输出token数',
    cost_yuan      DECIMAL(8,4)  NOT NULL DEFAULT 0.0000 COMMENT '本次调用费用（元）',
    latency_ms     INT           NOT NULL DEFAULT 0 COMMENT '响应耗时（毫秒）',
    status         VARCHAR(16)   NOT NULL DEFAULT 'success'
                                          COMMENT '状态: success/failed/rate_limited/timeout',
    retry_attempt  TINYINT       NOT NULL DEFAULT 0 COMMENT '重试次数',
    error_code     VARCHAR(32)            COMMENT '错误码',
    called_at      DATETIME      NOT NULL DEFAULT CURRENT_TIMESTAMP,
    PRIMARY KEY (id),
    INDEX idx_task_id   (task_id),
    INDEX idx_provider  (provider),
    INDEX idx_stage     (stage),
    INDEX idx_status    (status),
    INDEX idx_called_at (called_at),
    CONSTRAINT chk_log_provider CHECK (provider IN ('minimax','doubao','qwen','deepseek')),
    CONSTRAINT chk_log_status   CHECK (status IN ('success','failed','rate_limited','timeout'))
) ENGINE=InnoDB DEFAULT CHARSET=utf8mb4 COLLATE=utf8mb4_unicode_ci
  COMMENT='AI模型调用日志表';


-- ============================================================
-- 10. 系统告警日志表 system_alerts
--     记录版权留存失败、服务不可用等告警（需求 9.6、12.5）
-- ============================================================
CREATE TABLE system_alerts (
    id          BIGINT       NOT NULL AUTO_INCREMENT,
    alert_type  VARCHAR(64)  NOT NULL COMMENT '告警类型: copyright_trace_fail/service_unavailable/crawl_error/quota_exceeded',
    severity    VARCHAR(16)  NOT NULL DEFAULT 'warning' COMMENT '严重级别: info/warning/error/critical',
    task_id     CHAR(36)              COMMENT '关联任务ID（可为空）',
    message     TEXT         NOT NULL COMMENT '告警详情',
    resolved    TINYINT(1)   NOT NULL DEFAULT 0 COMMENT '是否已处理',
    resolved_at DATETIME              COMMENT '处理时间',
    created_at  DATETIME     NOT NULL DEFAULT CURRENT_TIMESTAMP,
    PRIMARY KEY (id),
    INDEX idx_alert_type (alert_type),
    INDEX idx_severity   (severity),
    INDEX idx_resolved   (resolved),
    INDEX idx_created_at (created_at),
    CONSTRAINT chk_severity CHECK (severity IN ('info','warning','error','critical'))
) ENGINE=InnoDB DEFAULT CHARSET=utf8mb4 COLLATE=utf8mb4_unicode_ci
  COMMENT='系统告警日志表';


-- ============================================================
-- 11. 数据看板聚合缓存表 dashboard_stats
--     按账号+日期预聚合，加速看板查询（需求 11.1-11.4）
-- ============================================================
CREATE TABLE dashboard_stats (
    id              BIGINT         NOT NULL AUTO_INCREMENT,
    account_id      CHAR(36)       NOT NULL COMMENT '账号ID',
    stat_date       DATE           NOT NULL COMMENT '统计日期',
    platform        VARCHAR(32)    NOT NULL COMMENT '平台',
    agent_type      VARCHAR(32)    NOT NULL COMMENT '智能体类型',
    chapters_count  INT            NOT NULL DEFAULT 0 COMMENT '当日发布章节数',
    total_words     INT            NOT NULL DEFAULT 0 COMMENT '当日发布总字数',
    total_reads     INT            NOT NULL DEFAULT 0 COMMENT '当日累计阅读量',
    total_collects  INT            NOT NULL DEFAULT 0 COMMENT '当日累计收藏量',
    total_revenue   DECIMAL(10,2)  NOT NULL DEFAULT 0.00 COMMENT '当日收益（元）',
    updated_at      DATETIME       NOT NULL DEFAULT CURRENT_TIMESTAMP ON UPDATE CURRENT_TIMESTAMP,
    PRIMARY KEY (id),
    UNIQUE KEY uk_account_date_platform (account_id, stat_date, platform),
    INDEX idx_stat_date  (stat_date),
    INDEX idx_agent_type (agent_type),
    CONSTRAINT fk_stats_account FOREIGN KEY (account_id)
        REFERENCES accounts(id) ON DELETE CASCADE
) ENGINE=InnoDB DEFAULT CHARSET=utf8mb4 COLLATE=utf8mb4_unicode_ci
  COMMENT='看板数据聚合缓存表（按账号+日期）';


-- ============================================================
-- 12. 初始化数据：默认账号示例
-- ============================================================
INSERT INTO accounts (id, platform, agent_type, username, display_name, status, daily_quota)
VALUES
    (UUID(), 'fanqie',  'female_rebirth', 'fanqie_account_01', '重生大佬驾到',  'active', 3),
    (UUID(), 'qimao',   'male_power',     'qimao_account_01',  '都市无敌系统',  'active', 3),
    (UUID(), 'zhihu',   'suspense',       'zhihu_account_01',  '悬疑推理社',    'active', 2),
    (UUID(), 'fanqie',  'romance',        'fanqie_account_02', '甜宠日常',      'active', 3);


-- ============================================================
-- 13. 视图：待审核任务视图
--     方便 ReviewAPI 查询（需求 7.1）
-- ============================================================
CREATE OR REPLACE VIEW v_pending_review AS
SELECT
    ct.id           AS task_id,
    ct.agent_type,
    ct.topic,
    ct.word_count,
    ct.polish_at,
    ct.review_at,
    COUNT(c.id)     AS chapter_count
FROM creation_tasks ct
LEFT JOIN chapters c ON c.task_id = ct.id AND c.status = 'polished'
WHERE ct.stage = 'human_review'
GROUP BY ct.id, ct.agent_type, ct.topic, ct.word_count, ct.polish_at, ct.review_at
ORDER BY ct.review_at ASC;


-- ============================================================
-- 14. 视图：账号今日发布配额视图
--     用于 Publisher 配额检查（需求 8.3，属性 P8）
-- ============================================================
CREATE OR REPLACE VIEW v_account_daily_quota AS
SELECT
    a.id            AS account_id,
    a.platform,
    a.agent_type,
    a.daily_quota,
    a.status,
    COALESCE(today.published_today, 0)          AS published_today,
    a.daily_quota - COALESCE(today.published_today, 0) AS remaining_quota
FROM accounts a
LEFT JOIN (
    SELECT
        account_id,
        COUNT(*) AS published_today
    FROM publish_records
    WHERE DATE(published_at) = CURDATE()
    GROUP BY account_id
) today ON today.account_id = a.id;


-- ============================================================
-- 15. 视图：语料库统计视图
--     支持按题材、质量、时间查询（需求 10.5）
-- ============================================================
CREATE OR REPLACE VIEW v_corpus_stats AS
SELECT
    category,
    corpus_type,
    COUNT(*)                                    AS total_count,
    SUM(CASE WHEN is_valid = 1 THEN 1 ELSE 0 END) AS valid_count,
    SUM(CASE WHEN quality_score >= 0.8 THEN 1 ELSE 0 END) AS high_quality_count,
    ROUND(AVG(quality_score), 3)                AS avg_quality,
    SUM(word_count)                             AS total_words,
    MAX(crawl_time)                             AS latest_crawl
FROM corpus_meta
GROUP BY category, corpus_type;


-- ============================================================
-- 16. 视图：模型调用成本统计视图
-- ============================================================
CREATE OR REPLACE VIEW v_model_cost_stats AS
SELECT
    provider,
    stage,
    DATE(called_at)                             AS stat_date,
    COUNT(*)                                    AS call_count,
    SUM(CASE WHEN status = 'success' THEN 1 ELSE 0 END) AS success_count,
    SUM(CASE WHEN status = 'failed'  THEN 1 ELSE 0 END) AS fail_count,
    SUM(prompt_tokens + output_tokens)          AS total_tokens,
    ROUND(SUM(cost_yuan), 4)                    AS total_cost_yuan,
    ROUND(AVG(latency_ms), 0)                   AS avg_latency_ms
FROM model_call_logs
GROUP BY provider, stage, DATE(called_at);


-- ============================================================
-- 17. 大纲表 outlines
--     存储 AI 生成的大纲及其审核状态（需求 9.1、9.5）
-- ============================================================
CREATE TABLE outlines (
    id              CHAR(36)     NOT NULL COMMENT '大纲UUID',
    agent_type      VARCHAR(32)  NOT NULL COMMENT '生成智能体类型',
    batch_id        CHAR(36)     NOT NULL COMMENT '批次ID（同一批次生成的大纲共享）',
    title           VARCHAR(256)          COMMENT '大纲标题',
    content         MEDIUMTEXT   NOT NULL COMMENT '大纲正文（JSON或纯文本）',
    topic_hint      TEXT                  COMMENT '生成时的主题提示',
    trend_data      TEXT                  COMMENT '生成时的热榜数据快照',
    status          VARCHAR(32)  NOT NULL DEFAULT 'pending_review'
                                          COMMENT '状态: pending_review/approved/rejected/in_use/used',
    reviewer        VARCHAR(64)           COMMENT '审核人',
    review_comments TEXT                  COMMENT '审核意见',
    reject_reason   TEXT                  COMMENT '拒绝原因',
    reviewed_at     DATETIME              COMMENT '审核时间',
    novel_id        CHAR(36)              COMMENT '关联的小说任务ID（in_use时填写）',
    created_at      DATETIME     NOT NULL DEFAULT CURRENT_TIMESTAMP,
    updated_at      DATETIME     NOT NULL DEFAULT CURRENT_TIMESTAMP ON UPDATE CURRENT_TIMESTAMP,
    PRIMARY KEY (id),
    INDEX idx_status      (status),
    INDEX idx_agent_type  (agent_type),
    INDEX idx_batch_id    (batch_id),
    INDEX idx_created_at  (created_at),
    CONSTRAINT chk_outline_status CHECK (status IN (
        'pending_review', 'approved', 'rejected', 'in_use', 'used'
    )),
    CONSTRAINT chk_outline_agent CHECK (agent_type IN (
        'female_rebirth', 'male_power', 'suspense', 'romance'
    ))
) ENGINE=InnoDB DEFAULT CHARSET=utf8mb4 COLLATE=utf8mb4_unicode_ci COMMENT='大纲表';


-- ============================================================
-- 18. 小说任务表 novels
--     记录每个小说任务的全生命周期（需求 9.2、9.6）
-- ============================================================
CREATE TABLE novels (
    id                  CHAR(36)     NOT NULL COMMENT '小说任务UUID',
    outline_id          CHAR(36)     NOT NULL COMMENT '关联大纲ID',
    agent_type          VARCHAR(32)  NOT NULL COMMENT '编写智能体类型',
    title               VARCHAR(256)          COMMENT '小说标题',
    status              VARCHAR(32)  NOT NULL DEFAULT 'writing'
                                              COMMENT '状态: writing/novel_pending_review/novel_approved/novel_rejected/revising/publishing/done',
    word_count          INT          NOT NULL DEFAULT 0 COMMENT '当前字数',
    revision_round      TINYINT      NOT NULL DEFAULT 0 COMMENT '修改轮次',
    reviewer            VARCHAR(64)           COMMENT '审核人',
    review_comments     TEXT                  COMMENT '审核意见',
    revision_instructions TEXT                COMMENT '最新修改指令',
    reject_reason       TEXT                  COMMENT '拒绝原因',
    reviewed_at         DATETIME              COMMENT '最近审核时间',
    writing_started_at  DATETIME              COMMENT '开始编写时间',
    writing_finished_at DATETIME              COMMENT '编写完成时间',
    created_at          DATETIME     NOT NULL DEFAULT CURRENT_TIMESTAMP,
    updated_at          DATETIME     NOT NULL DEFAULT CURRENT_TIMESTAMP ON UPDATE CURRENT_TIMESTAMP,
    PRIMARY KEY (id),
    INDEX idx_outline_id  (outline_id),
    INDEX idx_status      (status),
    INDEX idx_agent_type  (agent_type),
    INDEX idx_created_at  (created_at),
    CONSTRAINT chk_novel_status CHECK (status IN (
        'writing', 'novel_pending_review', 'novel_approved',
        'novel_rejected', 'revising', 'publishing', 'done'
    )),
    CONSTRAINT chk_novel_agent CHECK (agent_type IN (
        'female_rebirth', 'male_power', 'suspense', 'romance'
    )),
    CONSTRAINT fk_novel_outline FOREIGN KEY (outline_id)
        REFERENCES outlines(id) ON DELETE RESTRICT
) ENGINE=InnoDB DEFAULT CHARSET=utf8mb4 COLLATE=utf8mb4_unicode_ci COMMENT='小说任务表';


-- ============================================================
-- 19. 小说章节表 novel_chapters
--     存储每部小说的章节内容，(novel_id, chapter_no) 唯一（需求 9.1）
-- ============================================================
CREATE TABLE novel_chapters (
    id               CHAR(36)     NOT NULL COMMENT '章节UUID',
    novel_id         CHAR(36)     NOT NULL COMMENT '所属小说ID',
    chapter_no       SMALLINT     NOT NULL COMMENT '章节序号（从1开始）',
    chapter_title    VARCHAR(256)          COMMENT '章节标题',
    content          MEDIUMTEXT            COMMENT '当前章节内容（最新版本）',
    word_count       INT          NOT NULL DEFAULT 0,
    status           VARCHAR(16)  NOT NULL DEFAULT 'draft'
                                           COMMENT '状态: draft/finalized',
    created_at       DATETIME     NOT NULL DEFAULT CURRENT_TIMESTAMP,
    updated_at       DATETIME     NOT NULL DEFAULT CURRENT_TIMESTAMP ON UPDATE CURRENT_TIMESTAMP,
    PRIMARY KEY (id),
    UNIQUE KEY uk_novel_chapter (novel_id, chapter_no),
    INDEX idx_novel_id (novel_id),
    CONSTRAINT fk_novel_chapter FOREIGN KEY (novel_id)
        REFERENCES novels(id) ON DELETE CASCADE,
    CONSTRAINT chk_novel_chapter_status CHECK (status IN ('draft', 'finalized'))
) ENGINE=InnoDB DEFAULT CHARSET=utf8mb4 COLLATE=utf8mb4_unicode_ci COMMENT='小说章节表';


-- ============================================================
-- 20. 小说修改历史表 novel_revision_history
--     记录每轮修改的指令和修改前内容快照（需求 9.1）
-- ============================================================
CREATE TABLE novel_revision_history (
    id                    BIGINT       NOT NULL AUTO_INCREMENT,
    novel_id              CHAR(36)     NOT NULL COMMENT '关联小说ID',
    revision_round        TINYINT      NOT NULL COMMENT '修改轮次',
    revision_instructions TEXT         NOT NULL COMMENT '修改指令',
    reviewer              VARCHAR(64)           COMMENT '提出修改意见的审核人',
    content_snapshot      LONGTEXT              COMMENT '修改前内容快照（JSON格式，按章节存储）',
    created_at            DATETIME     NOT NULL DEFAULT CURRENT_TIMESTAMP,
    PRIMARY KEY (id),
    INDEX idx_novel_id (novel_id),
    CONSTRAINT fk_revision_novel FOREIGN KEY (novel_id)
        REFERENCES novels(id) ON DELETE CASCADE
) ENGINE=InnoDB DEFAULT CHARSET=utf8mb4 COLLATE=utf8mb4_unicode_ci COMMENT='小说修改历史表';


-- ============================================================
-- 21. 大纲审核历史表 outline_review_history
--     记录大纲每次状态变更（需求 9.1）
-- ============================================================
CREATE TABLE outline_review_history (
    id          BIGINT       NOT NULL AUTO_INCREMENT,
    outline_id  CHAR(36)     NOT NULL COMMENT '关联大纲ID',
    from_status VARCHAR(32)           COMMENT '变更前状态',
    to_status   VARCHAR(32)  NOT NULL COMMENT '变更后状态',
    operator    VARCHAR(64)           COMMENT '操作人',
    remark      TEXT                  COMMENT '备注/审核意见',
    created_at  DATETIME     NOT NULL DEFAULT CURRENT_TIMESTAMP,
    PRIMARY KEY (id),
    INDEX idx_outline_id (outline_id),
    CONSTRAINT fk_outline_history FOREIGN KEY (outline_id)
        REFERENCES outlines(id) ON DELETE CASCADE
) ENGINE=InnoDB DEFAULT CHARSET=utf8mb4 COLLATE=utf8mb4_unicode_ci COMMENT='大纲审核历史表';

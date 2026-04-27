"""
MySQL ORM 模型 — 使用 SQLAlchemy 2.x async 风格
字段与 schema.sql 完全一致（MySQL 版本）
"""
import os
from datetime import datetime, date
from decimal import Decimal
from typing import Optional

from sqlalchemy import (
    BigInteger, Boolean, CHAR, CheckConstraint, Column, Date,
    DateTime, ForeignKey, Index, Integer, JSON, Numeric,
    SmallInteger, String, Text, UniqueConstraint,
)
from sqlalchemy.ext.asyncio import AsyncSession, async_sessionmaker, create_async_engine
from sqlalchemy.orm import DeclarativeBase, relationship

from ai_novel_studio.config.settings import settings

# ---------------------------------------------------------------------------
# 异步引擎与 Session 工厂（从 settings 读取连接 URL）
# ---------------------------------------------------------------------------
engine = create_async_engine(
    settings.mysql_url,
    echo=False,
    pool_pre_ping=True,
    pool_recycle=3600,
)

AsyncSessionLocal: async_sessionmaker[AsyncSession] = async_sessionmaker(
    bind=engine,
    class_=AsyncSession,
    expire_on_commit=False,
)


# ---------------------------------------------------------------------------
# 声明基类
# ---------------------------------------------------------------------------
class Base(DeclarativeBase):
    pass


# ---------------------------------------------------------------------------
# 1. accounts — 账号矩阵表
# ---------------------------------------------------------------------------
class Account(Base):
    __tablename__ = "accounts"

    id           = Column(CHAR(36),     primary_key=True, comment="账号UUID")
    platform     = Column(String(32),   nullable=False,   comment="平台标识")
    agent_type   = Column(String(32),   nullable=False,   comment="绑定智能体")
    username     = Column(String(128),  nullable=False,   comment="平台用户名")
    display_name = Column(String(128),  nullable=True,    comment="账号昵称/笔名")
    status       = Column(String(16),   nullable=False,   default="active",  comment="状态")
    daily_quota  = Column(Integer,      nullable=False,   default=3,         comment="每日发布章节数上限")
    total_published = Column(Integer,   nullable=False,   default=0,         comment="累计发布章节数")
    created_at   = Column(DateTime,     nullable=False,   default=datetime.utcnow)
    updated_at   = Column(DateTime,     nullable=False,   default=datetime.utcnow, onupdate=datetime.utcnow)

    __table_args__ = (
        Index("idx_platform",   "platform"),
        Index("idx_agent_type", "agent_type"),
        Index("idx_status",     "status"),
        CheckConstraint("platform IN ('fanqie','qimao','zhihu','xiaohongshu','douyin')", name="chk_platform"),
        CheckConstraint("agent_type IN ('female_rebirth','male_power','suspense','romance')", name="chk_agent"),
        CheckConstraint("status IN ('active','inactive','banned')", name="chk_status"),
        CheckConstraint("daily_quota > 0", name="chk_quota"),
    )


# ---------------------------------------------------------------------------
# 2. creation_tasks — 创作任务表
# ---------------------------------------------------------------------------
class CreationTask(Base):
    __tablename__ = "creation_tasks"

    id            = Column(CHAR(36),    primary_key=True, comment="任务UUID（幂等键）")
    agent_type    = Column(String(32),  nullable=False,   comment="使用的智能体类型")
    stage         = Column(String(32),  nullable=False,   default="pending", comment="当前阶段")
    topic         = Column(Text,        nullable=True,    comment="生成的选题内容")
    outline       = Column(JSON,        nullable=True,    comment="分卷大纲")
    word_count    = Column(Integer,     nullable=False,   default=0)
    retry_count   = Column(SmallInteger, nullable=False,  default=0)
    reject_reason = Column(Text,        nullable=True,    comment="审核拒绝原因")
    trend_data    = Column(Text,        nullable=True,    comment="热榜数据快照")
    topic_at      = Column(DateTime,    nullable=True)
    outline_at    = Column(DateTime,    nullable=True)
    content_at    = Column(DateTime,    nullable=True)
    polish_at     = Column(DateTime,    nullable=True)
    review_at     = Column(DateTime,    nullable=True)
    publish_at    = Column(DateTime,    nullable=True)
    created_at    = Column(DateTime,    nullable=False,   default=datetime.utcnow)
    updated_at    = Column(DateTime,    nullable=False,   default=datetime.utcnow, onupdate=datetime.utcnow)

    __table_args__ = (
        Index("idx_stage",      "stage"),
        Index("idx_agent_type", "agent_type"),
        Index("idx_created_at", "created_at"),
        CheckConstraint(
            "stage IN ('pending','topic_generating','outline_generating',"
            "'content_generating','polishing','human_review','publishing','done','rejected')",
            name="chk_task_stage",
        ),
        CheckConstraint(
            "agent_type IN ('female_rebirth','male_power','suspense','romance')",
            name="chk_task_agent",
        ),
    )


# ---------------------------------------------------------------------------
# 3. task_stage_history — 任务状态历史表
# ---------------------------------------------------------------------------
class TaskStageHistory(Base):
    __tablename__ = "task_stage_history"

    id         = Column(BigInteger,  primary_key=True, autoincrement=True)
    task_id    = Column(CHAR(36),    ForeignKey("creation_tasks.id", ondelete="CASCADE"), nullable=False)
    from_stage = Column(String(32),  nullable=True,  comment="变更前阶段")
    to_stage   = Column(String(32),  nullable=False, comment="变更后阶段")
    operator   = Column(String(64),  nullable=True,  comment="操作人")
    remark     = Column(String(512), nullable=True,  comment="备注")
    created_at = Column(DateTime,    nullable=False,  default=datetime.utcnow)

    __table_args__ = (
        Index("idx_history_task_id",   "task_id"),
        Index("idx_history_created_at", "created_at"),
    )


# ---------------------------------------------------------------------------
# 4. chapters — 章节内容表
# ---------------------------------------------------------------------------
class Chapter(Base):
    __tablename__ = "chapters"

    id               = Column(CHAR(36),    primary_key=True, comment="章节UUID")
    task_id          = Column(CHAR(36),    ForeignKey("creation_tasks.id", ondelete="CASCADE"), nullable=False)
    chapter_no       = Column(SmallInteger, nullable=False, comment="章节序号（从1开始）")
    chapter_title    = Column(String(256), nullable=True)
    raw_content      = Column(Text,        nullable=True,  comment="AI初稿正文")
    polished_content = Column(Text,        nullable=True,  comment="润色后正文")
    final_content    = Column(Text,        nullable=True,  comment="人工定稿正文")
    word_count       = Column(Integer,     nullable=False, default=0)
    status           = Column(String(16),  nullable=False, default="draft")
    created_at       = Column(DateTime,    nullable=False, default=datetime.utcnow)
    updated_at       = Column(DateTime,    nullable=False, default=datetime.utcnow, onupdate=datetime.utcnow)

    __table_args__ = (
        UniqueConstraint("task_id", "chapter_no", name="uk_task_chapter"),
        Index("idx_chapter_task_id", "task_id"),
        Index("idx_chapter_status",  "status"),
        CheckConstraint("status IN ('draft','polished','finalized','published')", name="chk_chapter_status"),
    )


# ---------------------------------------------------------------------------
# 5. copyright_traces — 版权留存表
# ---------------------------------------------------------------------------
class CopyrightTrace(Base):
    __tablename__ = "copyright_traces"

    id          = Column(CHAR(36),  primary_key=True, comment="记录UUID")
    task_id     = Column(CHAR(36),  ForeignKey("creation_tasks.id", ondelete="RESTRICT"), nullable=False)
    chapter_id  = Column(CHAR(36),  ForeignKey("chapters.id", ondelete="SET NULL"), nullable=True)
    prompt_hash = Column(CHAR(32),  nullable=False, comment="提示词MD5")
    draft_hash  = Column(CHAR(32),  nullable=False, comment="AI初稿MD5")
    final_hash  = Column(CHAR(32),  nullable=True,  comment="定稿MD5")
    prompt_text = Column(Text,      nullable=True,  comment="提示词原文")
    trace_time  = Column(DateTime,  nullable=False, default=datetime.utcnow, comment="留存时间戳")

    __table_args__ = (
        Index("idx_trace_task_id",    "task_id"),
        Index("idx_trace_chapter_id", "chapter_id"),
        Index("idx_trace_time",       "trace_time"),
    )


# ---------------------------------------------------------------------------
# 6. publish_records — 发布记录表
# ---------------------------------------------------------------------------
class PublishRecord(Base):
    __tablename__ = "publish_records"

    id            = Column(CHAR(36),      primary_key=True, comment="发布记录UUID")
    task_id       = Column(CHAR(36),      ForeignKey("creation_tasks.id", ondelete="RESTRICT"), nullable=False)
    chapter_id    = Column(CHAR(36),      ForeignKey("chapters.id", ondelete="SET NULL"), nullable=True)
    account_id    = Column(CHAR(36),      ForeignKey("accounts.id", ondelete="RESTRICT"), nullable=False)
    platform      = Column(String(32),    nullable=False)
    chapter_no    = Column(SmallInteger,  nullable=False)
    chapter_title = Column(String(256),   nullable=True)
    word_count    = Column(Integer,       nullable=False, default=0)
    published_at  = Column(DateTime,      nullable=False, default=datetime.utcnow)
    read_count    = Column(Integer,       nullable=False, default=0)
    collect_count = Column(Integer,       nullable=False, default=0)
    comment_count = Column(Integer,       nullable=False, default=0)
    revenue       = Column(Numeric(10, 2), nullable=False, default=Decimal("0.00"))
    data_synced_at = Column(DateTime,     nullable=True)

    __table_args__ = (
        Index("idx_pub_task_id",      "task_id"),
        Index("idx_pub_account_id",   "account_id"),
        Index("idx_pub_platform",     "platform"),
        Index("idx_pub_published_at", "published_at"),
        Index("idx_pub_account_date", "account_id", "published_at"),
        CheckConstraint(
            "platform IN ('fanqie','qimao','zhihu','xiaohongshu','douyin')",
            name="chk_pub_platform",
        ),
    )


# ---------------------------------------------------------------------------
# 7. crawl_jobs — 爬虫任务表
# ---------------------------------------------------------------------------
class CrawlJob(Base):
    __tablename__ = "crawl_jobs"

    id            = Column(BigInteger,   primary_key=True, autoincrement=True)
    spider_name   = Column(String(64),   nullable=False, comment="爬虫名称")
    target_url    = Column(String(512),  nullable=True,  comment="本次爬取目标URL")
    status        = Column(String(16),   nullable=False, default="pending")
    items_crawled = Column(Integer,      nullable=False, default=0)
    items_skipped = Column(Integer,      nullable=False, default=0)
    error_msg     = Column(Text,         nullable=True)
    started_at    = Column(DateTime,     nullable=True)
    finished_at   = Column(DateTime,     nullable=True)
    created_at    = Column(DateTime,     nullable=False, default=datetime.utcnow)

    __table_args__ = (
        Index("idx_crawl_spider_name", "spider_name"),
        Index("idx_crawl_status",      "status"),
        Index("idx_crawl_created_at",  "created_at"),
        CheckConstraint("status IN ('pending','running','success','failed','skipped')", name="chk_crawl_status"),
    )


# ---------------------------------------------------------------------------
# 8. corpus_meta — 语料元数据表
# ---------------------------------------------------------------------------
class CorpusMeta(Base):
    __tablename__ = "corpus_meta"

    id                   = Column(CHAR(36),      primary_key=True, comment="语料UUID")
    mongo_id             = Column(String(64),    nullable=True,  comment="MongoDB ObjectId")
    source               = Column(String(32),    nullable=False, comment="来源")
    category             = Column(String(32),    nullable=False, comment="题材分类")
    corpus_type          = Column(String(16),    nullable=False, default="raw")
    book_title           = Column(String(256),   nullable=True)
    chapter_title        = Column(String(256),   nullable=True)
    word_count           = Column(Integer,       nullable=False, default=0)
    quality_score        = Column(Numeric(4, 3), nullable=False, default=Decimal("0.000"))
    content_hash         = Column(CHAR(32),      nullable=False, unique=True, comment="内容MD5（去重）")
    is_valid             = Column(Boolean,       nullable=False, default=True)
    is_copyright_suspect = Column(Boolean,       nullable=False, default=False)
    crawl_job_id         = Column(BigInteger,    nullable=True)
    crawl_time           = Column(DateTime,      nullable=False, default=datetime.utcnow)
    selected_at          = Column(DateTime,      nullable=True)

    __table_args__ = (
        Index("idx_corpus_category",      "category"),
        Index("idx_corpus_type",          "corpus_type"),
        Index("idx_corpus_quality_score", "quality_score"),
        Index("idx_corpus_crawl_time",    "crawl_time"),
        Index("idx_corpus_is_valid",      "is_valid"),
        CheckConstraint("category IN ('female_rebirth','male_power','suspense','romance')", name="chk_corpus_category"),
        CheckConstraint("corpus_type IN ('raw','training')", name="chk_corpus_type"),
        CheckConstraint("quality_score BETWEEN 0.000 AND 1.000", name="chk_quality_range"),
    )


# ---------------------------------------------------------------------------
# 9. model_call_logs — 模型调用日志表
# ---------------------------------------------------------------------------
class ModelCallLog(Base):
    __tablename__ = "model_call_logs"

    id            = Column(BigInteger,    primary_key=True, autoincrement=True)
    task_id       = Column(CHAR(36),      nullable=True)
    provider      = Column(String(32),    nullable=False)
    model_name    = Column(String(64),    nullable=False)
    stage         = Column(String(32),    nullable=False)
    prompt_tokens = Column(Integer,       nullable=False, default=0)
    output_tokens = Column(Integer,       nullable=False, default=0)
    cost_yuan     = Column(Numeric(8, 4), nullable=False, default=Decimal("0.0000"))
    latency_ms    = Column(Integer,       nullable=False, default=0)
    status        = Column(String(16),    nullable=False, default="success")
    retry_attempt = Column(SmallInteger,  nullable=False, default=0)
    error_code    = Column(String(32),    nullable=True)
    called_at     = Column(DateTime,      nullable=False, default=datetime.utcnow)

    __table_args__ = (
        Index("idx_log_task_id",   "task_id"),
        Index("idx_log_provider",  "provider"),
        Index("idx_log_stage",     "stage"),
        Index("idx_log_status",    "status"),
        Index("idx_log_called_at", "called_at"),
        CheckConstraint("provider IN ('minimax','doubao','qwen','deepseek')", name="chk_log_provider"),
        CheckConstraint("status IN ('success','failed','rate_limited','timeout')", name="chk_log_status"),
    )


# ---------------------------------------------------------------------------
# 10. system_alerts — 系统告警日志表
# ---------------------------------------------------------------------------
class SystemAlert(Base):
    __tablename__ = "system_alerts"

    id          = Column(BigInteger,  primary_key=True, autoincrement=True)
    alert_type  = Column(String(64),  nullable=False)
    severity    = Column(String(16),  nullable=False, default="warning")
    task_id     = Column(CHAR(36),    nullable=True)
    message     = Column(Text,        nullable=False)
    resolved    = Column(Boolean,     nullable=False, default=False)
    resolved_at = Column(DateTime,    nullable=True)
    created_at  = Column(DateTime,    nullable=False, default=datetime.utcnow)

    __table_args__ = (
        Index("idx_alert_type",       "alert_type"),
        Index("idx_alert_severity",   "severity"),
        Index("idx_alert_resolved",   "resolved"),
        Index("idx_alert_created_at", "created_at"),
        CheckConstraint("severity IN ('info','warning','error','critical')", name="chk_severity"),
    )


# ---------------------------------------------------------------------------
# 11. dashboard_stats — 数据看板聚合缓存表
# ---------------------------------------------------------------------------
class DashboardStats(Base):
    __tablename__ = "dashboard_stats"

    id             = Column(BigInteger,    primary_key=True, autoincrement=True)
    account_id     = Column(CHAR(36),      ForeignKey("accounts.id", ondelete="CASCADE"), nullable=False)
    stat_date      = Column(Date,          nullable=False)
    platform       = Column(String(32),    nullable=False)
    agent_type     = Column(String(32),    nullable=False)
    chapters_count = Column(Integer,       nullable=False, default=0)
    total_words    = Column(Integer,       nullable=False, default=0)
    total_reads    = Column(Integer,       nullable=False, default=0)
    total_collects = Column(Integer,       nullable=False, default=0)
    total_revenue  = Column(Numeric(10, 2), nullable=False, default=Decimal("0.00"))
    updated_at     = Column(DateTime,      nullable=False, default=datetime.utcnow, onupdate=datetime.utcnow)

    __table_args__ = (
        UniqueConstraint("account_id", "stat_date", "platform", name="uk_account_date_platform"),
        Index("idx_stats_stat_date",  "stat_date"),
        Index("idx_stats_agent_type", "agent_type"),
    )

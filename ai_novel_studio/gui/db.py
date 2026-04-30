"""
GUI 专用数据库连接 — 同步 SQLAlchemy（PyQt6 不需要 async）
直连 MySQL，连接参数从 config.env / 环境变量读取
"""
import os
from contextlib import contextmanager
from datetime import datetime, date
from decimal import Decimal
from typing import Optional, Generator

from sqlalchemy import (
    BigInteger, Boolean, CHAR, CheckConstraint, Column, Date,
    DateTime, ForeignKey, Index, Integer, JSON, Numeric,
    SmallInteger, String, Text, UniqueConstraint, create_engine, text,
)
from sqlalchemy.orm import DeclarativeBase, sessionmaker, Session

from ai_novel_studio.config.settings import settings

# ---------------------------------------------------------------------------
# 连接配置（从 settings 读取，自动加载 config.env）
# ---------------------------------------------------------------------------
engine = create_engine(
    settings.mysql_sync_url,
    echo=False,
    pool_pre_ping=True,
    pool_recycle=3600,
    connect_args={"connect_timeout": 10},
)

SessionLocal = sessionmaker(bind=engine, autocommit=False, autoflush=False)


@contextmanager
def get_session() -> Generator[Session, None, None]:
    """同步 Session 上下文管理器"""
    session = SessionLocal()
    try:
        yield session
        session.commit()
    except Exception:
        session.rollback()
        raise
    finally:
        session.close()


def test_connection() -> bool:
    """测试数据库连通性"""
    try:
        with engine.connect() as conn:
            conn.execute(text("SELECT 1"))
        return True
    except Exception:
        return False


# ---------------------------------------------------------------------------
# ORM 模型（与 schema.sql 一致）
# ---------------------------------------------------------------------------
class Base(DeclarativeBase):
    pass


class Account(Base):
    __tablename__ = "accounts"
    id           = Column(CHAR(36),    primary_key=True)
    platform     = Column(String(32),  nullable=False)
    agent_type   = Column(String(32),  nullable=False)
    username     = Column(String(128), nullable=False)
    display_name = Column(String(128), nullable=True)
    status       = Column(String(16),  nullable=False, default="active")
    daily_quota  = Column(Integer,     nullable=False, default=3)
    total_published = Column(Integer,  nullable=False, default=0)
    created_at   = Column(DateTime,    nullable=False, default=datetime.utcnow)
    updated_at   = Column(DateTime,    nullable=False, default=datetime.utcnow)


class CreationTask(Base):
    __tablename__ = "creation_tasks"
    id            = Column(CHAR(36),   primary_key=True)
    agent_type    = Column(String(32), nullable=False)
    stage         = Column(String(32), nullable=False, default="pending")
    topic         = Column(Text,       nullable=True)
    outline       = Column(JSON,       nullable=True)
    word_count    = Column(Integer,    nullable=False, default=0)
    retry_count   = Column(SmallInteger, nullable=False, default=0)
    reject_reason = Column(Text,       nullable=True)
    trend_data    = Column(Text,       nullable=True)
    topic_at      = Column(DateTime,   nullable=True)
    outline_at    = Column(DateTime,   nullable=True)
    content_at    = Column(DateTime,   nullable=True)
    polish_at     = Column(DateTime,   nullable=True)
    review_at     = Column(DateTime,   nullable=True)
    publish_at    = Column(DateTime,   nullable=True)
    created_at    = Column(DateTime,   nullable=False, default=datetime.utcnow)
    updated_at    = Column(DateTime,   nullable=False, default=datetime.utcnow)


class PublishRecord(Base):
    __tablename__ = "publish_records"
    id            = Column(CHAR(36),      primary_key=True)
    task_id       = Column(CHAR(36),      ForeignKey("creation_tasks.id"), nullable=False)
    chapter_id    = Column(CHAR(36),      nullable=True)
    account_id    = Column(CHAR(36),      ForeignKey("accounts.id"), nullable=False)
    platform      = Column(String(32),    nullable=False)
    chapter_no    = Column(SmallInteger,  nullable=False)
    chapter_title = Column(String(256),   nullable=True)
    word_count    = Column(Integer,       nullable=False, default=0)
    published_at  = Column(DateTime,      nullable=False, default=datetime.utcnow)
    read_count    = Column(Integer,       nullable=False, default=0)
    collect_count = Column(Integer,       nullable=False, default=0)
    revenue       = Column(Numeric(10, 2), nullable=False, default=Decimal("0.00"))


class CopyrightTrace(Base):
    __tablename__ = "copyright_traces"
    id          = Column(CHAR(36),  primary_key=True)
    task_id     = Column(CHAR(36),  ForeignKey("creation_tasks.id"), nullable=False)
    chapter_id  = Column(CHAR(36),  nullable=True)
    prompt_hash = Column(CHAR(32),  nullable=False)
    draft_hash  = Column(CHAR(32),  nullable=False)
    final_hash  = Column(CHAR(32),  nullable=True)
    prompt_text = Column(Text,      nullable=True)
    trace_time  = Column(DateTime,  nullable=False, default=datetime.utcnow)


class CorpusMeta(Base):
    __tablename__ = "corpus_meta"
    id                   = Column(CHAR(36),      primary_key=True)
    mongo_id             = Column(String(64),    nullable=True)
    source               = Column(String(32),    nullable=False)
    category             = Column(String(32),    nullable=False)
    corpus_type          = Column(String(16),    nullable=False, default="raw")
    book_title           = Column(String(256),   nullable=True)
    chapter_title        = Column(String(256),   nullable=True)
    word_count           = Column(Integer,       nullable=False, default=0)
    quality_score        = Column(Numeric(4, 3), nullable=False, default=Decimal("0.000"))
    content_hash         = Column(CHAR(32),      nullable=False, unique=True)
    is_valid             = Column(Boolean,       nullable=False, default=True)
    is_copyright_suspect = Column(Boolean,       nullable=False, default=False)
    crawl_time           = Column(DateTime,      nullable=False, default=datetime.utcnow)


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


class Outline(Base):
    __tablename__ = "outlines"
    id              = Column(CHAR(36),    primary_key=True)
    agent_type      = Column(String(32),  nullable=False)
    batch_id        = Column(CHAR(36),    nullable=False)
    title           = Column(String(256), nullable=True)
    content         = Column(Text,        nullable=False, default="")
    topic_hint      = Column(Text,        nullable=True)
    trend_data      = Column(Text,        nullable=True)
    status          = Column(String(32),  nullable=False, default="pending_review")
    reviewer        = Column(String(64),  nullable=True)
    review_comments = Column(Text,        nullable=True)
    reject_reason   = Column(Text,        nullable=True)
    reviewed_at     = Column(DateTime,    nullable=True)
    novel_id        = Column(CHAR(36),    nullable=True)
    created_at      = Column(DateTime,    nullable=False, default=datetime.utcnow)
    updated_at      = Column(DateTime,    nullable=False, default=datetime.utcnow)


class OutlineReviewHistory(Base):
    __tablename__ = "outline_review_history"
    id          = Column(BigInteger,  primary_key=True, autoincrement=True)
    outline_id  = Column(CHAR(36),    ForeignKey("outlines.id"), nullable=False)
    from_status = Column(String(32),  nullable=True)
    to_status   = Column(String(32),  nullable=False)
    operator    = Column(String(64),  nullable=True)
    remark      = Column(Text,        nullable=True)
    created_at  = Column(DateTime,    nullable=False, default=datetime.utcnow)


class Novel(Base):
    __tablename__ = "novels"
    id                    = Column(CHAR(36),    primary_key=True)
    outline_id            = Column(CHAR(36),    ForeignKey("outlines.id"), nullable=False)
    agent_type            = Column(String(32),  nullable=False)
    title                 = Column(String(256), nullable=True)
    status                = Column(String(32),  nullable=False, default="writing")
    word_count            = Column(Integer,     nullable=False, default=0)
    revision_round        = Column(SmallInteger, nullable=False, default=0)
    reviewer              = Column(String(64),  nullable=True)
    review_comments       = Column(Text,        nullable=True)
    revision_instructions = Column(Text,        nullable=True)
    reject_reason         = Column(Text,        nullable=True)
    reviewed_at           = Column(DateTime,    nullable=True)
    writing_started_at    = Column(DateTime,    nullable=True)
    writing_finished_at   = Column(DateTime,    nullable=True)
    created_at            = Column(DateTime,    nullable=False, default=datetime.utcnow)
    updated_at            = Column(DateTime,    nullable=False, default=datetime.utcnow)


class NovelChapter(Base):
    __tablename__ = "novel_chapters"
    id            = Column(CHAR(36),    primary_key=True)
    novel_id      = Column(CHAR(36),    ForeignKey("novels.id"), nullable=False)
    chapter_no    = Column(SmallInteger, nullable=False)
    chapter_title = Column(String(256), nullable=True)
    content       = Column(Text,        nullable=True)
    word_count    = Column(Integer,     nullable=False, default=0)
    status        = Column(String(16),  nullable=False, default="draft")
    created_at    = Column(DateTime,    nullable=False, default=datetime.utcnow)
    updated_at    = Column(DateTime,    nullable=False, default=datetime.utcnow)


class NovelRevisionHistory(Base):
    __tablename__ = "novel_revision_history"
    id                    = Column(BigInteger,  primary_key=True, autoincrement=True)
    novel_id              = Column(CHAR(36),    ForeignKey("novels.id"), nullable=False)
    revision_round        = Column(SmallInteger, nullable=False)
    revision_instructions = Column(Text,        nullable=False)
    reviewer              = Column(String(64),  nullable=True)
    content_snapshot      = Column(Text,        nullable=True)
    created_at            = Column(DateTime,    nullable=False, default=datetime.utcnow)

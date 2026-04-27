"""
GUI 专用数据库连接 — 同步 SQLAlchemy（PyQt6 不需要 async）
直连 MySQL localhost:3306，账号密码 root/root
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

# ---------------------------------------------------------------------------
# 连接配置（优先读环境变量，默认 root/root@localhost）
# ---------------------------------------------------------------------------
MYSQL_SYNC_URL: str = os.getenv(
    "MYSQL_SYNC_URL",
    "mysql+pymysql://root:root@localhost:3306/ai_novel_studio?charset=utf8mb4",
)

engine = create_engine(
    MYSQL_SYNC_URL,
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

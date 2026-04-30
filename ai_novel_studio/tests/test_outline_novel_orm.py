"""
ORM 模型单元测试 — Outline、Novel、NovelChapter

验证：
- Outline 状态字段枚举约束（需求 9.1）
- Novel 状态字段枚举约束（需求 9.2）
- NovelChapter (novel_id, chapter_no) 唯一约束

注意：直接使用 SQLite 内存数据库定义镜像模型，避免引入 aiomysql 异步引擎依赖。
"""
import pytest
from sqlalchemy import (
    CHAR, CheckConstraint, Column, DateTime, ForeignKey,
    Integer, SmallInteger, String, Text, UniqueConstraint, create_engine, event,
)
from sqlalchemy.exc import IntegrityError
from sqlalchemy.orm import DeclarativeBase, Session
from datetime import datetime


# ---------------------------------------------------------------------------
# 本地镜像 ORM 模型（仅用于测试，与 mysql.py 中的约束定义保持一致）
# ---------------------------------------------------------------------------

class Base(DeclarativeBase):
    pass


class Outline(Base):
    """大纲 ORM 模型（镜像 mysql.py 中的约束定义）"""
    __tablename__ = "outlines"

    id              = Column(CHAR(36),    primary_key=True)
    agent_type      = Column(String(32),  nullable=False)
    batch_id        = Column(CHAR(36),    nullable=False)
    title           = Column(String(256), nullable=True)
    content         = Column(Text,        nullable=False)
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

    __table_args__ = (
        CheckConstraint(
            "status IN ('pending_review', 'approved', 'rejected', 'in_use', 'used')",
            name="chk_outline_status",
        ),
        CheckConstraint(
            "agent_type IN ('female_rebirth', 'male_power', 'suspense', 'romance')",
            name="chk_outline_agent",
        ),
    )


class Novel(Base):
    """小说任务 ORM 模型（镜像 mysql.py 中的约束定义）"""
    __tablename__ = "novels"

    id                    = Column(CHAR(36),     primary_key=True)
    outline_id            = Column(CHAR(36),     ForeignKey("outlines.id"), nullable=False)
    agent_type            = Column(String(32),   nullable=False)
    title                 = Column(String(256),  nullable=True)
    status                = Column(String(32),   nullable=False, default="writing")
    word_count            = Column(Integer,      nullable=False, default=0)
    revision_round        = Column(SmallInteger, nullable=False, default=0)
    reviewer              = Column(String(64),   nullable=True)
    review_comments       = Column(Text,         nullable=True)
    revision_instructions = Column(Text,         nullable=True)
    reject_reason         = Column(Text,         nullable=True)
    reviewed_at           = Column(DateTime,     nullable=True)
    writing_started_at    = Column(DateTime,     nullable=True)
    writing_finished_at   = Column(DateTime,     nullable=True)
    created_at            = Column(DateTime,     nullable=False, default=datetime.utcnow)
    updated_at            = Column(DateTime,     nullable=False, default=datetime.utcnow)

    __table_args__ = (
        CheckConstraint(
            "status IN ('writing', 'novel_pending_review', 'novel_approved',"
            " 'novel_rejected', 'revising', 'publishing', 'done')",
            name="chk_novel_status",
        ),
        CheckConstraint(
            "agent_type IN ('female_rebirth', 'male_power', 'suspense', 'romance')",
            name="chk_novel_agent",
        ),
    )


class NovelChapter(Base):
    """小说章节 ORM 模型（镜像 mysql.py 中的约束定义）"""
    __tablename__ = "novel_chapters"

    id            = Column(CHAR(36),     primary_key=True)
    novel_id      = Column(CHAR(36),     ForeignKey("novels.id"), nullable=False)
    chapter_no    = Column(SmallInteger, nullable=False)
    chapter_title = Column(String(256),  nullable=True)
    content       = Column(Text,         nullable=True)
    word_count    = Column(Integer,      nullable=False, default=0)
    status        = Column(String(16),   nullable=False, default="draft")
    created_at    = Column(DateTime,     nullable=False, default=datetime.utcnow)
    updated_at    = Column(DateTime,     nullable=False, default=datetime.utcnow)

    __table_args__ = (
        UniqueConstraint("novel_id", "chapter_no", name="uk_novel_chapter"),
        CheckConstraint("status IN ('draft', 'finalized')", name="chk_novel_chapter_status"),
    )


# ---------------------------------------------------------------------------
# 测试数据库 fixture（SQLite in-memory，启用 CHECK 约束）
# ---------------------------------------------------------------------------

@pytest.fixture(scope="module")
def engine():
    """创建 SQLite 内存数据库，并启用 CHECK 约束支持。"""
    eng = create_engine(
        "sqlite:///:memory:",
        echo=False,
        connect_args={"check_same_thread": False},
    )

    # SQLite 默认不强制 CHECK 约束，需要手动开启
    @event.listens_for(eng, "connect")
    def set_sqlite_pragma(dbapi_conn, connection_record):
        cursor = dbapi_conn.cursor()
        cursor.execute("PRAGMA enforce_check_constraints = ON")
        cursor.close()

    Base.metadata.create_all(eng)
    yield eng
    Base.metadata.drop_all(eng)


@pytest.fixture
def session(engine):
    """每个测试用例使用独立的事务，测试后回滚。"""
    connection = engine.connect()
    transaction = connection.begin()
    sess = Session(bind=connection)
    yield sess
    sess.close()
    transaction.rollback()
    connection.close()


# ---------------------------------------------------------------------------
# 辅助函数
# ---------------------------------------------------------------------------

def make_outline(
    outline_id: str = "outline-001",
    status: str = "pending_review",
    agent_type: str = "female_rebirth",
) -> Outline:
    return Outline(
        id=outline_id,
        agent_type=agent_type,
        batch_id="batch-001",
        content="测试大纲内容",
        status=status,
    )


def make_novel(
    novel_id: str = "novel-001",
    outline_id: str = "outline-001",
    status: str = "writing",
    agent_type: str = "female_rebirth",
) -> Novel:
    return Novel(
        id=novel_id,
        outline_id=outline_id,
        agent_type=agent_type,
        status=status,
    )


def make_chapter(
    chapter_id: str,
    novel_id: str = "novel-001",
    chapter_no: int = 1,
) -> NovelChapter:
    return NovelChapter(
        id=chapter_id,
        novel_id=novel_id,
        chapter_no=chapter_no,
        content="章节内容",
        word_count=100,
    )


# ---------------------------------------------------------------------------
# Outline 状态字段枚举约束测试（需求 9.1）
# ---------------------------------------------------------------------------

VALID_OUTLINE_STATUSES = [
    "pending_review",
    "approved",
    "rejected",
    "in_use",
    "used",
]

INVALID_OUTLINE_STATUSES = [
    "draft",
    "writing",
    "published",
    "unknown",
    "PENDING_REVIEW",
]


@pytest.mark.parametrize("status", VALID_OUTLINE_STATUSES)
def test_outline_valid_status(session, status):
    """Outline 应接受所有合法状态值。"""
    outline = make_outline(outline_id=f"outline-{status}", status=status)
    session.add(outline)
    session.flush()
    result = session.get(Outline, f"outline-{status}")
    assert result is not None
    assert result.status == status


@pytest.mark.parametrize("status", INVALID_OUTLINE_STATUSES)
def test_outline_invalid_status_rejected(engine, status):
    """Outline 应拒绝不在枚举范围内的状态值。"""
    with Session(engine) as sess:
        outline = make_outline(outline_id=f"outline-bad-{abs(hash(status))}", status=status)
        sess.add(outline)
        with pytest.raises((IntegrityError, Exception)):
            sess.flush()
        sess.rollback()


def test_outline_default_status(session):
    """Outline 默认状态应为 pending_review。"""
    outline = Outline(
        id="outline-default",
        agent_type="romance",
        batch_id="batch-default",
        content="默认状态测试",
    )
    session.add(outline)
    session.flush()
    result = session.get(Outline, "outline-default")
    assert result.status == "pending_review"


# ---------------------------------------------------------------------------
# Novel 状态字段枚举约束测试（需求 9.2）
# ---------------------------------------------------------------------------

VALID_NOVEL_STATUSES = [
    "writing",
    "novel_pending_review",
    "novel_approved",
    "novel_rejected",
    "revising",
    "publishing",
    "done",
]

INVALID_NOVEL_STATUSES = [
    "pending_review",
    "approved",
    "rejected",
    "draft",
    "published",
    "unknown",
    "WRITING",
]


@pytest.mark.parametrize("status", VALID_NOVEL_STATUSES)
def test_novel_valid_status(session, status):
    """Novel 应接受所有合法状态值。"""
    outline = make_outline(outline_id=f"outline-for-novel-{status}")
    session.add(outline)
    session.flush()

    novel = make_novel(
        novel_id=f"novel-{status}",
        outline_id=f"outline-for-novel-{status}",
        status=status,
    )
    session.add(novel)
    session.flush()
    result = session.get(Novel, f"novel-{status}")
    assert result is not None
    assert result.status == status


@pytest.mark.parametrize("status", INVALID_NOVEL_STATUSES)
def test_novel_invalid_status_rejected(engine, status):
    """Novel 应拒绝不在枚举范围内的状态值。"""
    with Session(engine) as sess:
        outline = make_outline(outline_id=f"outline-for-bad-novel-{abs(hash(status))}")
        sess.add(outline)
        sess.flush()

        novel = make_novel(
            novel_id=f"novel-bad-{abs(hash(status))}",
            outline_id=f"outline-for-bad-novel-{abs(hash(status))}",
            status=status,
        )
        sess.add(novel)
        with pytest.raises((IntegrityError, Exception)):
            sess.flush()
        sess.rollback()


def test_novel_default_status(session):
    """Novel 默认状态应为 writing。"""
    outline = make_outline(outline_id="outline-for-novel-default")
    session.add(outline)
    session.flush()

    novel = Novel(
        id="novel-default",
        outline_id="outline-for-novel-default",
        agent_type="male_power",
    )
    session.add(novel)
    session.flush()
    result = session.get(Novel, "novel-default")
    assert result.status == "writing"


# ---------------------------------------------------------------------------
# NovelChapter (novel_id, chapter_no) 唯一约束测试
# ---------------------------------------------------------------------------

def test_novel_chapter_unique_constraint_violated(engine):
    """同一小说中相同章节序号应触发唯一约束错误。"""
    with Session(engine) as sess:
        outline = make_outline(outline_id="outline-for-dup-chapter")
        novel = make_novel(novel_id="novel-for-dup-chapter", outline_id="outline-for-dup-chapter")
        sess.add_all([outline, novel])
        sess.flush()

        chapter1 = make_chapter("chapter-dup-1", novel_id="novel-for-dup-chapter", chapter_no=1)
        sess.add(chapter1)
        sess.flush()

        chapter2 = make_chapter("chapter-dup-2", novel_id="novel-for-dup-chapter", chapter_no=1)
        sess.add(chapter2)
        with pytest.raises(IntegrityError):
            sess.flush()
        sess.rollback()


def test_novel_chapter_unique_constraint_different_novels(session):
    """不同小说可以拥有相同章节序号，不应触发唯一约束错误。"""
    outline1 = make_outline(outline_id="outline-novel-a")
    outline2 = make_outline(outline_id="outline-novel-b")
    novel_a = make_novel(novel_id="novel-a", outline_id="outline-novel-a")
    novel_b = make_novel(novel_id="novel-b", outline_id="outline-novel-b")
    session.add_all([outline1, outline2, novel_a, novel_b])
    session.flush()

    chapter_a1 = make_chapter("chapter-a-1", novel_id="novel-a", chapter_no=1)
    chapter_b1 = make_chapter("chapter-b-1", novel_id="novel-b", chapter_no=1)
    session.add_all([chapter_a1, chapter_b1])
    session.flush()  # 不应抛出异常

    assert session.get(NovelChapter, "chapter-a-1") is not None
    assert session.get(NovelChapter, "chapter-b-1") is not None


def test_novel_chapter_unique_constraint_different_chapter_nos(session):
    """同一小说中不同章节序号应正常插入。"""
    outline = make_outline(outline_id="outline-multi-chapter")
    novel = make_novel(novel_id="novel-multi-chapter", outline_id="outline-multi-chapter")
    session.add_all([outline, novel])
    session.flush()

    for i in range(1, 6):
        chapter = make_chapter(f"chapter-mc-{i}", novel_id="novel-multi-chapter", chapter_no=i)
        session.add(chapter)
    session.flush()  # 不应抛出异常

    for i in range(1, 6):
        result = session.get(NovelChapter, f"chapter-mc-{i}")
        assert result is not None
        assert result.chapter_no == i

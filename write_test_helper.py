content = r"""
\"\"\"
Small novel revision tests
\"\"\"
from __future__ import annotations
import json
import uuid
from datetime import datetime
from typing import Optional
import pytest
from hypothesis import given, settings
from hypothesis import strategies as st
from sqlalchemy import (
    CHAR, CheckConstraint, Column, DateTime, ForeignKey,
    Index, Integer, SmallInteger, String, Text, UniqueConstraint,
    create_engine, event,
)
from sqlalchemy.orm import DeclarativeBase, Session


class Base(DeclarativeBase):
    pass


class Outline(Base):
    __tablename__ = "outlines"
    id = Column(CHAR(36), primary_key=True)
    agent_type = Column(String(32), nullable=False)
    batch_id = Column(CHAR(36), nullable=False)
    content = Column(Text, nullable=False)
    status = Column(String(32), nullable=False, default="pending_review")
    novel_id = Column(CHAR(36), nullable=True)
    created_at = Column(DateTime, nullable=False, default=datetime.utcnow)
    updated_at = Column(DateTime, nullable=False, default=datetime.utcnow)
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
    __tablename__ = "novels"
    id = Column(CHAR(36), primary_key=True)
    outline_id = Column(CHAR(36), ForeignKey("outlines.id"), nullable=False)
    agent_type = Column(String(32), nullable=False)
    title = Column(String(256), nullable=True)
    status = Column(String(32), nullable=False, default="writing")
    word_count = Column(Integer, nullable=False, default=0)
    revision_round = Column(SmallInteger, nullable=False, default=0)
    reviewer = Column(String(64), nullable=True)
    review_comments = Column(Text, nullable=True)
    revision_instructions = Column(Text, nullable=True)
    reject_reason = Column(Text, nullable=True)
    reviewed_at = Column(DateTime, nullable=True)
    writing_started_at = Column(DateTime, nullable=True)
    writing_finished_at = Column(DateTime, nullable=True)
    created_at = Column(DateTime, nullable=False, default=datetime.utcnow)
    updated_at = Column(DateTime, nullable=False, default=datetime.utcnow)
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
    __tablename__ = "novel_chapters"
    id = Column(CHAR(36), primary_key=True)
    novel_id = Column(CHAR(36), ForeignKey("novels.id"), nullable=False)
    chapter_no = Column(SmallInteger, nullable=False)
    chapter_title = Column(String(256), nullable=True)
    content = Column(Text, nullable=True)
    word_count = Column(Integer, nullable=False, default=0)
    status = Column(String(16), nullable=False, default="draft")
    created_at = Column(DateTime, nullable=False, default=datetime.utcnow)
    updated_at = Column(DateTime, nullable=False, default=datetime.utcnow)
    __table_args__ = (
        UniqueConstraint("novel_id", "chapter_no", name="uk_novel_chapter"),
        CheckConstraint("status IN ('draft', 'finalized')", name="chk_novel_chapter_status"),
    )


class NovelRevisionHistory(Base):
    __tablename__ = "novel_revision_history"
    id = Column(Integer, primary_key=True, autoincrement=True)
    novel_id = Column(CHAR(36), ForeignKey("novels.id"), nullable=False)
    revision_round = Column(SmallInteger, nullable=False)
    revision_instructions = Column(Text, nullable=False)
    reviewer = Column(String(64), nullable=True)
    content_snapshot = Column(Text, nullable=True)
    created_at = Column(DateTime, nullable=False, default=datetime.utcnow)
    __table_args__ = (
        Index("idx_revision_novel_id", "novel_id"),
    )


VALID_AGENT_TYPES = ["female_rebirth", "male_power", "suspense", "romance"]


def create_test_engine():
    eng = create_engine(
        "sqlite:///:memory:",
        echo=False,
        connect_args={"check_same_thread": False},
    )

    @event.listens_for(eng, "connect")
    def set_sqlite_pragma(dbapi_conn, connection_record):
        cursor = dbapi_conn.cursor()
        cursor.execute("PRAGMA enforce_check_constraints = ON")
        cursor.close()

    Base.metadata.create_all(eng)
    return eng


def make_outline(
    status: str = "in_use",
    agent_type: str = "female_rebirth",
    outline_id: Optional[str] = None,
    content: str = "大纲内容",
) -> Outline:
    return Outline(
        id=outline_id or str(uuid.uuid4()),
        agent_type=agent_type,
        batch_id=str(uuid.uuid4()),
        content=content,
        status=status,
    )


def make_novel(
    outline_id: str,
    agent_type: str = "female_rebirth",
    status: str = "revising",
    novel_id: Optional[str] = None,
    revision_round: int = 0,
) -> Novel:
    return Novel(
        id=novel_id or str(uuid.uuid4()),
        outline_id=outline_id,
        agent_type=agent_type,
        status=status,
        word_count=0,
        revision_round=revision_round,
    )


def make_chapter(
    novel_id: str,
    chapter_no: int,
    content: str = "chapter content",
) -> NovelChapter:
    return NovelChapter(
        id=str(uuid.uuid4()),
        novel_id=novel_id,
        chapter_no=chapter_no,
        content=content,
        word_count=len(content),
        status="draft",
    )


def apply_revision_round(
    session: Session,
    novel_id: str,
    revision_instructions: str,
    chapters_snapshot: Optional[dict] = None,
) -> int:
    \"\"\"
    Simulate one revision round: increment revision_round, save history, update status.
    Returns the new revision_round.
    \"\"\"
    novel = session.get(Novel, novel_id)
    assert novel is not None

    chapters = (
        session.query(NovelChapter)
        .filter(NovelChapter.novel_id == novel_id)
        .order_by(NovelChapter.chapter_no)
        .all()
    )

    snapshot = chapters_snapshot or {
        str(c.chapter_no): {
            "id": c.id,
            "chapter_no": c.chapter_no,
            "chapter_title": c.chapter_title,
            "content": c.content,
            "word_count": c.word_count,
        }
        for c in chapters
    }
    snapshot_json = json.dumps(snapshot, ensure_ascii=False)

    new_round = novel.revision_round + 1
    now = datetime.utcnow()

    history = NovelRevisionHistory(
        novel_id=novel_id,
        revision_round=new_round,
        revision_instructions=revision_instructions,
        reviewer="reviewer",
        content_snapshot=snapshot_json,
        created_at=now,
    )
    session.add(history)

    novel.status = "revising"
    novel.revision_round = new_round
    novel.revision_instructions = revision_instructions
    novel.reviewed_at = now
    novel.updated_at = now

    session.flush()
    return new_round


def complete_revision(session: Session, novel_id: str, revised_chapters: list[str]) -> None:
    \"\"\"
    Simulate task_revise_novel completion: update chapters and set status to novel_pending_review.
    \"\"\"
    chapters = (
        session.query(NovelChapter)
        .filter(NovelChapter.novel_id == novel_id)
        .order_by(NovelChapter.chapter_no)
        .all()
    )

    total_words = 0
    for chapter, new_content in zip(chapters, revised_chapters):
        chapter.content = new_content
        chapter.word_count = len(new_content)
        chapter.updated_at = datetime.utcnow()
        total_words += len(new_content)

    novel = session.get(Novel, novel_id)
    novel.status = "novel_pending_review"
    novel.word_count = total_words
    novel.updated_at = datetime.utcnow()
    session.flush()


# ---------------------------------------------------------------------------
# Task 8.1: Property 10 - revision_round monotonically increasing
# Validates: Requirements 8.1
# ---------------------------------------------------------------------------

agent_type_st = st.sampled_from(VALID_AGENT_TYPES)
revision_instructions_st = st.text(min_size=1, max_size=200)
revision_count_st = st.integers(min_value=1, max_value=5)


@given(agent_type=agent_type_st, num_revisions=revision_count_st)
@settings(max_examples=30, deadline=None)
def test_revision_round_monotonically_increasing(agent_type: str, num_revisions: int):
    \"\"\"
    **Validates: Requirements 8.1**

    Property 10: revision_round monotonically increasing

    For any novel, each time a revision is submitted, novels.revision_round
    should increase by exactly 1. The round should never decrease or skip.
    \"\"\"
    engine = create_test_engine()
    with Session(engine) as sess:
        outline = make_outline(status="in_use", agent_type=agent_type)
        sess.add(outline)
        sess.flush()

        novel = make_novel(
            outline_id=outline.id,
            agent_type=agent_type,
            status="novel_pending_review",
            revision_round=0,
        )
        sess.add(novel)
        sess.flush()
        novel_id = novel.id

        prev_round = 0
        for i in range(num_revisions):
            # Set status to novel_pending_review before each revision
            n = sess.get(Novel, novel_id)
            n.status = "novel_pending_review"
            sess.flush()

            new_round = apply_revision_round(sess, novel_id, f"revision instruction {i+1}")

            # Property: new_round == prev_round + 1 (strictly monotone, no skip)
            assert new_round == prev_round + 1, (
                f"revision_round should increase by 1: prev={prev_round}, new={new_round}"
            )
            assert new_round > prev_round, (
                f"revision_round should never decrease: prev={prev_round}, new={new_round}"
            )

            prev_round = new_round

        # Final check: total rounds equals number of revisions
        final_novel = sess.get(Novel, novel_id)
        assert final_novel.revision_round == num_revisions, (
            f"Final revision_round should be {num_revisions}, got {final_novel.revision_round}"
        )

    Base.metadata.drop_all(engine)


@given(agent_type=agent_type_st)
@settings(max_examples=20, deadline=None)
def test_revision_round_starts_at_zero(agent_type: str):
    \"\"\"
    **Validates: Requirements 8.1**

    Property 10: revision_round starts at 0 for new novels.
    \"\"\"
    engine = create_test_engine()
    with Session(engine) as sess:
        outline = make_outline(status="in_use", agent_type=agent_type)
        sess.add(outline)
        sess.flush()

        novel = make_novel(
            outline_id=outline.id,
            agent_type=agent_type,
            status="writing",
            revision_round=0,
        )
        sess.add(novel)
        sess.flush()

        assert novel.revision_round == 0, (
            f"New novel revision_round should be 0, got {novel.revision_round}"
        )

    Base.metadata.drop_all(engine)


@given(
    agent_type=agent_type_st,
    initial_round=st.integers(min_value=0, max_value=10),
)
@settings(max_examples=30, deadline=None)
def test_revision_round_increments_from_any_starting_point(
    agent_type: str, initial_round: int
):
    \"\"\"
    **Validates: Requirements 8.1**

    Property 10: revision_round increments by exactly 1 regardless of starting value.
    \"\"\"
    engine = create_test_engine()
    with Session(engine) as sess:
        outline = make_outline(status="in_use", agent_type=agent_type)
        sess.add(outline)
        sess.flush()

        novel = make_novel(
            outline_id=outline.id,
            agent_type=agent_type,
            status="novel_pending_review",
            revision_round=initial_round,
        )
        sess.add(novel)
        sess.flush()
        novel_id = novel.id

        new_round = apply_revision_round(sess, novel_id, "some revision instruction")

        assert new_round == initial_round + 1, (
            f"revision_round should be {initial_round + 1}, got {new_round}"
        )

        refreshed = sess.get(Novel, novel_id)
        assert refreshed.revision_round == initial_round + 1

    Base.metadata.drop_all(engine)


# ---------------------------------------------------------------------------
# Task 8.2: Property 11 - revision history snapshot round-trip consistency
# Validates: Requirements 8.3
# ---------------------------------------------------------------------------

chapter_content_st = st.text(min_size=1, max_size=300)
chapter_list_st = st.lists(chapter_content_st, min_size=1, max_size=5)


@given(agent_type=agent_type_st, chapter_contents=chapter_list_st)
@settings(max_examples=40, deadline=None)
def test_revision_snapshot_roundtrip_consistency(
    agent_type: str, chapter_contents: list[str]
):
    \"\"\"
    **Validates: Requirements 8.3**

    Property 11: revision history snapshot round-trip consistency

    For any revision history record, deserializing content_snapshot (JSON)
    should restore a data structure equivalent to the pre-revision chapter contents.
    Serializing then deserializing should produce an equivalent object (round-trip property).
    \"\"\"
    engine = create_test_engine()
    with Session(engine) as sess:
        outline = make_outline(status="in_use", agent_type=agent_type)
        sess.add(outline)
        sess.flush()

        novel = make_novel(
            outline_id=outline.id,
            agent_type=agent_type,
            status="novel_pending_review",
            revision_round=0,
        )
        sess.add(novel)
        sess.flush()
        novel_id = novel.id

        # Add chapters
        for i, content in enumerate(chapter_contents):
            chapter = make_chapter(novel_id, i + 1, content)
            sess.add(chapter)
        sess.flush()

        # Capture original chapter data before revision
        original_chapters = (
            sess.query(NovelChapter)
            .filter(NovelChapter.novel_id == novel_id)
            .order_by(NovelChapter.chapter_no)
            .all()
        )
        original_data = {
            str(c.chapter_no): {
                "id": c.id,
                "chapter_no": c.chapter_no,
                "chapter_title": c.chapter_title,
                "content": c.content,
                "word_count": c.word_count,
            }
            for c in original_chapters
        }

        # Apply revision (saves snapshot)
        apply_revision_round(sess, novel_id, "revision instructions")

        # Retrieve the saved history record
        history = (
            sess.query(NovelRevisionHistory)
            .filter(NovelRevisionHistory.novel_id == novel_id)
            .first()
        )
        assert history is not None, "Revision history record should exist"
        assert history.content_snapshot is not None, "content_snapshot should not be None"

        # Round-trip: deserialize the snapshot
        deserialized = json.loads(history.content_snapshot)

        # Property 11.1: deserialized snapshot should have same keys as original
        assert set(deserialized.keys()) == set(original_data.keys()), (
            f"Snapshot keys {set(deserialized.keys())} should match "
            f"original keys {set(original_data.keys())}"
        )

        # Property 11.2: each chapter's content should be preserved exactly
        for chapter_no_str, original_chapter in original_data.items():
            assert chapter_no_str in deserialized, (
                f"Chapter {chapter_no_str} should be in snapshot"
            )
            restored_chapter = deserialized[chapter_no_str]
            assert restored_chapter["content"] == original_chapter["content"], (
                f"Chapter {chapter_no_str} content mismatch after round-trip: "
                f"expected {original_chapter['content']!r}, "
                f"got {restored_chapter['content']!r}"
            )
            assert restored_chapter["chapter_no"] == original_chapter["chapter_no"], (
                f"Chapter number mismatch: expected {original_chapter['chapter_no']}, "
                f"got {restored_chapter['chapter_no']}"
            )
            assert restored_chapter["word_count"] == original_chapter["word_count"], (
                f"Word count mismatch for chapter {chapter_no_str}"
            )

        # Property 11.3: re-serializing the deserialized data should produce
        # the same JSON (idempotent serialization)
        re_serialized = json.dumps(deserialized, ensure_ascii=False)
        re_deserialized = json.loads(re_serialized)
        assert re_deserialized == deserialized, (
            "Re-serializing the snapshot should produce an equivalent object"
        )

    Base.metadata.drop_all(engine)


@given(agent_type=agent_type_st, chapter_contents=chapter_list_st)
@settings(max_examples=20, deadline=None)
def test_revision_snapshot_preserves_all_chapters(
    agent_type: str, chapter_contents: list[str]
):
    \"\"\"
    **Validates: Requirements 8.3**

    Property 11: snapshot should contain all chapters, none missing.
    \"\"\"
    engine = create_test_engine()
    with Session(engine) as sess:
        outline = make_outline(status="in_use", agent_type=agent_type)
        sess.add(outline)
        sess.flush()

        novel = make_novel(
            outline_id=outline.id,
            agent_type=agent_type,
            status="novel_pending_review",
        )
        sess.add(novel)
        sess.flush()
        novel_id = novel.id

        for i, content in enumerate(chapter_contents):
            sess.add(make_chapter(novel_id, i + 1, content))
        sess.flush()

        apply_revision_round(sess, novel_id, "revision instructions")

        history = (
            sess.query(NovelRevisionHistory)
            .filter(NovelRevisionHistory.novel_id == novel_id)
            .first()
        )
        snapshot = json.loads(history.content_snapshot)

        # All chapter numbers from 1..N should be present
        expected_keys = {str(i + 1) for i in range(len(chapter_contents))}
        assert set(snapshot.keys()) == expected_keys, (
            f"Snapshot should contain chapters {expected_keys}, got {set(snapshot.keys())}"
        )

        # Each chapter content should match the original
        for i, content in enumerate(chapter_contents):
            key = str(i + 1)
            assert snapshot[key]["content"] == content, (
                f"Chapter {key} content should be preserved in snapshot"
            )

    Base.metadata.drop_all(engine)


# ---------------------------------------------------------------------------
# Task 8.3: Unit tests for NovelRevisionService
# Validates: Requirements 7.3, 7.4
# ---------------------------------------------------------------------------


@pytest.fixture(scope="module")
def unit_test_engine():
    eng = create_engine(
        "sqlite:///:memory:",
        echo=False,
        connect_args={"check_same_thread": False},
    )

    @event.listens_for(eng, "connect")
    def set_sqlite_pragma(dbapi_conn, connection_record):
        cursor = dbapi_conn.cursor()
        cursor.execute("PRAGMA enforce_check_constraints = ON")
        cursor.close()

    Base.metadata.create_all(eng)
    yield eng
    Base.metadata.drop_all(eng)


@pytest.fixture
def unit_session(unit_test_engine):
    connection = unit_test_engine.connect()
    transaction = connection.begin()
    sess = Session(bind=connection)
    yield sess
    sess.close()
    transaction.rollback()
    connection.close()


def test_revision_completion_updates_status_to_pending_review(unit_session):
    \"\"\"
    Requirements 7.3: After all chapters are revised, novel status should be
    updated to novel_pending_review.
    \"\"\"
    outline = make_outline(status="in_use")
    unit_session.add(outline)
    unit_session.flush()

    novel = make_novel(outline_id=outline.id, status="revising", revision_round=1)
    unit_session.add(novel)
    unit_session.flush()

    chapters = ["chapter 1 original", "chapter 2 original", "chapter 3 original"]
    for i, content in enumerate(chapters):
        unit_session.add(make_chapter(novel.id, i + 1, content))
    unit_session.flush()

    revised = ["chapter 1 revised", "chapter 2 revised", "chapter 3 revised"]
    complete_revision(unit_session, novel.id, revised)

    refreshed = unit_session.get(Novel, novel.id)
    assert refreshed.status == "novel_pending_review", (
        f"After revision, status should be 'novel_pending_review', got '{refreshed.status}'"
    )


def test_revision_completion_updates_word_count(unit_session):
    \"\"\"
    Requirements 7.3: After revision, word_count should equal sum of revised chapter lengths.
    \"\"\"
    outline = make_outline(status="in_use")
    unit_session.add(outline)
    unit_session.flush()

    novel = make_novel(outline_id=outline.id, status="revising", revision_round=1)
    unit_session.add(novel)
    unit_session.flush()

    original_chapters = ["original content 1", "original content 2"]
    for i, content in enumerate(original_chapters):
        unit_session.add(make_chapter(novel.id, i + 1, content))
    unit_session.flush()

    revised_chapters = ["revised content chapter one longer", "revised content chapter two"]
    complete_revision(unit_session, novel.id, revised_chapters)

    refreshed = unit_session.get(Novel, novel.id)
    expected_wc = sum(len(c) for c in revised_chapters)
    assert refreshed.word_count == expected_wc, (
        f"word_count should be {expected_wc}, got {refreshed.word_count}"
    )


def test_revision_updates_chapter_content(unit_session):
    \"\"\"
    Requirements 7.2: Each chapter's content should be updated after revision.
    \"\"\"
    outline = make_outline(status="in_use")
    unit_session.add(outline)
    unit_session.flush()

    novel = make_novel(outline_id=outline.id, status="revising", revision_round=1)
    unit_session.add(novel)
    unit_session.flush()

    original_content = "original chapter content"
    chapter = make_chapter(novel.id, 1, original_content)
    unit_session.add(chapter)
    unit_session.flush()

    revised_content = "revised chapter content with more detail"
    complete_revision(unit_session, novel.id, [revised_content])

    refreshed_chapter = unit_session.get(NovelChapter, chapter.id)
    assert refreshed_chapter.content == revised_content, (
        f"Chapter content should be updated to revised content"
    )
    assert refreshed_chapter.word_count == len(revised_content), (
        f"Chapter word_count should match revised content length"
    )


def test_revision_history_ordered_by_round(unit_session):
    \"\"\"
    Requirements 8.2: get_revision_history should return records ordered by revision_round ascending.
    \"\"\"
    outline = make_outline(status="in_use")
    unit_session.add(outline)
    unit_session.flush()

    novel = make_novel(
        outline_id=outline.id,
        status="novel_pending_review",
        revision_round=0,
    )
    unit_session.add(novel)
    unit_session.flush()

    # Add chapters
    unit_session.add(make_chapter(novel.id, 1, "chapter 1"))
    unit_session.flush()

    # Apply 3 revision rounds
    for i in range(3):
        n = unit_session.get(Novel, novel.id)
        n.status = "novel_pending_review"
        unit_session.flush()
        apply_revision_round(unit_session, novel.id, f"revision {i+1}")

    # Query history ordered by revision_round
    from sqlalchemy import select as sa_select
    histories = (
        unit_session.query(NovelRevisionHistory)
        .filter(NovelRevisionHistory.novel_id == novel.id)
        .order_by(NovelRevisionHistory.revision_round.asc())
        .all()
    )

    assert len(histories) == 3, f"Should have 3 history records, got {len(histories)}"
    rounds = [h.revision_round for h in histories]
    assert rounds == [1, 2, 3], f"Rounds should be [1, 2, 3], got {rounds}"


def test_revision_history_snapshot_contains_correct_instructions(unit_session):
    \"\"\"
    Requirements 8.1: Revision history should record the correct revision_instructions.
    \"\"\"
    outline = make_outline(status="in_use")
    unit_session.add(outline)
    unit_session.flush()

    novel = make_novel(
        outline_id=outline.id,
        status="novel_pending_review",
        revision_round=0,
    )
    unit_session.add(novel)
    unit_session.flush()

    unit_session.add(make_chapter(novel.id, 1, "chapter content"))
    unit_session.flush()

    instructions = "Please improve the pacing in chapter 1"
    apply_revision_round(unit_session, novel.id, instructions)

    history = (
        unit_session.query(NovelRevisionHistory)
        .filter(NovelRevisionHistory.novel_id == novel.id)
        .first()
    )
    assert history is not None
    assert history.revision_instructions == instructions
    assert history.revision_round == 1


def test_apply_revision_triggers_celery_task(monkeypatch):
    \"\"\"
    Requirements 7.1: apply_revision should trigger the Celery task_revise_novel task.
    \"\"\"
    import asyncio
    from unittest.mock import MagicMock, patch

    sent_tasks = []

    mock_celery_app = MagicMock()
    mock_celery_app.send_task = lambda name, args=None, **kwargs: sent_tasks.append(
        {"name": name, "args": args}
    )

    with patch(
        "ai_novel_studio.pipeline.outline_tasks.app",
        mock_celery_app,
    ):
        from ai_novel_studio.services.novel_revision import NovelRevisionService

        service = NovelRevisionService()
        novel_id = str(uuid.uuid4())
        instructions = "Please revise chapter 2"
        revision_round = 1

        asyncio.run(service.apply_revision(novel_id, instructions, revision_round))

    assert len(sent_tasks) == 1, f"Should have sent 1 task, got {len(sent_tasks)}"
    task = sent_tasks[0]
    assert "task_revise_novel" in task["name"], (
        f"Task name should contain 'task_revise_novel', got {task['name']}"
    )
    assert task["args"] == [novel_id, instructions, revision_round], (
        f"Task args mismatch: {task['args']}"
    )


def test_get_revision_history_returns_empty_for_no_history(monkeypatch):
    \"\"\"
    Requirements 8.2: get_revision_history should return empty list when no history exists.
    \"\"\"
    import asyncio
    from unittest.mock import AsyncMock, MagicMock, patch

    mock_session = MagicMock()
    mock_result = MagicMock()
    mock_result.scalars.return_value.all.return_value = []
    mock_session.execute = AsyncMock(return_value=mock_result)
    mock_session.__aenter__ = AsyncMock(return_value=mock_session)
    mock_session.__aexit__ = AsyncMock(return_value=None)

    mock_session_local = MagicMock(return_value=mock_session)

    with patch(
        "ai_novel_studio.services.novel_revision.AsyncSessionLocal",
        mock_session_local,
    ):
        from ai_novel_studio.services.novel_revision import NovelRevisionService

        service = NovelRevisionService()
        result = asyncio.run(service.get_revision_history(str(uuid.uuid4())))

    assert result == [], f"Should return empty list, got {result}"
"""

with open('ai_novel_studio/tests/test_novel_revision.py', 'w', encoding='utf-8') as f:
    f.write(content)
print('done')

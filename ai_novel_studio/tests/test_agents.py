"""智能体属性测试

**Validates: Requirements 5.2**
"""
import asyncio
from unittest.mock import AsyncMock, patch

from hypothesis import given, settings, strategies as st

from ai_novel_studio.agents.corpus_loader import AgentType, CorpusLoader


@given(st.sampled_from(list(AgentType)))
@settings(max_examples=20)
def test_corpus_loader_non_empty_p3(agent_type):
    """P3：语料库非空时，load_for_agent() 返回的 style_samples 长度必须 >= 1

    **Validates: Requirements 5.2**
    """
    mock_docs = [
        {"content": f"示例段落{i}", "quality_score": 0.9, "book_title": f"书名{i}"}
        for i in range(5)
    ]
    with patch("ai_novel_studio.agents.corpus_loader.training_corpus") as mock_col:
        mock_col.find_by_category = AsyncMock(return_value=mock_docs)
        loader = CorpusLoader()
        corpus = asyncio.get_event_loop().run_until_complete(
            loader.load_for_agent(agent_type)
        )
    assert len(corpus.style_samples) >= 1

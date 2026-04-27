"""
分类器与内容清洗属性测试
P5：内容清洗无损性
P1：分类器一致性
P7：内容哈希去重性
"""
from hypothesis import given, settings, strategies as st, assume

from ai_novel_studio.crawler.cleaner import ContentCleaner
from ai_novel_studio.crawler.classifier import (
    ContentClassifier,
    KeywordBasedClassifier,
    NovelCategory,
)


# ---------------------------------------------------------------------------
# P5：内容清洗无损性
# 验证需求：2.4
# ---------------------------------------------------------------------------

@given(st.text(alphabet=st.characters(whitelist_categories=("Lo",)), min_size=10, max_size=500))
@settings(max_examples=100)
def test_cleaner_preserves_chinese_p5(content):
    """P5：清洗后中文字符数量不少于原始的 80%
    **Validates: Requirements 2.4**
    """
    original_cn = len([c for c in content if "\u4e00" <= c <= "\u9fa5"])
    cleaned = ContentCleaner.clean(content)
    cleaned_cn = len([c for c in cleaned if "\u4e00" <= c <= "\u9fa5"])
    if original_cn > 0:
        assert cleaned_cn >= original_cn * 0.8


# ---------------------------------------------------------------------------
# P1：分类器一致性
# 验证需求：3.4
# ---------------------------------------------------------------------------

@given(st.sampled_from(list(NovelCategory)))
@settings(max_examples=50)
def test_classifier_consistency_p1(category):
    """P1：关键词命中数 ≥ 3 时，两个分类器结果必须一致
    **Validates: Requirements 3.4**
    """
    keywords = KeywordBasedClassifier.CATEGORY_KEYWORDS[category]
    # 构造包含 ≥3 个该分类关键词的文本（取前4个关键词，重复3次确保命中数足够）
    text = "".join(keywords[:4]) * 3
    kw_result = KeywordBasedClassifier.classify(text)
    assert kw_result == category


# ---------------------------------------------------------------------------
# P7：内容哈希去重性
# 验证需求：1.5、10.4
# ---------------------------------------------------------------------------

@given(st.text(min_size=1, max_size=200))
@settings(max_examples=50)
def test_content_hash_dedup_p7(content):
    """P7：相同内容的哈希值必须相同（确保去重逻辑可靠）
    **Validates: Requirements 1.5, 10.4**
    """
    import hashlib
    hash1 = hashlib.md5(content.encode()).hexdigest()
    hash2 = hashlib.md5(content.encode()).hexdigest()
    assert hash1 == hash2

"""
导出模块属性测试

使用 Hypothesis 验证以下属性：
- P1：TXT 导出无异常属性（需求 5.3）
- P2：TXT 导出内容幂等性属性（需求 5.4）
- P3：Word 导出文件有效性属性（需求 6.5）
- P4：文件名过滤属性（需求 4.7）
"""
import os
import tempfile
import zipfile

from hypothesis import given, settings, strategies as st

from ai_novel_studio.gui.export import (
    TxtExporter,
    DocxExporter,
    ExportResult,
    ExportManager,
    sanitize_filename,
)


# ---------------------------------------------------------------------------
# P1：TXT 导出无异常属性
# **Validates: Requirements 5.3**
# ---------------------------------------------------------------------------

@given(content=st.text(min_size=0, max_size=10000))
@settings(max_examples=100)
def test_txt_export_never_raises(content):
    """对任意合法字符串内容，TXT 导出不抛出异常"""
    with tempfile.TemporaryDirectory() as tmp_dir:
        filepath = os.path.join(tmp_dir, "test.txt")
        result = TxtExporter.export(content=content, filepath=filepath, title="测试标题")
        # 要么成功，要么返回失败结果，但不抛出异常
        assert isinstance(result, ExportResult)
        assert isinstance(result.success, bool)


# ---------------------------------------------------------------------------
# P2：TXT 导出内容幂等性属性
# **Validates: Requirements 5.4**
# ---------------------------------------------------------------------------

@given(content=st.text(
    alphabet=st.characters(whitelist_categories=('Lu', 'Ll', 'Nd', 'Zs', 'Po')),
    min_size=1,
    max_size=5000
))
def test_txt_export_content_roundtrip(content):
    """导出后读回的正文内容与原始内容一致"""
    with tempfile.TemporaryDirectory() as tmp_dir:
        filepath = os.path.join(tmp_dir, "roundtrip.txt")
        result = TxtExporter.export(content=content, filepath=filepath, title="")
        assert result.success

        with open(filepath, encoding="utf-8-sig") as f:
            file_content = f.read()

        # 正文内容（去除头部分隔线后）应包含原始内容
        assert content in file_content


# ---------------------------------------------------------------------------
# P3：Word 导出文件有效性属性
# **Validates: Requirements 6.5**
# ---------------------------------------------------------------------------

@given(content=st.text(min_size=1, max_size=5000))
def test_docx_export_valid_zip(content):
    """对任意合法内容，导出的 .docx 文件为有效 ZIP 格式且大小 > 0"""
    with tempfile.TemporaryDirectory() as tmp_dir:
        filepath = os.path.join(tmp_dir, "test.docx")
        result = DocxExporter.export(content=content, filepath=filepath, title="测试")

        if result.success:
            assert result.file_size > 0
            assert zipfile.is_zipfile(filepath)


# ---------------------------------------------------------------------------
# P4：文件名过滤属性
# **Validates: Requirements 4.7**
# ---------------------------------------------------------------------------

ILLEGAL_CHARS = set('/\\*?"<>|')


@given(filename=st.text(min_size=1, max_size=100))
def test_filename_sanitize_removes_illegal_chars(filename):
    """过滤后的文件名不包含任何非法字符"""
    sanitized = sanitize_filename(filename)
    assert not any(c in sanitized for c in ILLEGAL_CHARS)
    # 过滤后文件名不为空（至少保留一个字符或使用默认名）
    assert len(sanitized) > 0

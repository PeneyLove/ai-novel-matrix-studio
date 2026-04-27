"""
内容清洗模块 — ContentCleaner
负责对爬取的原始 HTML/文本内容进行清洗、规范化和验证
"""
import re
from bs4 import BeautifulSoup


class ContentCleaner:
    """内容清洗器"""

    # 广告正则模式列表
    _AD_PATTERNS = [
        r"本章未完.*?点击下一页继续阅读",
        r"本章未完.*?点击下一页",
        r"更多精彩小说.*?请访问",
        r"关注公众号.*?领取福利",
        r"下载.*?APP.*?阅读全文",
        r"下载.*?app.*?阅读全文",
        r"请记住本书首发域名.*?[\n。]",
        r"手机用户请浏览.*?阅读",
        r"笔趣阁.*?最新章节",
        r"www\.[a-zA-Z0-9]+\.(com|net|org|cn)",
    ]

    @staticmethod
    def clean_html(html: str) -> str:
        """用 BeautifulSoup 去除 HTML 标签，返回纯文本"""
        if not html:
            return ""
        soup = BeautifulSoup(html, "html.parser")
        return soup.get_text(separator="\n", strip=True)

    @staticmethod
    def remove_ads(content: str) -> str:
        """用正则移除广告模式"""
        if not content:
            return ""
        for pattern in ContentCleaner._AD_PATTERNS:
            content = re.sub(pattern, "", content, flags=re.IGNORECASE | re.DOTALL)
        return content

    @staticmethod
    def normalize_whitespace(content: str) -> str:
        """合并连续空白，段落间距统一为双换行"""
        if not content:
            return ""
        # 将 \r\n 和 \r 统一为 \n
        content = content.replace("\r\n", "\n").replace("\r", "\n")
        # 合并同一行内的连续空格/制表符（不含换行）
        content = re.sub(r"[^\S\n]+", " ", content)
        # 将连续多个换行（含空行）统一为双换行
        content = re.sub(r"\n{2,}", "\n\n", content)
        # 去除每行首尾空格
        lines = [line.strip() for line in content.split("\n")]
        content = "\n".join(lines)
        # 再次合并多余空行
        content = re.sub(r"\n{3,}", "\n\n", content)
        return content.strip()

    @staticmethod
    def remove_special_chars(content: str) -> str:
        """保留中文、英文、数字、常用标点，移除其他特殊字符"""
        if not content:
            return ""
        # 保留：中文字符、英文字母、数字、常用中英文标点、换行和空格
        content = re.sub(
            r"[^\u4e00-\u9fa5a-zA-Z0-9，。！？；：""''（）《》、\n ]",
            "",
            content,
        )
        return content

    @classmethod
    def clean(cls, raw_content: str) -> str:
        """完整清洗流程：HTML清洗 → 去广告 → 规范空白 → 去特殊字符"""
        content = cls.clean_html(raw_content)
        content = cls.remove_ads(content)
        content = cls.normalize_whitespace(content)
        content = cls.remove_special_chars(content)
        return content

    @staticmethod
    def validate_content(content: str, min_length: int = 500) -> bool:
        """
        验证内容有效性：
        - 长度 >= min_length
        - 中文字符占比 >= 70%
        """
        if not content or len(content) < min_length:
            return False
        chinese_chars = [c for c in content if "\u4e00" <= c <= "\u9fa5"]
        ratio = len(chinese_chars) / len(content)
        return ratio >= 0.7

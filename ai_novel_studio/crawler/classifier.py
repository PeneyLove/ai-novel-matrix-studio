"""
内容分类模块 — ContentClassifier（TF-IDF + 朴素贝叶斯）+ KeywordBasedClassifier
"""
import pickle
from enum import Enum
from typing import Dict, List

import jieba
from sklearn.feature_extraction.text import TfidfVectorizer
from sklearn.naive_bayes import MultinomialNB


class NovelCategory(str, Enum):
    """小说分类枚举"""
    FEMALE_REBIRTH = "female_rebirth"  # 女频重生
    MALE_POWER = "male_power"          # 男频爽文
    SUSPENSE = "suspense"              # 悬疑短篇
    ROMANCE = "romance"                # 甜宠


def _jieba_tokenizer(text: str) -> List[str]:
    """jieba 分词函数，供 TfidfVectorizer 使用"""
    return list(jieba.cut(text))


class ContentClassifier:
    """基于 TF-IDF + 朴素贝叶斯的内容分类器"""

    def __init__(self):
        self.vectorizer = TfidfVectorizer(
            tokenizer=_jieba_tokenizer,
            max_features=5000,
            token_pattern=None,  # 使用自定义 tokenizer 时需禁用默认 pattern
        )
        self.classifier = MultinomialNB()
        self.is_trained = False

    def train(self, texts: List[str], labels: List[NovelCategory]) -> None:
        """训练分类器"""
        label_values = [lbl.value if isinstance(lbl, NovelCategory) else lbl for lbl in labels]
        X = self.vectorizer.fit_transform(texts)
        self.classifier.fit(X, label_values)
        self.is_trained = True

    def predict(self, text: str) -> NovelCategory:
        """
        预测分类，对所有有效输入返回合法 NovelCategory 枚举值，不抛异常。
        未训练时回退到 KeywordBasedClassifier。
        """
        try:
            if not self.is_trained:
                return KeywordBasedClassifier.classify(text)
            X = self.vectorizer.transform([text])
            prediction = self.classifier.predict(X)[0]
            return NovelCategory(prediction)
        except Exception:
            return KeywordBasedClassifier.classify(text)

    def predict_proba(self, text: str) -> Dict[NovelCategory, float]:
        """预测各分类概率"""
        if not self.is_trained:
            # 未训练时返回均匀分布
            categories = list(NovelCategory)
            uniform = 1.0 / len(categories)
            return {cat: uniform for cat in categories}
        X = self.vectorizer.transform([text])
        probas = self.classifier.predict_proba(X)[0]
        return {
            NovelCategory(cls): float(prob)
            for cls, prob in zip(self.classifier.classes_, probas)
        }

    def save(self, filepath: str) -> None:
        """保存模型到文件"""
        with open(filepath, "wb") as f:
            pickle.dump(
                {"vectorizer": self.vectorizer, "classifier": self.classifier},
                f,
            )

    def load(self, filepath: str) -> None:
        """从文件加载模型"""
        with open(filepath, "rb") as f:
            data = pickle.load(f)
            self.vectorizer = data["vectorizer"]
            self.classifier = data["classifier"]
            self.is_trained = True


class KeywordBasedClassifier:
    """基于关键词的补充分类器"""

    CATEGORY_KEYWORDS: Dict[NovelCategory, List[str]] = {
        NovelCategory.FEMALE_REBIRTH: [
            "重生", "穿越", "虐渣", "马甲", "大佬", "打脸", "豪门", "霸总",
        ],
        NovelCategory.MALE_POWER: [
            "都市", "异能", "修仙", "系统", "签到", "无敌", "爽文", "龙王",
        ],
        NovelCategory.SUSPENSE: [
            "悬疑", "推理", "侦探", "凶手", "真相", "反转", "密室", "诡异",
        ],
        NovelCategory.ROMANCE: [
            "甜宠", "恋爱", "暖文", "治愈", "校园", "青梅竹马", "霸道总裁", "小娇妻",
        ],
    }

    @classmethod
    def classify(cls, text: str) -> NovelCategory:
        """统计各分类关键词命中数，返回最高分分类"""
        scores: Dict[NovelCategory, int] = {cat: 0 for cat in NovelCategory}
        for category, keywords in cls.CATEGORY_KEYWORDS.items():
            for keyword in keywords:
                scores[category] += text.count(keyword)
        # 若所有分类得分均为 0，默认返回 ROMANCE
        return max(scores, key=lambda c: (scores[c], list(NovelCategory).index(c)))


class ClassificationWriter:
    """将分类结果写入 MySQL corpus_meta 并同步到 MongoDB"""

    async def write_classification(
        self,
        corpus_id: str,
        category: NovelCategory,
        quality_score: float,
    ) -> None:
        """
        将分类结果更新到 MySQL corpus_meta，并同步到 MongoDB。

        Args:
            corpus_id: 语料 UUID（MySQL corpus_meta.id）
            category: 分类结果枚举值
            quality_score: 质量评分 0.0-1.0
        """
        import logging
        logger = logging.getLogger(__name__)

        # 更新 MySQL corpus_meta
        try:
            from ai_novel_studio.storage.mysql import AsyncSessionLocal, CorpusMeta
            from sqlalchemy import update

            async with AsyncSessionLocal() as session:
                stmt = (
                    update(CorpusMeta)
                    .where(CorpusMeta.id == corpus_id)
                    .values(
                        category=category.value,
                        quality_score=quality_score,
                    )
                )
                await session.execute(stmt)
                await session.commit()
                logger.info(
                    "已更新 MySQL corpus_meta: id=%s category=%s score=%.3f",
                    corpus_id,
                    category.value,
                    quality_score,
                )
        except Exception as exc:
            logger.error("更新 MySQL corpus_meta 失败: id=%s error=%s", corpus_id, exc)

        # 同步到 MongoDB raw_corpus（按 mongo_id 更新）
        try:
            from ai_novel_studio.storage.mongo import raw_corpus as raw_col

            col = raw_col._col()
            await col.update_many(
                {"corpus_id": corpus_id},
                {"$set": {"category": category.value, "quality_score": quality_score}},
            )
            logger.info("已同步 MongoDB raw_corpus: corpus_id=%s", corpus_id)
        except Exception as exc:
            logger.error("同步 MongoDB 失败: corpus_id=%s error=%s", corpus_id, exc)

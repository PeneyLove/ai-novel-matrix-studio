"""语料库管理页面"""
from PyQt6.QtWidgets import (
    QWidget, QVBoxLayout, QHBoxLayout, QLabel, QPushButton,
    QTableWidget, QTableWidgetItem, QHeaderView, QComboBox,
    QDoubleSpinBox, QFrame, QMessageBox,
)
from PyQt6.QtCore import Qt
from PyQt6.QtGui import QColor
from sqlalchemy import func

from ai_novel_studio.gui.db import get_session, CorpusMeta
from ai_novel_studio.gui.styles import AGENT_LABELS


class CorpusPage(QWidget):
    def __init__(self):
        super().__init__()
        self._build_ui()
        self.load_data()

    def _build_ui(self):
        layout = QVBoxLayout(self)
        layout.setContentsMargins(24, 20, 24, 20)
        layout.setSpacing(14)

        # 标题栏
        top = QHBoxLayout()
        title = QLabel("语料库管理")
        title.setObjectName("title")
        top.addWidget(title)
        top.addStretch()
        layout.addLayout(top)

        # 统计卡片
        stats_row = QHBoxLayout()
        stats_row.setSpacing(12)
        self._stat_cards = {}
        for key, label, color in [
            ("total",    "总语料数",   "#89b4fa"),
            ("valid",    "有效语料",   "#a6e3a1"),
            ("training", "训练语料",   "#cba6f7"),
            ("suspect",  "疑似侵权",   "#f38ba8"),
        ]:
            card = QFrame()
            card.setObjectName("card")
            v = QVBoxLayout(card)
            v.setContentsMargins(16, 12, 16, 12)
            val = QLabel("—")
            val.setStyleSheet(f"font-size: 24px; font-weight: bold; color: {color};")
            val.setAlignment(Qt.AlignmentFlag.AlignCenter)
            lbl = QLabel(label)
            lbl.setStyleSheet("font-size: 11px; color: #6c7086;")
            lbl.setAlignment(Qt.AlignmentFlag.AlignCenter)
            v.addWidget(val)
            v.addWidget(lbl)
            self._stat_cards[key] = val
            stats_row.addWidget(card)
        layout.addLayout(stats_row)

        # 筛选栏
        filter_row = QHBoxLayout()
        filter_row.setSpacing(10)

        self.cat_filter = QComboBox()
        self.cat_filter.addItem("全部题材", "")
        for k, v in AGENT_LABELS.items():
            self.cat_filter.addItem(v, k)
        self.cat_filter.currentIndexChanged.connect(self.load_data)

        self.type_filter = QComboBox()
        self.type_filter.addItems(["全部类型", "原始语料(raw)", "训练语料(training)"])
        self.type_filter.currentIndexChanged.connect(self.load_data)

        self.min_quality = QDoubleSpinBox()
        self.min_quality.setRange(0.0, 1.0)
        self.min_quality.setSingleStep(0.1)
        self.min_quality.setDecimals(1)
        self.min_quality.setValue(0.0)
        self.min_quality.valueChanged.connect(self.load_data)

        filter_row.addWidget(QLabel("题材："))
        filter_row.addWidget(self.cat_filter)
        filter_row.addWidget(QLabel("类型："))
        filter_row.addWidget(self.type_filter)
        filter_row.addWidget(QLabel("最低质量："))
        filter_row.addWidget(self.min_quality)
        filter_row.addStretch()

        btn_refresh = QPushButton("刷新")
        btn_refresh.setObjectName("secondary")
        btn_refresh.clicked.connect(self.load_data)
        filter_row.addWidget(btn_refresh)
        layout.addLayout(filter_row)

        # 表格
        self.table = QTableWidget()
        self.table.setColumnCount(7)
        self.table.setHorizontalHeaderLabels(["书名", "章节标题", "来源", "题材", "类型", "质量分", "字数"])
        self.table.horizontalHeader().setSectionResizeMode(QHeaderView.ResizeMode.Stretch)
        self.table.setEditTriggers(QTableWidget.EditTrigger.NoEditTriggers)
        self.table.setSelectionBehavior(QTableWidget.SelectionBehavior.SelectRows)
        self.table.verticalHeader().setVisible(False)
        layout.addWidget(self.table)

        self.status_lbl = QLabel("")
        self.status_lbl.setObjectName("subtitle")
        layout.addWidget(self.status_lbl)

    def load_data(self):
        cat = self.cat_filter.currentData()
        type_idx = self.type_filter.currentIndex()
        type_map = {0: None, 1: "raw", 2: "training"}
        corpus_type = type_map.get(type_idx)
        min_q = self.min_quality.value()

        try:
            with get_session() as s:
                # 统计
                total = s.query(func.count(CorpusMeta.id)).scalar() or 0
                valid = s.query(func.count(CorpusMeta.id)).filter(CorpusMeta.is_valid == True).scalar() or 0
                training = s.query(func.count(CorpusMeta.id)).filter(CorpusMeta.corpus_type == "training").scalar() or 0
                suspect = s.query(func.count(CorpusMeta.id)).filter(CorpusMeta.is_copyright_suspect == True).scalar() or 0

                self._stat_cards["total"].setText(str(total))
                self._stat_cards["valid"].setText(str(valid))
                self._stat_cards["training"].setText(str(training))
                self._stat_cards["suspect"].setText(str(suspect))

                # 列表
                q = s.query(CorpusMeta).filter(CorpusMeta.quality_score >= min_q)
                if cat:
                    q = q.filter(CorpusMeta.category == cat)
                if corpus_type:
                    q = q.filter(CorpusMeta.corpus_type == corpus_type)
                items = q.order_by(CorpusMeta.quality_score.desc()).limit(300).all()

            self.table.setRowCount(len(items))
            for i, c in enumerate(items):
                self.table.setItem(i, 0, QTableWidgetItem(c.book_title or ""))
                self.table.setItem(i, 1, QTableWidgetItem(c.chapter_title or ""))
                self.table.setItem(i, 2, QTableWidgetItem(c.source or ""))
                self.table.setItem(i, 3, QTableWidgetItem(AGENT_LABELS.get(c.category, c.category or "")))
                self.table.setItem(i, 4, QTableWidgetItem(c.corpus_type or ""))
                score = float(c.quality_score or 0)
                score_item = QTableWidgetItem(f"{score:.3f}")
                color = "#a6e3a1" if score >= 0.8 else ("#f9e2af" if score >= 0.5 else "#f38ba8")
                score_item.setForeground(QColor(color))
                self.table.setItem(i, 5, score_item)
                self.table.setItem(i, 6, QTableWidgetItem(f"{c.word_count or 0:,}"))

            self.status_lbl.setText(f"显示 {len(items)} 条（共 {total} 条）")
        except Exception as e:
            self.status_lbl.setText(f"加载失败：{e}")

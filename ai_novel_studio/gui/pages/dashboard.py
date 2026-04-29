"""数据看板页面"""
from datetime import datetime, timedelta
from PyQt6.QtWidgets import (
    QWidget, QVBoxLayout, QHBoxLayout, QLabel, QFrame,
    QComboBox, QPushButton,
    QHeaderView, QSizePolicy,
)
from PyQt6.QtCore import Qt, QThread, pyqtSignal

from sqlalchemy import func

from ai_novel_studio.gui.db import get_session, CreationTask
from ai_novel_studio.gui.styles import STAGE_LABELS, STAGE_COLORS


# 生成中阶段列表
GENERATING_STAGES = ("topic_generating", "outline_generating", "content_generating", "polishing")


class LoadThread(QThread):
    done = pyqtSignal(dict)

    def __init__(self, period: str):
        super().__init__()
        self.period = period

    def run(self):
        try:
            now = datetime.utcnow()
            if self.period == "day":
                start = now.replace(hour=0, minute=0, second=0, microsecond=0)
            elif self.period == "week":
                start = (now - timedelta(days=now.weekday())).replace(hour=0, minute=0, second=0, microsecond=0)
            else:
                start = now.replace(day=1, hour=0, minute=0, second=0, microsecond=0)

            with get_session() as s:
                base_q = s.query(CreationTask).filter(CreationTask.created_at >= start)

                total_tasks = base_q.count()
                review_tasks = base_q.filter(CreationTask.stage == "human_review").count()
                done_tasks = base_q.filter(CreationTask.stage == "done").count()
                generating_tasks = (
                    s.query(func.count(CreationTask.id))
                    .filter(
                        CreationTask.created_at >= start,
                        CreationTask.stage.in_(GENERATING_STAGES),
                    )
                    .scalar() or 0
                )

            self.done.emit({
                "stats": {
                    "total_tasks":      total_tasks,
                    "review_tasks":     review_tasks,
                    "done_tasks":       done_tasks,
                    "generating_tasks": generating_tasks,
                },
            })
        except Exception as e:
            self.done.emit({"error": str(e)})


class DashboardPage(QWidget):
    def __init__(self):
        super().__init__()
        self._thread = None
        self._build_ui()
        self.refresh()

    def _build_ui(self):
        layout = QVBoxLayout(self)
        layout.setContentsMargins(24, 20, 24, 20)
        layout.setSpacing(16)

        # 标题栏
        top = QHBoxLayout()
        title = QLabel("内容看板")
        title.setObjectName("title")
        top.addWidget(title)
        top.addStretch()

        self.period_combo = QComboBox()
        self.period_combo.addItems(["今日", "本周", "本月"])
        self.period_combo.currentIndexChanged.connect(self.refresh)
        top.addWidget(QLabel("统计周期："))
        top.addWidget(self.period_combo)

        btn_refresh = QPushButton("刷新")
        btn_refresh.setObjectName("secondary")
        btn_refresh.clicked.connect(self.refresh)
        top.addWidget(btn_refresh)
        layout.addLayout(top)

        # 统计卡片行
        cards_layout = QHBoxLayout()
        cards_layout.setSpacing(12)
        self._stat_labels = {}
        stats = [
            ("total_tasks",      "总任务数",   "#89b4fa"),
            ("review_tasks",     "待审核",     "#f9e2af"),
            ("done_tasks",       "已完成",     "#a6e3a1"),
            ("generating_tasks", "生成中",     "#cba6f7"),
        ]
        for key, label, color in stats:
            card = QFrame()
            card.setObjectName("card")
            card.setMinimumWidth(130)
            v = QVBoxLayout(card)
            v.setContentsMargins(16, 14, 16, 14)
            val_lbl = QLabel("—")
            val_lbl.setObjectName("stat_value")
            val_lbl.setStyleSheet(f"color: {color}; font-size: 26px; font-weight: bold;")
            val_lbl.setAlignment(Qt.AlignmentFlag.AlignCenter)
            lbl = QLabel(label)
            lbl.setObjectName("stat_label")
            lbl.setAlignment(Qt.AlignmentFlag.AlignCenter)
            v.addWidget(val_lbl)
            v.addWidget(lbl)
            self._stat_labels[key] = val_lbl
            cards_layout.addWidget(card)
        layout.addLayout(cards_layout)

        layout.addStretch()

        self.status_lbl = QLabel("")
        self.status_lbl.setObjectName("subtitle")
        layout.addWidget(self.status_lbl)

    def refresh(self):
        period_map = {0: "day", 1: "week", 2: "month"}
        period = period_map.get(self.period_combo.currentIndex(), "day")
        self.status_lbl.setText("加载中...")
        self._thread = LoadThread(period)
        self._thread.done.connect(self._on_loaded)
        self._thread.start()

    def _on_loaded(self, data: dict):
        if "error" in data:
            self.status_lbl.setText(f"加载失败：{data['error']}")
            return

        stats = data["stats"]
        self._stat_labels["total_tasks"].setText(str(stats["total_tasks"]))
        self._stat_labels["review_tasks"].setText(str(stats["review_tasks"]))
        self._stat_labels["done_tasks"].setText(str(stats["done_tasks"]))
        self._stat_labels["generating_tasks"].setText(str(stats["generating_tasks"]))

        self.status_lbl.setText(f"最后更新：{datetime.now().strftime('%H:%M:%S')}")

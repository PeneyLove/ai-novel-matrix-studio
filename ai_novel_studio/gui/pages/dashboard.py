"""数据看板页面"""
from datetime import datetime, timedelta
from PyQt6.QtWidgets import (
    QWidget, QVBoxLayout, QHBoxLayout, QLabel, QFrame,
    QComboBox, QPushButton, QTableWidget, QTableWidgetItem,
    QHeaderView, QSizePolicy,
)
from PyQt6.QtCore import Qt, QThread, pyqtSignal
from PyQt6.QtGui import QColor
from sqlalchemy import func, select, text

from ai_novel_studio.gui.db import get_session, Account, PublishRecord, CreationTask
from ai_novel_studio.gui.styles import STAGE_LABELS, STAGE_COLORS, PLATFORM_LABELS, AGENT_LABELS


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
                start = (now - timedelta(days=now.weekday())).replace(hour=0, minute=0, second=0)
            else:
                start = now.replace(day=1, hour=0, minute=0, second=0)

            with get_session() as s:
                # 统计卡片数据
                total_accounts = s.query(func.count(Account.id)).scalar() or 0
                active_accounts = s.query(func.count(Account.id)).filter(Account.status == "active").scalar() or 0
                total_tasks = s.query(func.count(CreationTask.id)).scalar() or 0
                done_tasks = s.query(func.count(CreationTask.id)).filter(CreationTask.stage == "done").scalar() or 0
                review_tasks = s.query(func.count(CreationTask.id)).filter(CreationTask.stage == "human_review").scalar() or 0
                total_words = s.query(func.sum(PublishRecord.word_count)).filter(
                    PublishRecord.published_at >= start).scalar() or 0
                total_revenue = float(s.query(func.sum(PublishRecord.revenue)).filter(
                    PublishRecord.published_at >= start).scalar() or 0)

                # 各账号发布数据
                rows = s.execute(
                    select(
                        Account.username, Account.platform, Account.agent_type,
                        func.count(PublishRecord.id).label("chapters"),
                        func.sum(PublishRecord.word_count).label("words"),
                        func.sum(PublishRecord.read_count).label("reads"),
                        func.sum(PublishRecord.revenue).label("revenue"),
                    )
                    .join(PublishRecord, PublishRecord.account_id == Account.id, isouter=True)
                    .filter(PublishRecord.published_at >= start)
                    .group_by(Account.id, Account.username, Account.platform, Account.agent_type)
                ).fetchall()

            self.done.emit({
                "stats": {
                    "total_accounts": total_accounts,
                    "active_accounts": active_accounts,
                    "total_tasks": total_tasks,
                    "done_tasks": done_tasks,
                    "review_tasks": review_tasks,
                    "total_words": total_words,
                    "total_revenue": total_revenue,
                },
                "rows": rows,
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
        title = QLabel("数据看板")
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
            ("total_accounts",  "总账号数",   "#cba6f7"),
            ("active_accounts", "活跃账号",   "#a6e3a1"),
            ("total_tasks",     "总任务数",   "#89b4fa"),
            ("review_tasks",    "待审核",     "#f9e2af"),
            ("total_words",     "发布字数",   "#89dceb"),
            ("total_revenue",   "总收益(元)", "#f2cdcd"),
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

        # 账号发布明细表
        lbl = QLabel("账号发布明细")
        lbl.setStyleSheet("font-weight: bold; color: #a6adc8; margin-top: 4px;")
        layout.addWidget(lbl)

        self.table = QTableWidget()
        self.table.setColumnCount(6)
        self.table.setHorizontalHeaderLabels(["账号", "平台", "题材", "发布章节", "发布字数", "收益(元)"])
        self.table.horizontalHeader().setSectionResizeMode(QHeaderView.ResizeMode.Stretch)
        self.table.setEditTriggers(QTableWidget.EditTrigger.NoEditTriggers)
        self.table.setSelectionBehavior(QTableWidget.SelectionBehavior.SelectRows)
        self.table.verticalHeader().setVisible(False)
        layout.addWidget(self.table)

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
        self._stat_labels["total_accounts"].setText(str(stats["total_accounts"]))
        self._stat_labels["active_accounts"].setText(str(stats["active_accounts"]))
        self._stat_labels["total_tasks"].setText(str(stats["total_tasks"]))
        self._stat_labels["review_tasks"].setText(str(stats["review_tasks"]))
        self._stat_labels["total_words"].setText(f"{stats['total_words']:,}")
        self._stat_labels["total_revenue"].setText(f"¥{stats['total_revenue']:.2f}")

        rows = data["rows"]
        self.table.setRowCount(len(rows))
        for i, row in enumerate(rows):
            self.table.setItem(i, 0, QTableWidgetItem(row.username or ""))
            self.table.setItem(i, 1, QTableWidgetItem(PLATFORM_LABELS.get(row.platform, row.platform)))
            self.table.setItem(i, 2, QTableWidgetItem(AGENT_LABELS.get(row.agent_type, row.agent_type)))
            self.table.setItem(i, 3, QTableWidgetItem(str(row.chapters or 0)))
            self.table.setItem(i, 4, QTableWidgetItem(f"{row.words or 0:,}"))
            rev = float(row.revenue or 0)
            self.table.setItem(i, 5, QTableWidgetItem(f"¥{rev:.2f}"))

        self.status_lbl.setText(f"最后更新：{datetime.now().strftime('%H:%M:%S')}")

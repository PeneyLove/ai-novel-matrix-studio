"""创作任务管理页面"""
import uuid
from datetime import datetime
from PyQt6.QtWidgets import (
    QWidget, QVBoxLayout, QHBoxLayout, QLabel, QPushButton,
    QTableWidget, QTableWidgetItem, QHeaderView, QDialog,
    QFormLayout, QLineEdit, QComboBox, QTextEdit, QMessageBox,
    QDialogButtonBox, QSplitter, QFrame,
)
from PyQt6.QtCore import Qt
from PyQt6.QtGui import QColor

from ai_novel_studio.gui.db import get_session, CreationTask
from ai_novel_studio.gui.styles import STAGE_LABELS, STAGE_COLORS, AGENT_LABELS


class TaskDetailDialog(QDialog):
    """任务详情对话框"""

    def __init__(self, parent, task: CreationTask):
        super().__init__(parent)
        self.task = task
        self.setWindowTitle(f"任务详情 — {task.id[:8]}...")
        self.setMinimumSize(560, 420)
        self._build()

    def _build(self):
        layout = QVBoxLayout(self)
        layout.setContentsMargins(20, 20, 20, 20)
        layout.setSpacing(10)

        def row(label, value, color=None):
            h = QHBoxLayout()
            lbl = QLabel(f"{label}：")
            lbl.setFixedWidth(90)
            lbl.setStyleSheet("color: #6c7086;")
            val = QLabel(str(value) if value is not None else "—")
            if color:
                val.setStyleSheet(f"color: {color}; font-weight: bold;")
            h.addWidget(lbl)
            h.addWidget(val)
            h.addStretch()
            return h

        stage = self.task.stage or "pending"
        layout.addLayout(row("任务ID",   self.task.id))
        layout.addLayout(row("智能体",   AGENT_LABELS.get(self.task.agent_type, self.task.agent_type)))
        layout.addLayout(row("当前阶段", STAGE_LABELS.get(stage, stage), STAGE_COLORS.get(stage)))
        layout.addLayout(row("字数",     f"{self.task.word_count:,}" if self.task.word_count else "0"))
        layout.addLayout(row("创建时间", self.task.created_at.strftime("%Y-%m-%d %H:%M") if self.task.created_at else "—"))

        if self.task.reject_reason:
            layout.addLayout(row("拒绝原因", self.task.reject_reason, "#f38ba8"))

        if self.task.topic:
            lbl = QLabel("选题内容：")
            lbl.setStyleSheet("color: #6c7086; margin-top: 8px;")
            layout.addWidget(lbl)
            topic_box = QTextEdit()
            topic_box.setPlainText(self.task.topic)
            topic_box.setReadOnly(True)
            topic_box.setMaximumHeight(120)
            layout.addWidget(topic_box)

        btns = QDialogButtonBox(QDialogButtonBox.StandardButton.Close)
        btns.rejected.connect(self.reject)
        layout.addWidget(btns)


class NewTaskDialog(QDialog):
    """新建任务对话框"""

    def __init__(self, parent=None):
        super().__init__(parent)
        self.setWindowTitle("新建创作任务")
        self.setMinimumWidth(400)
        self._build()

    def _build(self):
        layout = QFormLayout(self)
        layout.setSpacing(12)
        layout.setContentsMargins(20, 20, 20, 20)

        self.agent_type = QComboBox()
        for k, v in AGENT_LABELS.items():
            self.agent_type.addItem(v, k)

        self.trend_data = QTextEdit()
        self.trend_data.setPlaceholderText("输入热榜数据或创作方向（可选）...")
        self.trend_data.setMaximumHeight(100)

        layout.addRow("题材智能体 *", self.agent_type)
        layout.addRow("热榜数据",     self.trend_data)

        btns = QDialogButtonBox(QDialogButtonBox.StandardButton.Ok | QDialogButtonBox.StandardButton.Cancel)
        btns.accepted.connect(self.accept)
        btns.rejected.connect(self.reject)
        layout.addRow(btns)

    def get_data(self) -> dict:
        return {
            "agent_type": self.agent_type.currentData(),
            "trend_data": self.trend_data.toPlainText().strip() or None,
        }


class TasksPage(QWidget):
    def __init__(self):
        super().__init__()
        self._tasks = []
        self._build_ui()
        self.load_data()

    def _build_ui(self):
        layout = QVBoxLayout(self)
        layout.setContentsMargins(24, 20, 24, 20)
        layout.setSpacing(14)

        # 标题栏
        top = QHBoxLayout()
        title = QLabel("创作任务管理")
        title.setObjectName("title")
        top.addWidget(title)
        top.addStretch()

        self.filter_combo = QComboBox()
        self.filter_combo.addItem("全部状态", "")
        for k, v in STAGE_LABELS.items():
            self.filter_combo.addItem(v, k)
        self.filter_combo.currentIndexChanged.connect(self.load_data)
        top.addWidget(QLabel("筛选："))
        top.addWidget(self.filter_combo)

        btn_new = QPushButton("+ 新建任务")
        btn_new.clicked.connect(self._new_task)
        top.addWidget(btn_new)
        btn_refresh = QPushButton("刷新")
        btn_refresh.setObjectName("secondary")
        btn_refresh.clicked.connect(self.load_data)
        top.addWidget(btn_refresh)
        layout.addLayout(top)

        # 表格
        self.table = QTableWidget()
        self.table.setColumnCount(7)
        self.table.setHorizontalHeaderLabels(["任务ID", "智能体", "阶段", "字数", "创建时间", "更新时间", "操作"])
        self.table.horizontalHeader().setSectionResizeMode(QHeaderView.ResizeMode.Stretch)
        self.table.horizontalHeader().setSectionResizeMode(6, QHeaderView.ResizeMode.Fixed)
        self.table.setColumnWidth(6, 180)
        self.table.setEditTriggers(QTableWidget.EditTrigger.NoEditTriggers)
        self.table.setSelectionBehavior(QTableWidget.SelectionBehavior.SelectRows)
        self.table.verticalHeader().setVisible(False)
        layout.addWidget(self.table)

        self.status_lbl = QLabel("")
        self.status_lbl.setObjectName("subtitle")
        layout.addWidget(self.status_lbl)

    def load_data(self):
        stage_filter = self.filter_combo.currentData()
        try:
            with get_session() as s:
                q = s.query(CreationTask).order_by(CreationTask.created_at.desc())
                if stage_filter:
                    q = q.filter(CreationTask.stage == stage_filter)
                tasks = q.limit(200).all()
            self._tasks = tasks
            self.table.setRowCount(len(tasks))
            for i, t in enumerate(tasks):
                self.table.setItem(i, 0, QTableWidgetItem(t.id[:12] + "..."))
                self.table.setItem(i, 1, QTableWidgetItem(AGENT_LABELS.get(t.agent_type, t.agent_type)))

                stage = t.stage or "pending"
                stage_item = QTableWidgetItem(STAGE_LABELS.get(stage, stage))
                stage_item.setForeground(QColor(STAGE_COLORS.get(stage, "#cdd6f4")))
                self.table.setItem(i, 2, stage_item)

                self.table.setItem(i, 3, QTableWidgetItem(f"{t.word_count or 0:,}"))
                self.table.setItem(i, 4, QTableWidgetItem(
                    t.created_at.strftime("%m-%d %H:%M") if t.created_at else ""))
                self.table.setItem(i, 5, QTableWidgetItem(
                    t.updated_at.strftime("%m-%d %H:%M") if t.updated_at else ""))

                btn_widget = QWidget()
                btn_layout = QHBoxLayout(btn_widget)
                btn_layout.setContentsMargins(4, 2, 4, 2)
                btn_layout.setSpacing(6)

                btn_detail = QPushButton("详情")
                btn_detail.setObjectName("secondary")
                btn_detail.setFixedWidth(56)
                btn_detail.clicked.connect(lambda _, idx=i: self._show_detail(idx))
                btn_layout.addWidget(btn_detail)

                if stage == "human_review":
                    btn_approve = QPushButton("通过")
                    btn_approve.setFixedWidth(56)
                    btn_approve.clicked.connect(lambda _, idx=i: self._approve(idx))
                    btn_reject = QPushButton("拒绝")
                    btn_reject.setObjectName("danger")
                    btn_reject.setFixedWidth(56)
                    btn_reject.clicked.connect(lambda _, idx=i: self._reject(idx))
                    btn_layout.addWidget(btn_approve)
                    btn_layout.addWidget(btn_reject)

                self.table.setCellWidget(i, 6, btn_widget)

            self.status_lbl.setText(f"共 {len(tasks)} 条任务")
        except Exception as e:
            self.status_lbl.setText(f"加载失败：{e}")

    def _new_task(self):
        dlg = NewTaskDialog(self)
        if dlg.exec() == QDialog.DialogCode.Accepted:
            data = dlg.get_data()
            try:
                with get_session() as s:
                    task = CreationTask(
                        id=str(uuid.uuid4()),
                        stage="pending",
                        word_count=0,
                        created_at=datetime.utcnow(),
                        updated_at=datetime.utcnow(),
                        **data,
                    )
                    s.add(task)
                self.load_data()
                QMessageBox.information(self, "成功", "任务已创建，请启动后端服务执行流水线。")
            except Exception as e:
                QMessageBox.critical(self, "错误", f"创建失败：{e}")

    def _show_detail(self, idx: int):
        TaskDetailDialog(self, self._tasks[idx]).exec()

    def _approve(self, idx: int):
        task = self._tasks[idx]
        reply = QMessageBox.question(self, "确认", "确定通过审核并进入发布流程？",
                                     QMessageBox.StandardButton.Yes | QMessageBox.StandardButton.No)
        if reply == QMessageBox.StandardButton.Yes:
            self._update_stage(task.id, "publishing")

    def _reject(self, idx: int):
        task = self._tasks[idx]
        from PyQt6.QtWidgets import QInputDialog
        reason, ok = QInputDialog.getText(self, "拒绝原因", "请输入拒绝原因：")
        if ok:
            self._update_stage(task.id, "pending", reject_reason=reason)

    def _update_stage(self, task_id: str, stage: str, **kwargs):
        try:
            with get_session() as s:
                t = s.get(CreationTask, task_id)
                t.stage = stage
                t.updated_at = datetime.utcnow()
                for k, v in kwargs.items():
                    setattr(t, k, v)
            self.load_data()
        except Exception as e:
            QMessageBox.critical(self, "错误", f"操作失败：{e}")

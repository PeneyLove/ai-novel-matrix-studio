"""创作任务管理页面"""
import os
import uuid
from datetime import datetime, date
from PyQt6.QtWidgets import (
    QWidget, QVBoxLayout, QHBoxLayout, QLabel, QPushButton,
    QTableWidget, QTableWidgetItem, QHeaderView, QDialog,
    QFormLayout, QLineEdit, QComboBox, QTextEdit, QMessageBox,
    QDialogButtonBox, QSplitter, QFrame,
    QRadioButton, QButtonGroup, QFileDialog,
)
from PyQt6.QtCore import Qt
from PyQt6.QtGui import QColor

from ai_novel_studio.gui.db import get_session, CreationTask
from ai_novel_studio.gui.styles import STAGE_LABELS, STAGE_COLORS, AGENT_LABELS
from ai_novel_studio.gui.export import ExportManager, sanitize_filename as _sanitize_filename


class ExportDialog(QDialog):
    """导出对话框：选择格式、文件名和保存路径，执行导出操作。"""

    def __init__(self, parent: QWidget, task: CreationTask):
        super().__init__(parent)
        self.task = task
        self.setWindowTitle("导出任务内容")
        self.setMinimumWidth(480)
        self._build()

    def _build(self):
        layout = QVBoxLayout(self)
        layout.setContentsMargins(20, 20, 20, 20)
        layout.setSpacing(12)

        # ---- 格式选择 ----
        fmt_label = QLabel("导出格式：")
        fmt_label.setStyleSheet("color: #6c7086;")
        layout.addWidget(fmt_label)

        fmt_row = QHBoxLayout()
        self._btn_txt = QRadioButton("TXT 文本文件")
        self._btn_docx = QRadioButton("Word 文档（.docx）")
        self._btn_txt.setChecked(True)
        self._fmt_group = QButtonGroup(self)
        self._fmt_group.addButton(self._btn_txt, 0)
        self._fmt_group.addButton(self._btn_docx, 1)
        fmt_row.addWidget(self._btn_txt)
        fmt_row.addWidget(self._btn_docx)
        fmt_row.addStretch()
        layout.addLayout(fmt_row)

        # ---- 文件名输入 ----
        fn_label = QLabel("文件名：")
        fn_label.setStyleSheet("color: #6c7086;")
        layout.addWidget(fn_label)

        self._filename_edit = QLineEdit()
        self._filename_edit.setText(self._default_filename())
        layout.addWidget(self._filename_edit)

        # ---- 保存路径选择 ----
        path_label = QLabel("保存路径：")
        path_label.setStyleSheet("color: #6c7086;")
        layout.addWidget(path_label)

        path_row = QHBoxLayout()
        self._path_edit = QLineEdit()
        self._path_edit.setText(os.path.expanduser("~"))
        self._path_edit.setReadOnly(True)
        btn_browse = QPushButton("浏览")
        btn_browse.setObjectName("secondary")
        btn_browse.setFixedWidth(60)
        btn_browse.clicked.connect(self._browse_path)
        path_row.addWidget(self._path_edit)
        path_row.addWidget(btn_browse)
        layout.addLayout(path_row)

        # ---- 状态提示标签 ----
        self._status_lbl = QLabel("")
        self._status_lbl.setWordWrap(True)
        layout.addWidget(self._status_lbl)

        # ---- 打开文件夹按钮（初始隐藏） ----
        self._btn_open_folder = QPushButton("打开文件夹")
        self._btn_open_folder.setObjectName("secondary")
        self._btn_open_folder.setVisible(False)
        self._btn_open_folder.clicked.connect(self._open_folder)
        layout.addWidget(self._btn_open_folder)

        # ---- 导出 / 取消按钮 ----
        btn_row = QHBoxLayout()
        btn_row.addStretch()
        btn_export = QPushButton("导出")
        btn_export.clicked.connect(self._do_export)
        btn_cancel = QPushButton("取消")
        btn_cancel.setObjectName("secondary")
        btn_cancel.clicked.connect(self.reject)
        btn_row.addWidget(btn_export)
        btn_row.addWidget(btn_cancel)
        layout.addLayout(btn_row)

        # 记录最后导出的文件路径（用于打开文件夹）
        self._last_filepath = ""

    def _default_filename(self) -> str:
        """生成默认文件名：{任务ID前8位}_{题材标签}_{YYYYMMDD}"""
        task_id_prefix = (self.task.id or "")[:8]
        agent_label = AGENT_LABELS.get(self.task.agent_type, self.task.agent_type or "")
        today = date.today().strftime("%Y%m%d")
        raw = f"{task_id_prefix}_{agent_label}_{today}"
        return self.sanitize_filename(raw)

    @staticmethod
    def sanitize_filename(filename: str) -> str:
        """移除非法文件名字符：/ \\ * ? \" < > |，过滤后为空则返回 'export'。"""
        return _sanitize_filename(filename)

    def _browse_path(self):
        """打开文件夹选择对话框。"""
        folder = QFileDialog.getExistingDirectory(
            self, "选择保存路径", self._path_edit.text()
        )
        if folder:
            self._path_edit.setText(folder)

    def _do_export(self):
        """执行导出操作。"""
        filename = self.sanitize_filename(self._filename_edit.text().strip())
        if not filename:
            filename = "export"

        save_dir = self._path_edit.text().strip()
        if not save_dir:
            save_dir = os.path.expanduser("~")

        content = self._get_content(self.task)

        # 根据格式选择后缀和导出方法
        if self._btn_docx.isChecked():
            ext = ".docx"
        else:
            ext = ".txt"

        filepath = os.path.join(save_dir, filename + ext)

        self._status_lbl.setText("⏳ 正在导出...")
        self._status_lbl.setStyleSheet("")
        self._btn_open_folder.setVisible(False)

        try:
            if self._btn_docx.isChecked():
                result = ExportManager.export_docx(
                    content=content,
                    filepath=filepath,
                    title=self.task.topic or "",
                )
            else:
                result = ExportManager.export_txt(
                    content=content,
                    filepath=filepath,
                    title=self.task.topic or "",
                )
        except Exception as exc:
            self._status_lbl.setText(f"❌ 导出失败：{exc}")
            self._status_lbl.setStyleSheet("color: #f38ba8;")
            return

        if result.success:
            size_kb = round(result.file_size / 1024, 1)
            self._status_lbl.setText(
                f"✅ 导出成功！文件路径：{result.filepath}，大小：{size_kb}KB"
            )
            self._status_lbl.setStyleSheet("color: #a6e3a1;")
            self._last_filepath = result.filepath
            self._btn_open_folder.setVisible(True)
        else:
            self._status_lbl.setText(f"❌ 导出失败：{result.error_message}")
            self._status_lbl.setStyleSheet("color: #f38ba8;")

    def _open_folder(self):
        """用系统文件管理器打开文件所在目录（Windows）。"""
        if self._last_filepath:
            folder = os.path.dirname(self._last_filepath)
            try:
                os.startfile(folder)
            except Exception as exc:
                QMessageBox.warning(self, "提示", f"无法打开文件夹：{exc}")

    def _get_content(self, task) -> str:
        """将 task.topic 和 task.outline（JSON 格式化）拼接为导出内容。"""
        import json
        parts = []
        if task.topic:
            parts.append(task.topic)
        if task.outline:
            try:
                if isinstance(task.outline, str):
                    outline_obj = json.loads(task.outline)
                else:
                    outline_obj = task.outline
                parts.append(json.dumps(outline_obj, ensure_ascii=False, indent=2))
            except (json.JSONDecodeError, TypeError):
                parts.append(str(task.outline))
        return "\n\n".join(parts)


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

                if stage in ("done", "human_review"):
                    btn_export = QPushButton("导出")
                    btn_export.setObjectName("secondary")
                    btn_export.setFixedWidth(56)
                    btn_export.clicked.connect(lambda _, idx=i: self._export_task(idx))
                    btn_layout.addWidget(btn_export)

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
        reply = QMessageBox.question(self, "确认", "确定通过审核？任务将进入已完成状态。",
                                     QMessageBox.StandardButton.Yes | QMessageBox.StandardButton.No)
        if reply == QMessageBox.StandardButton.Yes:
            self._update_stage(task.id, "done")

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

    def _export_task(self, idx: int):
        """查询任务内容，检查内容非空，打开 ExportDialog。"""
        task = self._tasks[idx]
        # 检查内容是否均为空
        if not task.topic and not task.outline:
            QMessageBox.warning(self, "提示", "任务内容为空，无法导出")
            return
        ExportDialog(self, task).exec()

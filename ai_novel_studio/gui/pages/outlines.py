"""大纲管理页面"""
import uuid
from datetime import datetime

from PyQt6.QtWidgets import (
    QWidget, QVBoxLayout, QHBoxLayout, QLabel, QPushButton,
    QTableWidget, QTableWidgetItem, QHeaderView, QDialog,
    QFormLayout, QComboBox, QSpinBox, QTextEdit,
    QMessageBox, QDialogButtonBox, QInputDialog, QTabWidget,
)
from PyQt6.QtCore import Qt
from PyQt6.QtGui import QColor

from ai_novel_studio.gui.db import get_session, Outline, OutlineReviewHistory
from ai_novel_studio.gui.styles import AGENT_LABELS


# 大纲状态标签与颜色
OUTLINE_STATUS_LABELS = {
    "pending_review":   "待审核",
    "approved":         "已通过",
    "rejected":         "已拒绝",
    "in_use":           "使用中",
    "used":             "已使用",
    "generation_failed": "生成失败",
}

OUTLINE_STATUS_COLORS = {
    "pending_review":   "#f9e2af",
    "approved":         "#a6e3a1",
    "rejected":         "#f38ba8",
    "in_use":           "#89b4fa",
    "used":             "#6c7086",
    "generation_failed": "#f38ba8",
}

AGENTS = list(AGENT_LABELS.keys())


# ---------------------------------------------------------------------------
# 批量生成大纲对话框
# ---------------------------------------------------------------------------
class BatchGenerateDialog(QDialog):
    """批量生成大纲对话框：选择 agent_type、count、topic_hint"""

    def __init__(self, parent=None):
        super().__init__(parent)
        self.setWindowTitle("批量生成大纲")
        self.setMinimumWidth(400)
        self._build()

    def _build(self):
        layout = QFormLayout(self)
        layout.setSpacing(12)
        layout.setContentsMargins(20, 20, 20, 20)

        self.agent_type = QComboBox()
        for k, v in AGENT_LABELS.items():
            self.agent_type.addItem(v, k)

        self.count = QSpinBox()
        self.count.setRange(1, 10)
        self.count.setValue(3)

        self.topic_hint = QTextEdit()
        self.topic_hint.setPlaceholderText("可选：输入主题提示，例如「都市重生女主逆袭」...")
        self.topic_hint.setMaximumHeight(80)

        layout.addRow("题材智能体 *", self.agent_type)
        layout.addRow("生成数量 *",   self.count)
        layout.addRow("主题提示",     self.topic_hint)

        btns = QDialogButtonBox(
            QDialogButtonBox.StandardButton.Ok | QDialogButtonBox.StandardButton.Cancel
        )
        btns.accepted.connect(self.accept)
        btns.rejected.connect(self.reject)
        layout.addRow(btns)

    def get_data(self) -> dict:
        return {
            "agent_type": self.agent_type.currentData(),
            "count":      self.count.value(),
            "topic_hint": self.topic_hint.toPlainText().strip() or None,
        }


# ---------------------------------------------------------------------------
# 大纲详情对话框
# ---------------------------------------------------------------------------
class OutlineDetailDialog(QDialog):
    """大纲详情对话框：显示完整大纲内容、审核意见、审核历史"""

    def __init__(self, parent, outline: Outline):
        super().__init__(parent)
        self.outline = outline
        title_text = outline.title or outline.id[:12] + "..."
        self.setWindowTitle(f"大纲详情 — {title_text}")
        self.setMinimumSize(680, 520)
        self._build()

    def _build(self):
        layout = QVBoxLayout(self)
        layout.setContentsMargins(20, 16, 20, 16)
        layout.setSpacing(10)

        # 基本信息行
        info_row = QHBoxLayout()

        def info_label(text, value, color=None):
            h = QHBoxLayout()
            lbl = QLabel(f"{text}：")
            lbl.setFixedWidth(80)
            lbl.setStyleSheet("color: #6c7086;")
            val = QLabel(str(value) if value is not None else "—")
            if color:
                val.setStyleSheet(f"color: {color}; font-weight: bold;")
            h.addWidget(lbl)
            h.addWidget(val)
            h.addStretch()
            return h

        status = self.outline.status or "pending_review"
        meta_col = QVBoxLayout()
        meta_col.addLayout(info_label("大纲ID",   self.outline.id[:16] + "..."))
        meta_col.addLayout(info_label("智能体",   AGENT_LABELS.get(self.outline.agent_type, self.outline.agent_type)))
        meta_col.addLayout(info_label("状态",     OUTLINE_STATUS_LABELS.get(status, status),
                                      OUTLINE_STATUS_COLORS.get(status)))
        meta_col.addLayout(info_label("创建时间", self.outline.created_at.strftime("%Y-%m-%d %H:%M")
                                      if self.outline.created_at else "—"))
        if self.outline.reviewer:
            meta_col.addLayout(info_label("审核人", self.outline.reviewer))
        if self.outline.reviewed_at:
            meta_col.addLayout(info_label("审核时间", self.outline.reviewed_at.strftime("%Y-%m-%d %H:%M")))
        info_row.addLayout(meta_col)
        layout.addLayout(info_row)

        # Tab 区域
        tabs = QTabWidget()

        # Tab 1: 大纲内容
        content_tab = QWidget()
        ct_layout = QVBoxLayout(content_tab)
        ct_layout.setContentsMargins(8, 8, 8, 8)
        content_edit = QTextEdit()
        content_edit.setPlainText(self.outline.content or "（暂无内容）")
        content_edit.setReadOnly(True)
        ct_layout.addWidget(content_edit)
        tabs.addTab(content_tab, "大纲内容")

        # Tab 2: 审核意见
        review_tab = QWidget()
        rv_layout = QVBoxLayout(review_tab)
        rv_layout.setContentsMargins(8, 8, 8, 8)
        review_text = QTextEdit()
        review_content = ""
        if self.outline.review_comments:
            review_content += f"审核意见：\n{self.outline.review_comments}\n\n"
        if self.outline.reject_reason:
            review_content += f"拒绝原因：\n{self.outline.reject_reason}\n"
        review_text.setPlainText(review_content or "（暂无审核意见）")
        review_text.setReadOnly(True)
        rv_layout.addWidget(review_text)
        tabs.addTab(review_tab, "审核意见")

        # Tab 3: 审核历史
        history_tab = QWidget()
        ht_layout = QVBoxLayout(history_tab)
        ht_layout.setContentsMargins(8, 8, 8, 8)
        history_table = QTableWidget()
        history_table.setColumnCount(5)
        history_table.setHorizontalHeaderLabels(["时间", "操作人", "变更前", "变更后", "备注"])
        history_table.horizontalHeader().setSectionResizeMode(QHeaderView.ResizeMode.Stretch)
        history_table.setEditTriggers(QTableWidget.EditTrigger.NoEditTriggers)
        history_table.verticalHeader().setVisible(False)
        history_table.setAlternatingRowColors(True)

        try:
            with get_session() as s:
                histories = (
                    s.query(OutlineReviewHistory)
                    .filter(OutlineReviewHistory.outline_id == self.outline.id)
                    .order_by(OutlineReviewHistory.created_at.asc())
                    .all()
                )
            history_table.setRowCount(len(histories))
            for i, h in enumerate(histories):
                history_table.setItem(i, 0, QTableWidgetItem(
                    h.created_at.strftime("%Y-%m-%d %H:%M") if h.created_at else ""))
                history_table.setItem(i, 1, QTableWidgetItem(h.operator or ""))
                from_lbl = OUTLINE_STATUS_LABELS.get(h.from_status or "", h.from_status or "")
                to_lbl   = OUTLINE_STATUS_LABELS.get(h.to_status or "",   h.to_status or "")
                history_table.setItem(i, 2, QTableWidgetItem(from_lbl))
                to_item = QTableWidgetItem(to_lbl)
                to_item.setForeground(QColor(OUTLINE_STATUS_COLORS.get(h.to_status or "", "#cdd6f4")))
                history_table.setItem(i, 3, to_item)
                history_table.setItem(i, 4, QTableWidgetItem(h.remark or ""))
        except Exception as e:
            history_table.setRowCount(1)
            history_table.setItem(0, 0, QTableWidgetItem(f"加载失败：{e}"))

        ht_layout.addWidget(history_table)
        tabs.addTab(history_tab, "审核历史")

        layout.addWidget(tabs)

        btns = QDialogButtonBox(QDialogButtonBox.StandardButton.Close)
        btns.rejected.connect(self.reject)
        layout.addWidget(btns)


# ---------------------------------------------------------------------------
# 大纲管理主页面
# ---------------------------------------------------------------------------
class OutlinesPage(QWidget):
    def __init__(self):
        super().__init__()
        self._outlines = []
        self._build_ui()
        self.load_data()

    def _build_ui(self):
        layout = QVBoxLayout(self)
        layout.setContentsMargins(24, 20, 24, 20)
        layout.setSpacing(14)

        # ── 顶部工具栏 ──────────────────────────────────────────
        top = QHBoxLayout()
        title = QLabel("大纲管理")
        title.setObjectName("title")
        top.addWidget(title)
        top.addStretch()

        # 状态筛选下拉框
        top.addWidget(QLabel("状态筛选："))
        self.status_filter = QComboBox()
        self.status_filter.addItem("全部状态", "")
        for k, v in OUTLINE_STATUS_LABELS.items():
            self.status_filter.addItem(v, k)
        self.status_filter.currentIndexChanged.connect(self.load_data)
        top.addWidget(self.status_filter)

        # 刷新按钮
        btn_refresh = QPushButton("刷新")
        btn_refresh.setObjectName("secondary")
        btn_refresh.clicked.connect(self.load_data)
        top.addWidget(btn_refresh)

        # 批量生成大纲按钮
        btn_generate = QPushButton("+ 批量生成大纲")
        btn_generate.clicked.connect(self._batch_generate)
        top.addWidget(btn_generate)

        layout.addLayout(top)

        # ── 大纲列表表格 ─────────────────────────────────────────
        self.table = QTableWidget()
        self.table.setColumnCount(7)
        self.table.setHorizontalHeaderLabels([
            "ID", "标题", "智能体类型", "状态", "创建时间", "批次ID", "操作"
        ])
        hdr = self.table.horizontalHeader()
        hdr.setSectionResizeMode(QHeaderView.ResizeMode.Stretch)
        hdr.setSectionResizeMode(6, QHeaderView.ResizeMode.Fixed)
        self.table.setColumnWidth(6, 200)
        self.table.setEditTriggers(QTableWidget.EditTrigger.NoEditTriggers)
        self.table.setSelectionBehavior(QTableWidget.SelectionBehavior.SelectRows)
        self.table.verticalHeader().setVisible(False)
        self.table.setAlternatingRowColors(True)
        layout.addWidget(self.table)

        self.status_lbl = QLabel("")
        self.status_lbl.setObjectName("subtitle")
        layout.addWidget(self.status_lbl)

    # ── 数据加载 ─────────────────────────────────────────────────
    def load_data(self):
        status_filter = self.status_filter.currentData()
        try:
            with get_session() as s:
                q = s.query(Outline).order_by(Outline.created_at.desc())
                if status_filter:
                    q = q.filter(Outline.status == status_filter)
                outlines = q.limit(300).all()
            self._outlines = outlines
            self._populate_table(outlines)
            self.status_lbl.setText(f"共 {len(outlines)} 条大纲")
        except Exception as e:
            self.status_lbl.setText(f"加载失败：{e}")

    def _populate_table(self, outlines):
        self.table.setRowCount(len(outlines))
        for i, o in enumerate(outlines):
            self.table.setItem(i, 0, QTableWidgetItem((o.id or "")[:12] + "..."))
            self.table.setItem(i, 1, QTableWidgetItem(o.title or "（生成中）"))
            self.table.setItem(i, 2, QTableWidgetItem(AGENT_LABELS.get(o.agent_type, o.agent_type or "")))

            status = o.status or "pending_review"
            status_item = QTableWidgetItem(OUTLINE_STATUS_LABELS.get(status, status))
            status_item.setForeground(QColor(OUTLINE_STATUS_COLORS.get(status, "#cdd6f4")))
            self.table.setItem(i, 3, status_item)

            self.table.setItem(i, 4, QTableWidgetItem(
                o.created_at.strftime("%m-%d %H:%M") if o.created_at else ""))
            self.table.setItem(i, 5, QTableWidgetItem((o.batch_id or "")[:12] + "..."))

            # 操作列
            btn_widget = QWidget()
            btn_layout = QHBoxLayout(btn_widget)
            btn_layout.setContentsMargins(4, 2, 4, 2)
            btn_layout.setSpacing(4)

            # 详情按钮（所有大纲都有）
            btn_detail = QPushButton("详情")
            btn_detail.setObjectName("secondary")
            btn_detail.setFixedWidth(52)
            btn_detail.clicked.connect(lambda _, idx=i: self._show_detail(idx))
            btn_layout.addWidget(btn_detail)

            # 待审核大纲显示"通过"和"拒绝"按钮
            if status == "pending_review":
                btn_approve = QPushButton("通过")
                btn_approve.setFixedWidth(52)
                btn_approve.clicked.connect(lambda _, idx=i: self._approve(idx))
                btn_layout.addWidget(btn_approve)

                btn_reject = QPushButton("拒绝")
                btn_reject.setObjectName("danger")
                btn_reject.setFixedWidth(52)
                btn_reject.clicked.connect(lambda _, idx=i: self._reject(idx))
                btn_layout.addWidget(btn_reject)

            self.table.setCellWidget(i, 6, btn_widget)

    # ── 操作处理 ─────────────────────────────────────────────────
    def _batch_generate(self):
        """弹出批量生成对话框，创建大纲记录并触发后端任务"""
        dlg = BatchGenerateDialog(self)
        if dlg.exec() != QDialog.DialogCode.Accepted:
            return

        data = dlg.get_data()
        agent_type = data["agent_type"]
        count      = data["count"]
        topic_hint = data["topic_hint"]

        try:
            import requests as _requests
            resp = _requests.post(
                "http://localhost:8000/outlines/batch_generate",
                json={
                    "agent_type": agent_type,
                    "count":      count,
                    "topic_hint": topic_hint,
                },
                timeout=15,
            )
            if resp.status_code == 200:
                result = resp.json()
                QMessageBox.information(
                    self, "成功",
                    f"已提交批量生成请求\n"
                    f"批次ID：{result.get('batch_id', '')[:16]}...\n"
                    f"共创建 {result.get('total', count)} 条大纲记录",
                )
                self.load_data()
            else:
                detail = resp.json().get("detail", resp.text)
                QMessageBox.warning(self, "请求失败", f"后端返回错误：{detail}")
        except Exception as e:
            # 后端不可用时，直接在本地创建占位记录
            self._create_outlines_locally(agent_type, count, topic_hint)

    def _create_outlines_locally(self, agent_type: str, count: int, topic_hint):
        """后端不可用时，在本地数据库创建占位大纲记录"""
        batch_id = str(uuid.uuid4())
        try:
            with get_session() as s:
                for _ in range(count):
                    outline = Outline(
                        id=str(uuid.uuid4()),
                        agent_type=agent_type,
                        batch_id=batch_id,
                        title=None,
                        content="",
                        topic_hint=topic_hint,
                        status="pending_review",
                        created_at=datetime.utcnow(),
                        updated_at=datetime.utcnow(),
                    )
                    s.add(outline)
            QMessageBox.information(
                self, "成功（本地模式）",
                f"已在本地创建 {count} 条大纲记录（批次ID：{batch_id[:16]}...）\n"
                "请启动后端服务以触发 AI 生成任务。",
            )
            self.load_data()
        except Exception as e:
            QMessageBox.critical(self, "错误", f"创建失败：{e}")

    def _show_detail(self, idx: int):
        """显示大纲详情对话框"""
        outline = self._outlines[idx]
        # 重新从数据库加载最新数据
        try:
            with get_session() as s:
                fresh = s.get(Outline, outline.id)
                if fresh is None:
                    QMessageBox.warning(self, "提示", "大纲记录不存在")
                    return
                # 在 session 关闭前读取所有需要的属性
                outline_data = Outline()
                for col in ["id", "agent_type", "batch_id", "title", "content",
                            "topic_hint", "trend_data", "status", "reviewer",
                            "review_comments", "reject_reason", "reviewed_at",
                            "novel_id", "created_at", "updated_at"]:
                    setattr(outline_data, col, getattr(fresh, col, None))
        except Exception:
            outline_data = outline

        OutlineDetailDialog(self, outline_data).exec()

    def _approve(self, idx: int):
        """审核通过"""
        outline = self._outlines[idx]
        reply = QMessageBox.question(
            self, "确认通过",
            f"确定通过大纲「{outline.title or outline.id[:12]}」的审核？\n通过后大纲将进入大纲池。",
            QMessageBox.StandardButton.Yes | QMessageBox.StandardButton.No,
        )
        if reply != QMessageBox.StandardButton.Yes:
            return

        # 尝试调用后端 API
        try:
            import requests as _requests
            resp = _requests.post(
                "http://localhost:8000/outlines/review_decision",
                json={
                    "outline_id": outline.id,
                    "decision":   "approve",
                    "reviewer":   "GUI用户",
                    "comments":   None,
                },
                timeout=10,
            )
            if resp.status_code == 200:
                self.load_data()
                return
            elif resp.status_code == 409:
                QMessageBox.warning(self, "状态冲突", resp.json().get("detail", "状态冲突"))
                return
            elif resp.status_code == 404:
                QMessageBox.warning(self, "不存在", resp.json().get("detail", "大纲不存在"))
                return
        except Exception:
            pass

        # 后端不可用时直接操作数据库
        self._update_outline_status(outline.id, "approved", reviewer="GUI用户")

    def _reject(self, idx: int):
        """审核拒绝，弹出输入框填写拒绝原因"""
        outline = self._outlines[idx]
        reason, ok = QInputDialog.getText(
            self, "拒绝原因",
            f"请输入拒绝大纲「{outline.title or outline.id[:12]}」的原因：",
        )
        if not ok:
            return
        if not reason.strip():
            QMessageBox.warning(self, "提示", "拒绝原因不能为空")
            return

        # 尝试调用后端 API
        try:
            import requests as _requests
            resp = _requests.post(
                "http://localhost:8000/outlines/review_decision",
                json={
                    "outline_id": outline.id,
                    "decision":   "reject",
                    "reviewer":   "GUI用户",
                    "reason":     reason.strip(),
                },
                timeout=10,
            )
            if resp.status_code == 200:
                self.load_data()
                return
            elif resp.status_code == 409:
                QMessageBox.warning(self, "状态冲突", resp.json().get("detail", "状态冲突"))
                return
            elif resp.status_code == 404:
                QMessageBox.warning(self, "不存在", resp.json().get("detail", "大纲不存在"))
                return
        except Exception:
            pass

        # 后端不可用时直接操作数据库
        self._update_outline_status(
            outline.id, "rejected",
            reviewer="GUI用户",
            reject_reason=reason.strip(),
        )

    def _update_outline_status(
        self,
        outline_id: str,
        new_status: str,
        reviewer: str = "GUI用户",
        reject_reason: str = None,
        review_comments: str = None,
    ):
        """直接更新数据库中的大纲状态（后端不可用时的降级方案）"""
        try:
            with get_session() as s:
                o = s.get(Outline, outline_id)
                if o is None:
                    QMessageBox.warning(self, "提示", "大纲记录不存在")
                    return
                old_status = o.status
                o.status = new_status
                o.reviewer = reviewer
                o.reviewed_at = datetime.utcnow()
                o.updated_at = datetime.utcnow()
                if reject_reason:
                    o.reject_reason = reject_reason
                if review_comments:
                    o.review_comments = review_comments

                # 写入审核历史
                history = OutlineReviewHistory(
                    outline_id=outline_id,
                    from_status=old_status,
                    to_status=new_status,
                    operator=reviewer,
                    remark=reject_reason or review_comments or "",
                    created_at=datetime.utcnow(),
                )
                s.add(history)
            self.load_data()
        except Exception as e:
            QMessageBox.critical(self, "错误", f"操作失败：{e}")

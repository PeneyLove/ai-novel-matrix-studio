"""小说管理页面"""
import uuid
from datetime import datetime

from PyQt6.QtWidgets import (
    QWidget, QVBoxLayout, QHBoxLayout, QLabel, QPushButton,
    QTableWidget, QTableWidgetItem, QHeaderView, QDialog,
    QFormLayout, QComboBox, QTextEdit,
    QMessageBox, QDialogButtonBox, QInputDialog, QTabWidget,
)
from PyQt6.QtCore import Qt
from PyQt6.QtGui import QColor

from ai_novel_studio.gui.db import get_session, Novel, NovelChapter, NovelRevisionHistory, Outline
from ai_novel_studio.gui.styles import AGENT_LABELS


# 小说状态标签与颜色
NOVEL_STATUS_LABELS = {
    "writing":               "编写中",
    "novel_pending_review":  "待审核",
    "novel_approved":        "已通过",
    "novel_rejected":        "已拒绝",
    "revising":              "修改中",
    "publishing":            "发布中",
    "done":                  "已完成",
}

NOVEL_STATUS_COLORS = {
    "writing":               "#89dceb",
    "novel_pending_review":  "#f9e2af",
    "novel_approved":        "#a6e3a1",
    "novel_rejected":        "#f38ba8",
    "revising":              "#89b4fa",
    "publishing":            "#cba6f7",
    "done":                  "#a6e3a1",
}

AGENTS = list(AGENT_LABELS.keys())


# ---------------------------------------------------------------------------
# 从大纲池创建小说对话框
# ---------------------------------------------------------------------------
class CreateNovelDialog(QDialog):
    """从大纲池创建小说对话框：选择大纲和 agent_type"""

    def __init__(self, parent=None):
        super().__init__(parent)
        self.setWindowTitle("从大纲池创建小说")
        self.setMinimumWidth(480)
        self._outlines = []
        self._build()
        self._load_outlines()

    def _build(self):
        layout = QFormLayout(self)
        layout.setSpacing(12)
        layout.setContentsMargins(20, 20, 20, 20)

        self.outline_combo = QComboBox()
        self.outline_combo.setMinimumWidth(320)

        self.agent_type = QComboBox()
        for k, v in AGENT_LABELS.items():
            self.agent_type.addItem(v, k)

        layout.addRow("选择大纲 *",   self.outline_combo)
        layout.addRow("题材智能体 *", self.agent_type)

        btns = QDialogButtonBox(
            QDialogButtonBox.StandardButton.Ok | QDialogButtonBox.StandardButton.Cancel
        )
        btns.accepted.connect(self.accept)
        btns.rejected.connect(self.reject)
        layout.addRow(btns)

    def _load_outlines(self):
        """加载大纲池中可用的大纲（status=approved）"""
        try:
            with get_session() as s:
                outlines = (
                    s.query(Outline)
                    .filter(Outline.status == "approved")
                    .order_by(Outline.created_at.desc())
                    .limit(100)
                    .all()
                )
            self._outlines = outlines
            self.outline_combo.clear()
            if not outlines:
                self.outline_combo.addItem("（大纲池为空，请先审核通过大纲）", None)
            else:
                for o in outlines:
                    label = f"{o.title or '（无标题）'} [{AGENT_LABELS.get(o.agent_type, o.agent_type)}]"
                    self.outline_combo.addItem(label, o.id)
        except Exception as e:
            self.outline_combo.addItem(f"加载失败：{e}", None)

    def get_data(self) -> dict:
        return {
            "outline_id": self.outline_combo.currentData(),
            "agent_type": self.agent_type.currentData(),
        }


# ---------------------------------------------------------------------------
# 修改意见对话框
# ---------------------------------------------------------------------------
class RevisionDialog(QDialog):
    """修改意见对话框：输入修改指令（非空校验）"""

    def __init__(self, parent=None, novel_title: str = ""):
        super().__init__(parent)
        self.setWindowTitle(f"提交修改意见 — {novel_title}")
        self.setMinimumWidth(480)
        self.setMinimumHeight(280)
        self._build()

    def _build(self):
        layout = QVBoxLayout(self)
        layout.setContentsMargins(20, 16, 20, 16)
        layout.setSpacing(10)

        hint = QLabel("请输入修改指令（必填）：")
        hint.setStyleSheet("color: #a6adc8;")
        layout.addWidget(hint)

        self.instructions = QTextEdit()
        self.instructions.setPlaceholderText(
            "例如：第三章节奏太慢，需要加快；主角性格需要更加鲜明；结局需要更有冲击力..."
        )
        layout.addWidget(self.instructions)

        btns = QDialogButtonBox(
            QDialogButtonBox.StandardButton.Ok | QDialogButtonBox.StandardButton.Cancel
        )
        btns.accepted.connect(self._on_accept)
        btns.rejected.connect(self.reject)
        layout.addWidget(btns)

    def _on_accept(self):
        if not self.instructions.toPlainText().strip():
            QMessageBox.warning(self, "提示", "修改指令不能为空")
            return
        self.accept()

    def get_instructions(self) -> str:
        return self.instructions.toPlainText().strip()


# ---------------------------------------------------------------------------
# 章节内容对话框
# ---------------------------------------------------------------------------
class ChapterContentDialog(QDialog):
    """显示单章节内容的子对话框"""

    def __init__(self, parent, chapter: NovelChapter):
        super().__init__(parent)
        chapter_no = chapter.chapter_no or 0
        chapter_title = chapter.chapter_title or f"第{chapter_no}章"
        self.setWindowTitle(f"章节内容 — {chapter_title}")
        self.setMinimumSize(640, 500)
        self._build(chapter)

    def _build(self, chapter: NovelChapter):
        layout = QVBoxLayout(self)
        layout.setContentsMargins(20, 16, 20, 16)
        layout.setSpacing(10)

        # 章节信息行
        info_row = QHBoxLayout()
        info_row.addWidget(QLabel(f"章节序号：{chapter.chapter_no}"))
        info_row.addWidget(QLabel(f"字数：{chapter.word_count or 0}"))
        status_lbl = QLabel(f"状态：{'已定稿' if chapter.status == 'finalized' else '草稿'}")
        info_row.addWidget(status_lbl)
        info_row.addStretch()
        layout.addLayout(info_row)

        content_edit = QTextEdit()
        content_edit.setPlainText(chapter.content or "（暂无内容）")
        content_edit.setReadOnly(True)
        layout.addWidget(content_edit)

        btns = QDialogButtonBox(QDialogButtonBox.StandardButton.Close)
        btns.rejected.connect(self.reject)
        layout.addWidget(btns)


# ---------------------------------------------------------------------------
# 小说详情对话框
# ---------------------------------------------------------------------------
class NovelDetailDialog(QDialog):
    """小说详情对话框：显示小说基本信息、章节列表（可点击查看章节内容）"""

    def __init__(self, parent, novel: Novel):
        super().__init__(parent)
        self.novel = novel
        title_text = novel.title or novel.id[:12] + "..."
        self.setWindowTitle(f"小说详情 — {title_text}")
        self.setMinimumSize(720, 560)
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

        status = self.novel.status or "writing"
        meta_col = QVBoxLayout()
        meta_col.addLayout(info_label("小说ID",   self.novel.id[:16] + "..."))
        meta_col.addLayout(info_label("智能体",   AGENT_LABELS.get(self.novel.agent_type, self.novel.agent_type)))
        meta_col.addLayout(info_label("状态",     NOVEL_STATUS_LABELS.get(status, status),
                                      NOVEL_STATUS_COLORS.get(status)))
        meta_col.addLayout(info_label("字数",     f"{self.novel.word_count or 0:,}"))
        meta_col.addLayout(info_label("修改轮次", self.novel.revision_round or 0))
        meta_col.addLayout(info_label("创建时间", self.novel.created_at.strftime("%Y-%m-%d %H:%M")
                                      if self.novel.created_at else "—"))
        if self.novel.reviewer:
            meta_col.addLayout(info_label("审核人", self.novel.reviewer))
        if self.novel.reviewed_at:
            meta_col.addLayout(info_label("审核时间", self.novel.reviewed_at.strftime("%Y-%m-%d %H:%M")))
        info_row.addLayout(meta_col)
        layout.addLayout(info_row)

        # Tab 区域
        tabs = QTabWidget()

        # Tab 1: 章节列表
        chapters_tab = QWidget()
        ch_layout = QVBoxLayout(chapters_tab)
        ch_layout.setContentsMargins(8, 8, 8, 8)

        self.chapters_table = QTableWidget()
        self.chapters_table.setColumnCount(4)
        self.chapters_table.setHorizontalHeaderLabels(["章节序号", "章节标题", "字数", "状态"])
        self.chapters_table.horizontalHeader().setSectionResizeMode(QHeaderView.ResizeMode.Stretch)
        self.chapters_table.setEditTriggers(QTableWidget.EditTrigger.NoEditTriggers)
        self.chapters_table.setSelectionBehavior(QTableWidget.SelectionBehavior.SelectRows)
        self.chapters_table.verticalHeader().setVisible(False)
        self.chapters_table.setAlternatingRowColors(True)
        self.chapters_table.doubleClicked.connect(self._on_chapter_double_clicked)

        hint_lbl = QLabel("双击章节行可查看章节内容")
        hint_lbl.setObjectName("subtitle")

        ch_layout.addWidget(self.chapters_table)
        ch_layout.addWidget(hint_lbl)
        tabs.addTab(chapters_tab, "章节列表")

        # Tab 2: 审核意见
        review_tab = QWidget()
        rv_layout = QVBoxLayout(review_tab)
        rv_layout.setContentsMargins(8, 8, 8, 8)
        review_text = QTextEdit()
        review_content = ""
        if self.novel.review_comments:
            review_content += f"审核意见：\n{self.novel.review_comments}\n\n"
        if self.novel.revision_instructions:
            review_content += f"最新修改指令：\n{self.novel.revision_instructions}\n\n"
        if self.novel.reject_reason:
            review_content += f"拒绝原因：\n{self.novel.reject_reason}\n"
        review_text.setPlainText(review_content or "（暂无审核意见）")
        review_text.setReadOnly(True)
        rv_layout.addWidget(review_text)
        tabs.addTab(review_tab, "审核意见")

        layout.addWidget(tabs)

        btns = QDialogButtonBox(QDialogButtonBox.StandardButton.Close)
        btns.rejected.connect(self.reject)
        layout.addWidget(btns)

        # 加载章节数据
        self._load_chapters()

    def _load_chapters(self):
        """从数据库加载章节列表"""
        self._chapters = []
        try:
            with get_session() as s:
                chapters = (
                    s.query(NovelChapter)
                    .filter(NovelChapter.novel_id == self.novel.id)
                    .order_by(NovelChapter.chapter_no.asc())
                    .all()
                )
            # 在 session 关闭前读取所有属性
            chapter_data_list = []
            for ch in chapters:
                ch_data = NovelChapter()
                for col in ["id", "novel_id", "chapter_no", "chapter_title",
                            "content", "word_count", "status", "created_at", "updated_at"]:
                    setattr(ch_data, col, getattr(ch, col, None))
                chapter_data_list.append(ch_data)
            self._chapters = chapter_data_list

            self.chapters_table.setRowCount(len(chapter_data_list))
            for i, ch in enumerate(chapter_data_list):
                self.chapters_table.setItem(i, 0, QTableWidgetItem(str(ch.chapter_no or "")))
                self.chapters_table.setItem(i, 1, QTableWidgetItem(
                    ch.chapter_title or f"第{ch.chapter_no}章"))
                self.chapters_table.setItem(i, 2, QTableWidgetItem(f"{ch.word_count or 0:,}"))
                status_text = "已定稿" if ch.status == "finalized" else "草稿"
                self.chapters_table.setItem(i, 3, QTableWidgetItem(status_text))
        except Exception as e:
            self.chapters_table.setRowCount(1)
            self.chapters_table.setItem(0, 0, QTableWidgetItem(f"加载失败：{e}"))

    def _on_chapter_double_clicked(self, index):
        """双击章节行，弹出章节内容子对话框"""
        row = index.row()
        if 0 <= row < len(self._chapters):
            ChapterContentDialog(self, self._chapters[row]).exec()


# ---------------------------------------------------------------------------
# 修改历史对话框
# ---------------------------------------------------------------------------
class RevisionHistoryDialog(QDialog):
    """修改历史对话框：按轮次展示修改历史记录"""

    def __init__(self, parent, novel: Novel):
        super().__init__(parent)
        title_text = novel.title or novel.id[:12] + "..."
        self.setWindowTitle(f"修改历史 — {title_text}")
        self.setMinimumSize(680, 420)
        self._build(novel)

    def _build(self, novel: Novel):
        layout = QVBoxLayout(self)
        layout.setContentsMargins(20, 16, 20, 16)
        layout.setSpacing(10)

        history_table = QTableWidget()
        history_table.setColumnCount(4)
        history_table.setHorizontalHeaderLabels(["修改轮次", "审核人", "修改指令", "时间"])
        history_table.horizontalHeader().setSectionResizeMode(QHeaderView.ResizeMode.Stretch)
        history_table.setEditTriggers(QTableWidget.EditTrigger.NoEditTriggers)
        history_table.verticalHeader().setVisible(False)
        history_table.setAlternatingRowColors(True)

        try:
            with get_session() as s:
                histories = (
                    s.query(NovelRevisionHistory)
                    .filter(NovelRevisionHistory.novel_id == novel.id)
                    .order_by(NovelRevisionHistory.revision_round.asc())
                    .all()
                )
            history_table.setRowCount(len(histories))
            for i, h in enumerate(histories):
                history_table.setItem(i, 0, QTableWidgetItem(str(h.revision_round or "")))
                history_table.setItem(i, 1, QTableWidgetItem(h.reviewer or ""))
                history_table.setItem(i, 2, QTableWidgetItem(h.revision_instructions or ""))
                history_table.setItem(i, 3, QTableWidgetItem(
                    h.created_at.strftime("%Y-%m-%d %H:%M") if h.created_at else ""))
        except Exception as e:
            history_table.setRowCount(1)
            history_table.setItem(0, 0, QTableWidgetItem(f"加载失败：{e}"))

        layout.addWidget(history_table)

        btns = QDialogButtonBox(QDialogButtonBox.StandardButton.Close)
        btns.rejected.connect(self.reject)
        layout.addWidget(btns)


# ---------------------------------------------------------------------------
# 小说管理主页面
# ---------------------------------------------------------------------------
class NovelsPage(QWidget):
    def __init__(self):
        super().__init__()
        self._novels = []
        self._build_ui()
        self.load_data()

    def _build_ui(self):
        layout = QVBoxLayout(self)
        layout.setContentsMargins(24, 20, 24, 20)
        layout.setSpacing(14)

        # ── 顶部工具栏 ──────────────────────────────────────────
        top = QHBoxLayout()
        title = QLabel("小说管理")
        title.setObjectName("title")
        top.addWidget(title)
        top.addStretch()

        # 状态筛选下拉框
        top.addWidget(QLabel("状态筛选："))
        self.status_filter = QComboBox()
        self.status_filter.addItem("全部状态", "")
        for k, v in NOVEL_STATUS_LABELS.items():
            self.status_filter.addItem(v, k)
        self.status_filter.currentIndexChanged.connect(self.load_data)
        top.addWidget(self.status_filter)

        # 刷新按钮
        btn_refresh = QPushButton("刷新")
        btn_refresh.setObjectName("secondary")
        btn_refresh.clicked.connect(self.load_data)
        top.addWidget(btn_refresh)

        # 从大纲池创建小说按钮
        btn_create = QPushButton("+ 从大纲池创建小说")
        btn_create.clicked.connect(self._create_novel)
        top.addWidget(btn_create)

        layout.addLayout(top)

        # ── 小说列表表格 ─────────────────────────────────────────
        self.table = QTableWidget()
        self.table.setColumnCount(8)
        self.table.setHorizontalHeaderLabels([
            "ID", "标题", "智能体类型", "状态", "字数", "修改轮次", "创建时间", "操作"
        ])
        hdr = self.table.horizontalHeader()
        hdr.setSectionResizeMode(QHeaderView.ResizeMode.Stretch)
        hdr.setSectionResizeMode(7, QHeaderView.ResizeMode.Fixed)
        self.table.setColumnWidth(7, 240)
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
                q = s.query(Novel).order_by(Novel.created_at.desc())
                if status_filter:
                    q = q.filter(Novel.status == status_filter)
                novels = q.limit(300).all()
            self._novels = novels
            self._populate_table(novels)
            self.status_lbl.setText(f"共 {len(novels)} 部小说")
        except Exception as e:
            self.status_lbl.setText(f"加载失败：{e}")

    def _populate_table(self, novels):
        self.table.setRowCount(len(novels))
        for i, n in enumerate(novels):
            self.table.setItem(i, 0, QTableWidgetItem((n.id or "")[:12] + "..."))
            self.table.setItem(i, 1, QTableWidgetItem(n.title or "（生成中）"))
            self.table.setItem(i, 2, QTableWidgetItem(AGENT_LABELS.get(n.agent_type, n.agent_type or "")))

            status = n.status or "writing"
            status_item = QTableWidgetItem(NOVEL_STATUS_LABELS.get(status, status))
            status_item.setForeground(QColor(NOVEL_STATUS_COLORS.get(status, "#cdd6f4")))
            self.table.setItem(i, 3, status_item)

            self.table.setItem(i, 4, QTableWidgetItem(f"{n.word_count or 0:,}"))
            self.table.setItem(i, 5, QTableWidgetItem(str(n.revision_round or 0)))
            self.table.setItem(i, 6, QTableWidgetItem(
                n.created_at.strftime("%m-%d %H:%M") if n.created_at else ""))

            # 操作列
            btn_widget = QWidget()
            btn_layout = QHBoxLayout(btn_widget)
            btn_layout.setContentsMargins(4, 2, 4, 2)
            btn_layout.setSpacing(4)

            # 详情按钮（所有小说都有）
            btn_detail = QPushButton("详情")
            btn_detail.setObjectName("secondary")
            btn_detail.setFixedWidth(52)
            btn_detail.clicked.connect(lambda _, idx=i: self._show_detail(idx))
            btn_layout.addWidget(btn_detail)

            # 待审核小说显示"通过"、"修改"和"拒绝"按钮
            if status == "novel_pending_review":
                btn_approve = QPushButton("通过")
                btn_approve.setFixedWidth(52)
                btn_approve.clicked.connect(lambda _, idx=i: self._approve(idx))
                btn_layout.addWidget(btn_approve)

                btn_revise = QPushButton("修改")
                btn_revise.setObjectName("secondary")
                btn_revise.setFixedWidth(52)
                btn_revise.clicked.connect(lambda _, idx=i: self._request_revision(idx))
                btn_layout.addWidget(btn_revise)

                btn_reject = QPushButton("拒绝")
                btn_reject.setObjectName("danger")
                btn_reject.setFixedWidth(52)
                btn_reject.clicked.connect(lambda _, idx=i: self._reject(idx))
                btn_layout.addWidget(btn_reject)

            self.table.setCellWidget(i, 7, btn_widget)

    # ── 操作处理 ─────────────────────────────────────────────────
    def _create_novel(self):
        """弹出创建小说对话框，从大纲池选择大纲创建小说任务"""
        dlg = CreateNovelDialog(self)
        if dlg.exec() != QDialog.DialogCode.Accepted:
            return

        data = dlg.get_data()
        outline_id = data["outline_id"]
        agent_type = data["agent_type"]

        if not outline_id:
            QMessageBox.warning(self, "提示", "请先在大纲管理页面审核通过大纲，使大纲池中有可用大纲。")
            return

        try:
            import requests as _requests
            resp = _requests.post(
                "http://localhost:8000/novels/create_from_outline",
                json={
                    "outline_id": outline_id,
                    "agent_type": agent_type,
                },
                timeout=15,
            )
            if resp.status_code == 200:
                result = resp.json()
                QMessageBox.information(
                    self, "成功",
                    f"已创建小说任务\n"
                    f"小说ID：{result.get('novel_id', '')[:16]}...\n"
                    f"状态：{NOVEL_STATUS_LABELS.get(result.get('status', ''), result.get('status', ''))}",
                )
                self.load_data()
            elif resp.status_code == 409:
                QMessageBox.warning(self, "状态冲突", resp.json().get("detail", "大纲状态不是 approved"))
            elif resp.status_code == 404:
                QMessageBox.warning(self, "不存在", resp.json().get("detail", "大纲不存在"))
            else:
                detail = resp.json().get("detail", resp.text)
                QMessageBox.warning(self, "请求失败", f"后端返回错误：{detail}")
        except Exception as e:
            QMessageBox.warning(
                self, "后端不可用",
                f"无法连接到后端服务：{e}\n\n请启动后端服务后重试。",
            )

    def _show_detail(self, idx: int):
        """显示小说详情对话框"""
        novel = self._novels[idx]
        # 重新从数据库加载最新数据
        try:
            with get_session() as s:
                fresh = s.get(Novel, novel.id)
                if fresh is None:
                    QMessageBox.warning(self, "提示", "小说记录不存在")
                    return
                novel_data = Novel()
                for col in ["id", "outline_id", "agent_type", "title", "status",
                            "word_count", "revision_round", "reviewer", "review_comments",
                            "revision_instructions", "reject_reason", "reviewed_at",
                            "writing_started_at", "writing_finished_at",
                            "created_at", "updated_at"]:
                    setattr(novel_data, col, getattr(fresh, col, None))
        except Exception:
            novel_data = novel

        NovelDetailDialog(self, novel_data).exec()

    def _approve(self, idx: int):
        """审核通过"""
        novel = self._novels[idx]
        reply = QMessageBox.question(
            self, "确认通过",
            f"确定通过小说「{novel.title or novel.id[:12]}」的审核？\n通过后小说将进入发布队列。",
            QMessageBox.StandardButton.Yes | QMessageBox.StandardButton.No,
        )
        if reply != QMessageBox.StandardButton.Yes:
            return

        try:
            import requests as _requests
            resp = _requests.post(
                "http://localhost:8000/novels/review_decision",
                json={
                    "novel_id":  novel.id,
                    "decision":  "approve",
                    "reviewer":  "GUI用户",
                    "comments":  None,
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
                QMessageBox.warning(self, "不存在", resp.json().get("detail", "小说不存在"))
                return
        except Exception:
            pass

        # 后端不可用时直接操作数据库
        self._update_novel_status(novel.id, "novel_approved", reviewer="GUI用户")

    def _request_revision(self, idx: int):
        """提交修改意见"""
        novel = self._novels[idx]
        dlg = RevisionDialog(self, novel_title=novel.title or novel.id[:12])
        if dlg.exec() != QDialog.DialogCode.Accepted:
            return

        instructions = dlg.get_instructions()

        try:
            import requests as _requests
            resp = _requests.post(
                "http://localhost:8000/novels/review_decision",
                json={
                    "novel_id":               novel.id,
                    "decision":               "request_revision",
                    "reviewer":               "GUI用户",
                    "revision_instructions":  instructions,
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
                QMessageBox.warning(self, "不存在", resp.json().get("detail", "小说不存在"))
                return
            elif resp.status_code == 422:
                QMessageBox.warning(self, "参数错误", resp.json().get("detail", "修改指令不能为空"))
                return
        except Exception:
            pass

        # 后端不可用时直接操作数据库
        self._update_novel_status(
            novel.id, "revising",
            reviewer="GUI用户",
            revision_instructions=instructions,
            increment_revision_round=True,
        )

    def _reject(self, idx: int):
        """审核拒绝，弹出输入框填写拒绝原因"""
        novel = self._novels[idx]
        reason, ok = QInputDialog.getText(
            self, "拒绝原因",
            f"请输入拒绝小说「{novel.title or novel.id[:12]}」的原因：",
        )
        if not ok:
            return
        if not reason.strip():
            QMessageBox.warning(self, "提示", "拒绝原因不能为空")
            return

        try:
            import requests as _requests
            resp = _requests.post(
                "http://localhost:8000/novels/review_decision",
                json={
                    "novel_id": novel.id,
                    "decision": "reject",
                    "reviewer": "GUI用户",
                    "reason":   reason.strip(),
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
                QMessageBox.warning(self, "不存在", resp.json().get("detail", "小说不存在"))
                return
        except Exception:
            pass

        # 后端不可用时直接操作数据库
        self._update_novel_status(
            novel.id, "novel_rejected",
            reviewer="GUI用户",
            reject_reason=reason.strip(),
        )

    def _update_novel_status(
        self,
        novel_id: str,
        new_status: str,
        reviewer: str = "GUI用户",
        reject_reason: str = None,
        review_comments: str = None,
        revision_instructions: str = None,
        increment_revision_round: bool = False,
    ):
        """直接更新数据库中的小说状态（后端不可用时的降级方案）"""
        try:
            with get_session() as s:
                n = s.get(Novel, novel_id)
                if n is None:
                    QMessageBox.warning(self, "提示", "小说记录不存在")
                    return
                n.status = new_status
                n.reviewer = reviewer
                n.reviewed_at = datetime.utcnow()
                n.updated_at = datetime.utcnow()
                if reject_reason:
                    n.reject_reason = reject_reason
                if review_comments:
                    n.review_comments = review_comments
                if revision_instructions:
                    n.revision_instructions = revision_instructions
                if increment_revision_round:
                    n.revision_round = (n.revision_round or 0) + 1
                    # 写入修改历史
                    history = NovelRevisionHistory(
                        novel_id=novel_id,
                        revision_round=n.revision_round,
                        revision_instructions=revision_instructions or "",
                        reviewer=reviewer,
                        content_snapshot=None,
                        created_at=datetime.utcnow(),
                    )
                    s.add(history)
            self.load_data()
        except Exception as e:
            QMessageBox.critical(self, "错误", f"操作失败：{e}")

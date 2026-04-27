"""账号矩阵管理页面"""
import uuid
from datetime import datetime
from PyQt6.QtWidgets import (
    QWidget, QVBoxLayout, QHBoxLayout, QLabel, QPushButton,
    QTableWidget, QTableWidgetItem, QHeaderView, QDialog,
    QFormLayout, QLineEdit, QComboBox, QSpinBox, QMessageBox,
    QDialogButtonBox,
)
from PyQt6.QtCore import Qt
from PyQt6.QtGui import QColor

from ai_novel_studio.gui.db import get_session, Account
from ai_novel_studio.gui.styles import PLATFORM_LABELS, AGENT_LABELS


PLATFORMS = list(PLATFORM_LABELS.keys())
AGENTS    = list(AGENT_LABELS.keys())


class AccountDialog(QDialog):
    """新建/编辑账号对话框"""

    def __init__(self, parent=None, account: Account = None):
        super().__init__(parent)
        self.account = account
        self.setWindowTitle("编辑账号" if account else "新建账号")
        self.setMinimumWidth(380)
        self._build()

    def _build(self):
        layout = QFormLayout(self)
        layout.setSpacing(12)
        layout.setContentsMargins(20, 20, 20, 20)

        self.username = QLineEdit()
        self.display_name = QLineEdit()
        self.platform = QComboBox()
        self.platform.addItems([PLATFORM_LABELS[p] for p in PLATFORMS])
        self.agent_type = QComboBox()
        self.agent_type.addItems([AGENT_LABELS[a] for a in AGENTS])
        self.daily_quota = QSpinBox()
        self.daily_quota.setRange(1, 20)
        self.daily_quota.setValue(3)
        self.status = QComboBox()
        self.status.addItems(["active", "inactive"])

        layout.addRow("平台用户名 *", self.username)
        layout.addRow("显示名称",     self.display_name)
        layout.addRow("平台 *",       self.platform)
        layout.addRow("题材智能体 *", self.agent_type)
        layout.addRow("每日配额",     self.daily_quota)
        layout.addRow("状态",         self.status)

        if self.account:
            self.username.setText(self.account.username or "")
            self.display_name.setText(self.account.display_name or "")
            plat_idx = PLATFORMS.index(self.account.platform) if self.account.platform in PLATFORMS else 0
            self.platform.setCurrentIndex(plat_idx)
            agent_idx = AGENTS.index(self.account.agent_type) if self.account.agent_type in AGENTS else 0
            self.agent_type.setCurrentIndex(agent_idx)
            self.daily_quota.setValue(self.account.daily_quota or 3)
            self.status.setCurrentText(self.account.status or "active")

        btns = QDialogButtonBox(QDialogButtonBox.StandardButton.Ok | QDialogButtonBox.StandardButton.Cancel)
        btns.accepted.connect(self._validate)
        btns.rejected.connect(self.reject)
        layout.addRow(btns)

    def _validate(self):
        if not self.username.text().strip():
            QMessageBox.warning(self, "提示", "平台用户名不能为空")
            return
        self.accept()

    def get_data(self) -> dict:
        return {
            "username":     self.username.text().strip(),
            "display_name": self.display_name.text().strip() or None,
            "platform":     PLATFORMS[self.platform.currentIndex()],
            "agent_type":   AGENTS[self.agent_type.currentIndex()],
            "daily_quota":  self.daily_quota.value(),
            "status":       self.status.currentText(),
        }


class AccountsPage(QWidget):
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
        title = QLabel("账号矩阵管理")
        title.setObjectName("title")
        top.addWidget(title)
        top.addStretch()
        btn_add = QPushButton("+ 新建账号")
        btn_add.clicked.connect(self._add_account)
        top.addWidget(btn_add)
        btn_refresh = QPushButton("刷新")
        btn_refresh.setObjectName("secondary")
        btn_refresh.clicked.connect(self.load_data)
        top.addWidget(btn_refresh)
        layout.addLayout(top)

        # 表格
        self.table = QTableWidget()
        self.table.setColumnCount(8)
        self.table.setHorizontalHeaderLabels([
            "用户名", "显示名称", "平台", "题材", "每日配额", "已发布", "状态", "操作"
        ])
        self.table.horizontalHeader().setSectionResizeMode(QHeaderView.ResizeMode.Stretch)
        self.table.horizontalHeader().setSectionResizeMode(7, QHeaderView.ResizeMode.Fixed)
        self.table.setColumnWidth(7, 160)
        self.table.setEditTriggers(QTableWidget.EditTrigger.NoEditTriggers)
        self.table.setSelectionBehavior(QTableWidget.SelectionBehavior.SelectRows)
        self.table.verticalHeader().setVisible(False)
        self.table.setAlternatingRowColors(True)
        layout.addWidget(self.table)

        self.status_lbl = QLabel("")
        self.status_lbl.setObjectName("subtitle")
        layout.addWidget(self.status_lbl)

    def load_data(self):
        try:
            with get_session() as s:
                accounts = s.query(Account).order_by(Account.created_at.desc()).all()
            self._accounts = accounts
            self.table.setRowCount(len(accounts))
            for i, acc in enumerate(accounts):
                self.table.setItem(i, 0, QTableWidgetItem(acc.username or ""))
                self.table.setItem(i, 1, QTableWidgetItem(acc.display_name or ""))
                self.table.setItem(i, 2, QTableWidgetItem(PLATFORM_LABELS.get(acc.platform, acc.platform)))
                self.table.setItem(i, 3, QTableWidgetItem(AGENT_LABELS.get(acc.agent_type, acc.agent_type)))
                self.table.setItem(i, 4, QTableWidgetItem(str(acc.daily_quota)))
                self.table.setItem(i, 5, QTableWidgetItem(str(acc.total_published)))

                status_item = QTableWidgetItem("● 活跃" if acc.status == "active" else "○ 停用")
                status_item.setForeground(QColor("#a6e3a1" if acc.status == "active" else "#6c7086"))
                self.table.setItem(i, 6, status_item)

                # 操作按钮组
                btn_widget = QWidget()
                btn_layout = QHBoxLayout(btn_widget)
                btn_layout.setContentsMargins(4, 2, 4, 2)
                btn_layout.setSpacing(6)
                btn_edit = QPushButton("编辑")
                btn_edit.setObjectName("secondary")
                btn_edit.setFixedWidth(56)
                btn_edit.clicked.connect(lambda _, idx=i: self._edit_account(idx))
                btn_del = QPushButton("删除")
                btn_del.setObjectName("danger")
                btn_del.setFixedWidth(56)
                btn_del.clicked.connect(lambda _, idx=i: self._delete_account(idx))
                btn_layout.addWidget(btn_edit)
                btn_layout.addWidget(btn_del)
                self.table.setCellWidget(i, 7, btn_widget)

            self.status_lbl.setText(f"共 {len(accounts)} 个账号")
        except Exception as e:
            self.status_lbl.setText(f"加载失败：{e}")

    def _add_account(self):
        dlg = AccountDialog(self)
        if dlg.exec() == QDialog.DialogCode.Accepted:
            data = dlg.get_data()
            try:
                with get_session() as s:
                    acc = Account(
                        id=str(uuid.uuid4()),
                        created_at=datetime.utcnow(),
                        updated_at=datetime.utcnow(),
                        **data,
                    )
                    s.add(acc)
                self.load_data()
            except Exception as e:
                QMessageBox.critical(self, "错误", f"创建失败：{e}")

    def _edit_account(self, idx: int):
        acc = self._accounts[idx]
        dlg = AccountDialog(self, acc)
        if dlg.exec() == QDialog.DialogCode.Accepted:
            data = dlg.get_data()
            try:
                with get_session() as s:
                    a = s.get(Account, acc.id)
                    for k, v in data.items():
                        setattr(a, k, v)
                    a.updated_at = datetime.utcnow()
                self.load_data()
            except Exception as e:
                QMessageBox.critical(self, "错误", f"更新失败：{e}")

    def _delete_account(self, idx: int):
        acc = self._accounts[idx]
        reply = QMessageBox.question(
            self, "确认删除",
            f"确定删除账号「{acc.username}」？",
            QMessageBox.StandardButton.Yes | QMessageBox.StandardButton.No,
        )
        if reply == QMessageBox.StandardButton.Yes:
            try:
                with get_session() as s:
                    a = s.get(Account, acc.id)
                    s.delete(a)
                self.load_data()
            except Exception as e:
                QMessageBox.critical(self, "错误", f"删除失败：{e}")

"""主窗口 — 侧边导航 + 页面切换"""
from PyQt6.QtWidgets import (
    QMainWindow, QWidget, QHBoxLayout, QVBoxLayout,
    QListWidget, QListWidgetItem, QStackedWidget, QLabel,
    QSizePolicy, QFrame, QMessageBox,
)
from PyQt6.QtCore import Qt, QSize
from PyQt6.QtGui import QIcon, QFont

from ai_novel_studio.gui.db import test_connection
from ai_novel_studio.gui.styles import MAIN_STYLE


NAV_ITEMS = [
    ("📊  内容看板",   "dashboard"),
    ("📋  创作任务",   "tasks"),
    ("📚  语料库",     "corpus"),
    ("🔔  系统告警",   "alerts"),
    ("⚙️  系统设置",   "settings"),
]


class MainWindow(QMainWindow):
    def __init__(self):
        super().__init__()
        self.setWindowTitle("AI 内容创作工作室")
        self.setMinimumSize(1200, 750)
        self.resize(1400, 860)
        self.setStyleSheet(MAIN_STYLE)

        self._pages = {}
        self._build_ui()
        self._check_db()

    def _build_ui(self):
        central = QWidget()
        self.setCentralWidget(central)
        root = QHBoxLayout(central)
        root.setContentsMargins(0, 0, 0, 0)
        root.setSpacing(0)

        # ── 侧边栏 ──────────────────────────────────────────────
        sidebar = QWidget()
        sidebar.setFixedWidth(200)
        sidebar.setStyleSheet("background-color: #181825;")
        sidebar_layout = QVBoxLayout(sidebar)
        sidebar_layout.setContentsMargins(0, 0, 0, 0)
        sidebar_layout.setSpacing(0)

        # Logo 区域
        logo_widget = QWidget()
        logo_widget.setFixedHeight(72)
        logo_widget.setStyleSheet("background-color: #181825; border-bottom: 1px solid #313244;")
        logo_layout = QVBoxLayout(logo_widget)
        logo_layout.setContentsMargins(20, 0, 20, 0)
        logo_title = QLabel("✍ AI内容创作工作室")
        logo_title.setStyleSheet("color: #cba6f7; font-size: 15px; font-weight: bold;")
        logo_sub = QLabel("内容生成管理系统")
        logo_sub.setStyleSheet("color: #6c7086; font-size: 11px;")
        logo_layout.addWidget(logo_title)
        logo_layout.addWidget(logo_sub)
        sidebar_layout.addWidget(logo_widget)

        # 导航列表
        self.nav = QListWidget()
        self.nav.setObjectName("nav")
        self.nav.setSizePolicy(QSizePolicy.Policy.Expanding, QSizePolicy.Policy.Expanding)
        for label, _ in NAV_ITEMS:
            item = QListWidgetItem(label)
            item.setSizeHint(QSize(180, 44))
            self.nav.addItem(item)
        self.nav.setCurrentRow(0)
        self.nav.currentRowChanged.connect(self._switch_page)
        sidebar_layout.addWidget(self.nav)

        # 底部版本信息
        ver_lbl = QLabel("v1.0.0")
        ver_lbl.setStyleSheet("color: #45475a; font-size: 11px; padding: 12px 20px;")
        sidebar_layout.addWidget(ver_lbl)

        root.addWidget(sidebar)

        # ── 分割线 ──────────────────────────────────────────────
        sep = QFrame()
        sep.setFrameShape(QFrame.Shape.VLine)
        sep.setStyleSheet("color: #313244;")
        root.addWidget(sep)

        # ── 内容区 ──────────────────────────────────────────────
        self.stack = QStackedWidget()
        self.stack.setStyleSheet("background-color: #1e1e2e;")
        root.addWidget(self.stack)

        # 懒加载：先放占位页
        for i, (label, key) in enumerate(NAV_ITEMS):
            placeholder = QWidget()
            self.stack.addWidget(placeholder)

        # 立即加载首页
        self._load_page(0)

    def _switch_page(self, idx: int):
        self._load_page(idx)
        self.stack.setCurrentIndex(idx)

    def _load_page(self, idx: int):
        key = NAV_ITEMS[idx][1]
        if key in self._pages:
            return  # 已加载

        page = self._create_page(key)
        self._pages[key] = page
        self.stack.removeWidget(self.stack.widget(idx))
        self.stack.insertWidget(idx, page)

    def _create_page(self, key: str) -> QWidget:
        if key == "dashboard":
            from ai_novel_studio.gui.pages.dashboard import DashboardPage
            return DashboardPage()
        elif key == "tasks":
            from ai_novel_studio.gui.pages.tasks import TasksPage
            return TasksPage()
        elif key == "corpus":
            from ai_novel_studio.gui.pages.corpus import CorpusPage
            return CorpusPage()
        elif key == "alerts":
            from ai_novel_studio.gui.pages.alerts import AlertsPage
            return AlertsPage()
        elif key == "settings":
            from ai_novel_studio.gui.pages.settings import SettingsPage
            return SettingsPage()
        return QWidget()

    def _check_db(self):
        """启动时检查数据库连接"""
        if not test_connection():
            QMessageBox.warning(
                self,
                "数据库连接失败",
                "无法连接到 MySQL 数据库（localhost:3306）。\n\n"
                "请确认：\n"
                "1. MySQL 服务已启动\n"
                "2. 账号密码正确（默认 root/root）\n"
                "3. 数据库 ai_novel_studio 已创建\n\n"
                "可在「系统设置」页面修改连接配置。",
            )

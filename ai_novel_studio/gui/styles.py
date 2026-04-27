"""全局样式表 — 深色现代风格"""

MAIN_STYLE = """
QMainWindow, QWidget {
    background-color: #1e1e2e;
    color: #cdd6f4;
    font-family: "Microsoft YaHei", "Segoe UI", sans-serif;
    font-size: 13px;
}

/* 侧边导航 */
QListWidget#nav {
    background-color: #181825;
    border: none;
    border-right: 1px solid #313244;
    padding: 8px 0;
    outline: none;
}
QListWidget#nav::item {
    padding: 12px 20px;
    border-radius: 6px;
    margin: 2px 8px;
    color: #a6adc8;
}
QListWidget#nav::item:selected {
    background-color: #313244;
    color: #cba6f7;
    font-weight: bold;
}
QListWidget#nav::item:hover:!selected {
    background-color: #262637;
    color: #cdd6f4;
}

/* 卡片容器 */
QFrame#card {
    background-color: #24273a;
    border: 1px solid #313244;
    border-radius: 10px;
    padding: 12px;
}

/* 按钮 */
QPushButton {
    background-color: #cba6f7;
    color: #1e1e2e;
    border: none;
    border-radius: 6px;
    padding: 7px 18px;
    font-weight: bold;
}
QPushButton:hover {
    background-color: #d4b8ff;
}
QPushButton:pressed {
    background-color: #b48ee8;
}
QPushButton#danger {
    background-color: #f38ba8;
    color: #1e1e2e;
}
QPushButton#danger:hover {
    background-color: #f5a0b5;
}
QPushButton#secondary {
    background-color: #313244;
    color: #cdd6f4;
}
QPushButton#secondary:hover {
    background-color: #45475a;
}

/* 表格 */
QTableWidget {
    background-color: #1e1e2e;
    border: 1px solid #313244;
    border-radius: 8px;
    gridline-color: #313244;
    selection-background-color: #313244;
    outline: none;
}
QTableWidget::item {
    padding: 6px 10px;
    border-bottom: 1px solid #2a2a3e;
}
QTableWidget::item:selected {
    background-color: #45475a;
    color: #cdd6f4;
}
QHeaderView::section {
    background-color: #181825;
    color: #a6adc8;
    padding: 8px 10px;
    border: none;
    border-bottom: 1px solid #313244;
    font-weight: bold;
}

/* 输入框 */
QLineEdit, QTextEdit, QComboBox, QSpinBox {
    background-color: #313244;
    border: 1px solid #45475a;
    border-radius: 6px;
    padding: 6px 10px;
    color: #cdd6f4;
    selection-background-color: #cba6f7;
    selection-color: #1e1e2e;
}
QLineEdit:focus, QTextEdit:focus, QComboBox:focus {
    border-color: #cba6f7;
}
QComboBox::drop-down {
    border: none;
    width: 24px;
}
QComboBox QAbstractItemView {
    background-color: #313244;
    border: 1px solid #45475a;
    selection-background-color: #cba6f7;
    selection-color: #1e1e2e;
}

/* 标签 */
QLabel#title {
    font-size: 18px;
    font-weight: bold;
    color: #cba6f7;
}
QLabel#subtitle {
    font-size: 12px;
    color: #6c7086;
}
QLabel#stat_value {
    font-size: 28px;
    font-weight: bold;
    color: #a6e3a1;
}
QLabel#stat_label {
    font-size: 11px;
    color: #6c7086;
}

/* 状态徽章 */
QLabel#badge_active   { color: #a6e3a1; font-weight: bold; }
QLabel#badge_inactive { color: #6c7086; font-weight: bold; }
QLabel#badge_pending  { color: #f9e2af; font-weight: bold; }
QLabel#badge_done     { color: #a6e3a1; font-weight: bold; }
QLabel#badge_rejected { color: #f38ba8; font-weight: bold; }
QLabel#badge_review   { color: #89b4fa; font-weight: bold; }

/* 滚动条 */
QScrollBar:vertical {
    background: #1e1e2e;
    width: 8px;
    border-radius: 4px;
}
QScrollBar::handle:vertical {
    background: #45475a;
    border-radius: 4px;
    min-height: 30px;
}
QScrollBar::handle:vertical:hover {
    background: #6c7086;
}
QScrollBar::add-line:vertical, QScrollBar::sub-line:vertical { height: 0; }

/* 分割线 */
QFrame[frameShape="4"], QFrame[frameShape="5"] {
    color: #313244;
}

/* 对话框 */
QDialog {
    background-color: #1e1e2e;
}

/* 消息框 */
QMessageBox {
    background-color: #1e1e2e;
}
QMessageBox QPushButton {
    min-width: 80px;
}

/* Tab */
QTabWidget::pane {
    border: 1px solid #313244;
    border-radius: 8px;
    background-color: #1e1e2e;
}
QTabBar::tab {
    background-color: #181825;
    color: #a6adc8;
    padding: 8px 20px;
    border-top-left-radius: 6px;
    border-top-right-radius: 6px;
    margin-right: 2px;
}
QTabBar::tab:selected {
    background-color: #313244;
    color: #cba6f7;
}
"""

# 状态颜色映射
STAGE_COLORS = {
    "pending":             "#f9e2af",
    "topic_generating":    "#89dceb",
    "outline_generating":  "#89dceb",
    "content_generating":  "#89dceb",
    "polishing":           "#89dceb",
    "human_review":        "#89b4fa",
    "publishing":          "#cba6f7",
    "done":                "#a6e3a1",
    "rejected":            "#f38ba8",
}

STAGE_LABELS = {
    "pending":             "待处理",
    "topic_generating":    "生成选题",
    "outline_generating":  "生成大纲",
    "content_generating":  "生成正文",
    "polishing":           "润色中",
    "human_review":        "待审核",
    "publishing":          "发布中",
    "done":                "已完成",
    "rejected":            "已拒绝",
}

PLATFORM_LABELS = {
    "fanqie":      "番茄小说",
    "qimao":       "七猫小说",
    "zhihu":       "知乎盐选",
    "xiaohongshu": "小红书",
    "douyin":      "抖音",
}

AGENT_LABELS = {
    "female_rebirth": "女频重生",
    "male_power":     "男频爽文",
    "suspense":       "悬疑短篇",
    "romance":        "甜宠",
}

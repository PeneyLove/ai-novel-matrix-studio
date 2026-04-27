"""系统设置页面 — 数据库连接、模型 API Key 配置"""
import os
from PyQt6.QtWidgets import (
    QWidget, QVBoxLayout, QHBoxLayout, QLabel, QPushButton,
    QLineEdit, QFormLayout, QFrame, QMessageBox, QTabWidget,
    QGroupBox,
)
from PyQt6.QtCore import Qt

from ai_novel_studio.gui.db import test_connection


class SettingsPage(QWidget):
    def __init__(self):
        super().__init__()
        self._build_ui()

    def _build_ui(self):
        layout = QVBoxLayout(self)
        layout.setContentsMargins(24, 20, 24, 20)
        layout.setSpacing(16)

        title = QLabel("系统设置")
        title.setObjectName("title")
        layout.addWidget(title)

        tabs = QTabWidget()
        tabs.addTab(self._build_db_tab(), "数据库连接")
        tabs.addTab(self._build_api_tab(), "AI 模型 API")
        layout.addWidget(tabs)
        layout.addStretch()

    def _build_db_tab(self) -> QWidget:
        w = QWidget()
        layout = QVBoxLayout(w)
        layout.setContentsMargins(20, 20, 20, 20)
        layout.setSpacing(16)

        group = QGroupBox("MySQL 连接配置")
        form = QFormLayout(group)
        form.setSpacing(10)

        self.db_host = QLineEdit(os.getenv("DB_HOST", "localhost"))
        self.db_port = QLineEdit(os.getenv("DB_PORT", "3306"))
        self.db_user = QLineEdit(os.getenv("DB_USER", "root"))
        self.db_pass = QLineEdit(os.getenv("DB_PASS", "root"))
        self.db_pass.setEchoMode(QLineEdit.EchoMode.Password)
        self.db_name = QLineEdit(os.getenv("DB_NAME", "ai_novel_studio"))

        form.addRow("主机地址：", self.db_host)
        form.addRow("端口：",     self.db_port)
        form.addRow("用户名：",   self.db_user)
        form.addRow("密码：",     self.db_pass)
        form.addRow("数据库名：", self.db_name)
        layout.addWidget(group)

        btn_row = QHBoxLayout()
        btn_test = QPushButton("测试连接")
        btn_test.setObjectName("secondary")
        btn_test.clicked.connect(self._test_db)
        btn_save = QPushButton("保存配置")
        btn_save.clicked.connect(self._save_db)
        btn_row.addWidget(btn_test)
        btn_row.addWidget(btn_save)
        btn_row.addStretch()
        layout.addLayout(btn_row)

        self.db_status = QLabel("")
        self.db_status.setObjectName("subtitle")
        layout.addWidget(self.db_status)
        layout.addStretch()
        return w

    def _build_api_tab(self) -> QWidget:
        w = QWidget()
        layout = QVBoxLayout(w)
        layout.setContentsMargins(20, 20, 20, 20)
        layout.setSpacing(16)

        models = [
            ("MINIMAX_API_KEY",  "MiniMax API Key",  "选题生成"),
            ("DOUBAO_API_KEY",   "豆包 API Key",     "大纲生成"),
            ("QWEN_API_KEY",     "Qwen API Key",     "正文生成"),
            ("DEEPSEEK_API_KEY", "DeepSeek API Key", "内容润色"),
        ]
        self._api_inputs = {}
        for env_key, label, hint in models:
            group = QGroupBox(f"{label}  （用于：{hint}）")
            form = QFormLayout(group)
            inp = QLineEdit(os.getenv(env_key, ""))
            inp.setEchoMode(QLineEdit.EchoMode.Password)
            inp.setPlaceholderText(f"输入 {label}...")
            form.addRow("API Key：", inp)
            self._api_inputs[env_key] = inp
            layout.addWidget(group)

        btn_save = QPushButton("保存 API 配置")
        btn_save.clicked.connect(self._save_api)
        layout.addWidget(btn_save)

        note = QLabel("⚠ API Key 保存到本地 .env 文件，请勿泄露。")
        note.setStyleSheet("color: #f9e2af; font-size: 11px;")
        layout.addWidget(note)
        layout.addStretch()
        return w

    def _test_db(self):
        self.db_status.setText("连接测试中...")
        ok = test_connection()
        if ok:
            self.db_status.setStyleSheet("color: #a6e3a1;")
            self.db_status.setText("✓ 数据库连接成功")
        else:
            self.db_status.setStyleSheet("color: #f38ba8;")
            self.db_status.setText("✗ 连接失败，请检查配置")

    def _save_db(self):
        host = self.db_host.text().strip()
        port = self.db_port.text().strip()
        user = self.db_user.text().strip()
        pwd  = self.db_pass.text()
        name = self.db_name.text().strip()
        url = f"mysql+pymysql://{user}:{pwd}@{host}:{port}/{name}?charset=utf8mb4"
        os.environ["MYSQL_SYNC_URL"] = url
        self._write_env("MYSQL_SYNC_URL", url)
        QMessageBox.information(self, "成功", "数据库配置已保存，重启应用后生效。")

    def _save_api(self):
        lines = []
        for env_key, inp in self._api_inputs.items():
            val = inp.text().strip()
            if val:
                os.environ[env_key] = val
                lines.append(f"{env_key}={val}")
        if lines:
            for line in lines:
                k, v = line.split("=", 1)
                self._write_env(k, v)
        QMessageBox.information(self, "成功", "API Key 已保存。")

    @staticmethod
    def _write_env(key: str, value: str):
        """写入 .env 文件"""
        env_path = os.path.join(os.path.dirname(__file__), "..", "..", ".env")
        lines = []
        found = False
        if os.path.exists(env_path):
            with open(env_path, "r", encoding="utf-8") as f:
                lines = f.readlines()
            for i, line in enumerate(lines):
                if line.startswith(f"{key}="):
                    lines[i] = f"{key}={value}\n"
                    found = True
                    break
        if not found:
            lines.append(f"{key}={value}\n")
        with open(env_path, "w", encoding="utf-8") as f:
            f.writelines(lines)

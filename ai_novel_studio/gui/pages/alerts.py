"""系统告警页面"""
from datetime import datetime
from PyQt6.QtWidgets import (
    QWidget, QVBoxLayout, QHBoxLayout, QLabel, QPushButton,
    QTableWidget, QTableWidgetItem, QHeaderView, QComboBox, QMessageBox,
)
from PyQt6.QtCore import Qt
from PyQt6.QtGui import QColor

from ai_novel_studio.gui.db import get_session, SystemAlert


SEVERITY_COLORS = {
    "info":     "#89b4fa",
    "warning":  "#f9e2af",
    "error":    "#f38ba8",
    "critical": "#ff0000",
}

SEVERITY_LABELS = {
    "info":     "信息",
    "warning":  "警告",
    "error":    "错误",
    "critical": "严重",
}


class AlertsPage(QWidget):
    def __init__(self):
        super().__init__()
        self._alerts = []
        self._build_ui()
        self.load_data()

    def _build_ui(self):
        layout = QVBoxLayout(self)
        layout.setContentsMargins(24, 20, 24, 20)
        layout.setSpacing(14)

        top = QHBoxLayout()
        title = QLabel("系统告警")
        title.setObjectName("title")
        top.addWidget(title)
        top.addStretch()

        self.filter_combo = QComboBox()
        self.filter_combo.addItem("全部", "")
        self.filter_combo.addItem("未处理", "unresolved")
        self.filter_combo.addItem("已处理", "resolved")
        self.filter_combo.currentIndexChanged.connect(self.load_data)
        top.addWidget(QLabel("状态："))
        top.addWidget(self.filter_combo)

        btn_refresh = QPushButton("刷新")
        btn_refresh.setObjectName("secondary")
        btn_refresh.clicked.connect(self.load_data)
        top.addWidget(btn_refresh)

        btn_clear = QPushButton("全部标记已处理")
        btn_clear.setObjectName("secondary")
        btn_clear.clicked.connect(self._resolve_all)
        top.addWidget(btn_clear)
        layout.addLayout(top)

        self.table = QTableWidget()
        self.table.setColumnCount(6)
        self.table.setHorizontalHeaderLabels(["时间", "类型", "级别", "消息", "状态", "操作"])
        self.table.horizontalHeader().setSectionResizeMode(QHeaderView.ResizeMode.Stretch)
        self.table.horizontalHeader().setSectionResizeMode(5, QHeaderView.ResizeMode.Fixed)
        self.table.setColumnWidth(5, 100)
        self.table.setEditTriggers(QTableWidget.EditTrigger.NoEditTriggers)
        self.table.setSelectionBehavior(QTableWidget.SelectionBehavior.SelectRows)
        self.table.verticalHeader().setVisible(False)
        layout.addWidget(self.table)

        self.status_lbl = QLabel("")
        self.status_lbl.setObjectName("subtitle")
        layout.addWidget(self.status_lbl)

    def load_data(self):
        filter_val = self.filter_combo.currentData()
        try:
            with get_session() as s:
                q = s.query(SystemAlert).order_by(SystemAlert.created_at.desc())
                if filter_val == "unresolved":
                    q = q.filter(SystemAlert.resolved == False)
                elif filter_val == "resolved":
                    q = q.filter(SystemAlert.resolved == True)
                alerts = q.limit(200).all()
            self._alerts = alerts
            self.table.setRowCount(len(alerts))
            for i, a in enumerate(alerts):
                self.table.setItem(i, 0, QTableWidgetItem(
                    a.created_at.strftime("%m-%d %H:%M:%S") if a.created_at else ""))
                self.table.setItem(i, 1, QTableWidgetItem(a.alert_type or ""))
                sev_item = QTableWidgetItem(SEVERITY_LABELS.get(a.severity, a.severity or ""))
                sev_item.setForeground(QColor(SEVERITY_COLORS.get(a.severity, "#cdd6f4")))
                self.table.setItem(i, 2, sev_item)
                msg = (a.message or "")[:80] + ("..." if len(a.message or "") > 80 else "")
                self.table.setItem(i, 3, QTableWidgetItem(msg))
                status_item = QTableWidgetItem("已处理" if a.resolved else "未处理")
                status_item.setForeground(QColor("#a6e3a1" if a.resolved else "#f38ba8"))
                self.table.setItem(i, 4, status_item)

                if not a.resolved:
                    btn = QPushButton("标记处理")
                    btn.setObjectName("secondary")
                    btn.clicked.connect(lambda _, idx=i: self._resolve(idx))
                    self.table.setCellWidget(i, 5, btn)

            unresolved = sum(1 for a in alerts if not a.resolved)
            self.status_lbl.setText(f"共 {len(alerts)} 条告警，{unresolved} 条未处理")
        except Exception as e:
            self.status_lbl.setText(f"加载失败：{e}")

    def _resolve(self, idx: int):
        alert = self._alerts[idx]
        try:
            with get_session() as s:
                a = s.get(SystemAlert, alert.id)
                a.resolved = True
                a.resolved_at = datetime.utcnow()
            self.load_data()
        except Exception as e:
            QMessageBox.critical(self, "错误", f"操作失败：{e}")

    def _resolve_all(self):
        reply = QMessageBox.question(self, "确认", "将所有未处理告警标记为已处理？",
                                     QMessageBox.StandardButton.Yes | QMessageBox.StandardButton.No)
        if reply == QMessageBox.StandardButton.Yes:
            try:
                with get_session() as s:
                    s.query(SystemAlert).filter(SystemAlert.resolved == False).update(
                        {"resolved": True, "resolved_at": datetime.utcnow()})
                self.load_data()
            except Exception as e:
                QMessageBox.critical(self, "错误", f"操作失败：{e}")

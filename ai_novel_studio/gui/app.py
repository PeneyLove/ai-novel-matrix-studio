"""GUI 应用入口"""
import sys
import os

# 确保项目根目录在 Python 路径中
sys.path.insert(0, os.path.dirname(os.path.dirname(os.path.dirname(__file__))))

from PyQt6.QtWidgets import QApplication
from PyQt6.QtCore import Qt
from PyQt6.QtGui import QFont

from ai_novel_studio.gui.main_window import MainWindow


def run():
    app = QApplication(sys.argv)
    app.setApplicationName("AI小说矩阵工作室")
    app.setApplicationVersion("1.0.0")

    # 设置默认字体
    font = QFont("Microsoft YaHei", 10)
    app.setFont(font)

    # 高 DPI 支持
    app.setAttribute(Qt.ApplicationAttribute.AA_UseHighDpiPixmaps)

    window = MainWindow()
    window.show()
    sys.exit(app.exec())


if __name__ == "__main__":
    run()

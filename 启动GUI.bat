@echo off
chcp 65001 >nul
title AI小说矩阵工作室

echo ========================================
echo   AI 小说矩阵工作室 - 启动中...
echo ========================================
echo.

:: 检查 Python
python --version >nul 2>&1
if errorlevel 1 (
    echo [错误] 未找到 Python，请先安装 Python 3.10+
    pause
    exit /b 1
)

:: 安装 GUI 依赖
echo [1/2] 检查依赖...
pip install PyQt6 SQLAlchemy PyMySQL cryptography -q
if errorlevel 1 (
    echo [错误] 依赖安装失败
    pause
    exit /b 1
)

echo [2/2] 启动应用...
echo.
python -m ai_novel_studio.gui.app

if errorlevel 1 (
    echo.
    echo [错误] 应用启动失败，请检查错误信息
    pause
)

@echo off
REM ===================================================
REM AI Novel Agent v3.0.1 — 发布脚本
REM 用法：在项目根目录执行 .\scripts\publish-3.0.1.bat
REM ===================================================
echo ===== 1. 编译多平台二进制 (make cross) =====
make cross
if %ERRORLEVEL% NEQ 0 (
    echo 编译失败！请检查 Go 环境。
    exit /b 1
)

echo.
echo ===== 2. 复制 Windows 二进制到 npm/dist/ =====
copy /Y dist\novel-agent-windows-amd64.exe npm\dist\novel-agent_windows_amd64.exe
if %ERRORLEVEL% NEQ 0 (
    echo 复制失败！
    exit /b 1
)

echo.
echo ===== 3. npm publish =====
cd npm
npm publish --access public
if %ERRORLEVEL% NEQ 0 (
    echo 发布失败！请检查 npm 登录状态 (npm whoami)。
    exit /b 1
)

echo.
echo ===== 发布完成！=====
echo novel-agent-cli@3.0.1 已发布到 npm。
echo 验证: npm info novel-agent-cli version
cd ..

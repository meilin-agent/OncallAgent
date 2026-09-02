@echo off
title OncallAgent 一键停止
cd /d "%~dp0"

echo ==========================================
echo    智能 OncallAgent 一键停止
echo ==========================================
echo.

rem ---------- 1. 停止本地进程（后端 6872 / 前端 8080 / 告警模拟 2112） ----------
echo [1/3] 停止本地进程...
for %%p in (6872 8080 2112) do (
    for /f "tokens=5" %%a in ('netstat -ano ^| findstr ":%%p " ^| findstr "LISTENING"') do (
        rem 只结束 go/python/testserver 进程，避免误杀 Docker 等系统进程
        tasklist /FI "PID eq %%a" /FO CSV 2>nul | findstr /I "python go main testserver" >nul 2>&1
        if not errorlevel 1 (
            taskkill /PID %%a /F >nul 2>&1
            echo    已结束端口 %%p 的进程 PID %%a
        )
    )
)

rem ---------- 2. 停止 Docker 容器 ----------
echo [2/3] 停止 Docker 容器 ^(Milvus / Prometheus / test-server^)...
docker info >nul 2>&1
if not errorlevel 1 (
    cd /d "%~dp0manifest\docker"
    docker compose stop 2>&1 | findstr /I "Stopped Stopping"
    cd /d "%~dp0"
) else (
    echo    Docker Desktop 未运行，跳过容器停止
)

rem ---------- 3. 收尾 ----------
echo [3/3] 停止完成！Ollama 保持不变，可随时重新运行 start-oncall.bat
echo.
pause

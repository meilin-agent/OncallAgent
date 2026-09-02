@echo off
setlocal EnableDelayedExpansion
title OncallAgent 一键启动
cd /d "%~dp0"

echo ==========================================
echo    智能 OnCall Agent 一键启动
echo    Milvus + Prometheus + 告警模拟 + 后端 + 前端
echo ==========================================
echo.

rem ========== 1. 确保 Docker Desktop 运行 ==========
docker info >nul 2>&1
if not errorlevel 1 goto docker_ok
echo [!] Docker Desktop 未运行，正在启动...
rem 兼容两种常见安装路径（系统级 / 用户级）
if exist "C:\Program Files\Docker\Docker\Docker Desktop.exe" (
    start "" "C:\Program Files\Docker\Docker\Docker Desktop.exe"
) else (
    start "" "%LOCALAPPDATA%\Programs\DockerDesktop\Docker Desktop.exe"
)
call :wait_docker
if errorlevel 1 (
    echo [x] Docker Desktop 启动超时 ^(200 秒^)，请手动启动后重新运行本脚本
    pause
    exit /b 1
)
:docker_ok
echo [ok] Docker Desktop 运行中

rem ========== 2. 端口占用检查 ==========
for %%p in (6872 8080 2112) do (
    netstat -ano | findstr ":%%p " | findstr "LISTENING" >nul 2>&1
    if not errorlevel 1 (
        echo [x] 端口 %%p 已被占用，可能已有服务在运行，请先运行 stop-oncall.bat
        pause
        exit /b 1
    )
)

rem ========== 3. 配置与依赖检查 ==========
if not exist "manifest\config\config.yaml" (
    echo [x] 缺少 manifest\config\config.yaml，请复制 config.example.yaml 并填写 API Key
    pause
    exit /b 1
)
go version >nul 2>&1
if errorlevel 1 ( echo [x] 未找到 Go，请安装 Go 1.26+ & pause & exit /b 1 )
python --version >nul 2>&1
if errorlevel 1 ( echo [x] 未找到 Python，请安装 Python 3 & pause & exit /b 1 )
curl --version >nul 2>&1
if errorlevel 1 ( echo [x] 未找到 curl & pause & exit /b 1 )

rem ========== 4. 启动 Docker 容器栈（Milvus / Prometheus） ==========
echo.
echo [1/6] 启动 Docker 容器 ^(Milvus / Prometheus^)...
cd /d "%~dp0manifest\docker"
docker compose up -d etcd minio standalone prometheus
if errorlevel 1 (
    echo [x] Docker 容器启动失败，请检查 Docker 是否正常
    pause
    exit /b 1
)
rem 确保 Prometheus 加载最新配置（host.docker.internal 抓取宿主机）
docker compose restart prometheus >nul 2>&1
cd /d "%~dp0"

rem ========== 5. 等待 Milvus 就绪（最长 60 秒） ==========
echo [2/6] 等待 Milvus 就绪...
set /a tries=0
:wait_milvus
set /a tries+=1
if !tries! gtr 12 (
    echo [x] Milvus 启动超时，请检查容器状态: docker ps
    pause
    exit /b 1
)
netstat -ano | findstr ":19530 " | findstr "LISTENING" >nul 2>&1
if errorlevel 1 (
    ping -n 6 127.0.0.1 >nul
    goto wait_milvus
)
echo [ok] Milvus 已就绪

rem ========== 6. 启动告警模拟器 test-server（原生进程） ==========
echo [3/6] 准备告警模拟器 test-server...
if not exist "manifest\docker\testserver.exe" (
    echo     首次运行，正在编译 test-server（约 10 秒）...
    go build -o manifest\docker\testserver.exe manifest\docker\prometheusTestServer\main.go
    if errorlevel 1 ( echo [x] test-server 编译失败 & pause & exit /b 1 )
)
start "OncallAgent-testserver" /d "%~dp0manifest\docker" "%~dp0manifest\docker\testserver.exe"
set /a tries=0
:wait_testserver
set /a tries+=1
if !tries! gtr 10 (
    echo [x] test-server 启动失败
    pause
    exit /b 1
)
netstat -ano | findstr ":2112 " | findstr "LISTENING" >nul 2>&1
if errorlevel 1 (
    ping -n 3 127.0.0.1 >nul
    goto wait_testserver
)
echo [ok] 告警模拟器已启动 ^(约 20 秒后 Prometheus 触发 3 条模拟告警^)

rem ========== 7. 检查 Ollama（可选，缺了知识问答不可用） ==========
curl -s --max-time 3 http://localhost:11434/api/tags >nul 2>&1
if errorlevel 1 (
    echo [!] Ollama 未运行 —— 知识问答将不可用，请手动启动 Ollama
) else (
    echo [ok] Ollama 运行中
)

    echo     正在预热向量模型（首次约 10-30 秒）...
    set /a tries=0
    :warmup_loop
    set /a tries+=1
    if !tries! gtr 30 (
        echo [!] 模型预热超时，问答可能报错，请稍后重试
        goto warmup_done
    )
    python -c "import urllib.request,json;o=urllib.request.build_opener(urllib.request.ProxyHandler({}));req=urllib.request.Request('http://localhost:11434/api/embed',data=json.dumps({'model':'nomic-embed-text','input':'warmup'}).encode(),headers={'Content-Type':'application/json'});o.open(req,timeout=120)" >nul 2>&1
    if errorlevel 1 (
        ping -n 3 127.0.0.1 >nul
        goto warmup_loop
    )
    echo [ok] 向量模型已预热
    :warmup_done
rem ========== 8. 启动后端 ==========
echo [4/6] 启动后端 http://localhost:6872 ...
start "OncallAgent-Backend" /d "%~dp0" cmd /k "go run main.go"

rem ========== 9. 启动前端 ==========
echo [5/6] 启动前端 http://localhost:8080 ...
start "OncallAgent-Frontend" /d "%~dp0SuperBizAgentFrontend" cmd /k "python -m http.server 8080"

rem ========== 10. 等待后端就绪并打开浏览器 ==========
echo [6/6] 等待后端就绪...
set /a tries=0
:wait_backend
set /a tries+=1
if !tries! gtr 30 (
    echo [!] 后端启动较慢，请手动刷新浏览器
    goto done
)
curl -s --max-time 2 http://localhost:6872/api.json >nul 2>&1
if errorlevel 1 (
    ping -n 3 127.0.0.1 >nul
    goto wait_backend
)

:done
echo 启动完成！正在打开浏览器...
ping -n 3 127.0.0.1 >nul
start http://localhost:8080
echo.
echo ==========================================
echo   启动完成！演示完成后运行 stop-oncall.bat
echo   提示：告警模拟器启动约 20 秒后，
echo   Prometheus 才会触发模拟告警，再点"AI Ops"
echo ==========================================
pause
exit /b 0

rem ========== 子程序：等待 Docker 就绪 ==========
:wait_docker
set /a tries=0
:wait_docker_loop
set /a tries+=1
if !tries! gtr 40 (
    exit /b 1
)
ping -n 6 127.0.0.1 >nul
docker info >nul 2>&1
if errorlevel 1 goto wait_docker_loop
exit /b 0

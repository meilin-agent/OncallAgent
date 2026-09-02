@echo off
setlocal EnableDelayedExpansion
title SuperBizAgent һ������
cd /d "%~dp0"

echo ==========================================
echo    ���� SuperBizAgent һ������
echo    Milvus + Prometheus + �澯ģ�� + ��� + ǰ��
echo ==========================================
echo.

rem ========== 1. ȷ�� Docker Desktop ���� ==========
docker info >nul 2>&1
if not errorlevel 1 goto docker_ok
echo [!] Docker Desktop δ���У���������...
rem �������ֳ�����װ·����ϵͳ�� / �û�����
if exist "C:\Program Files\Docker\Docker\Docker Desktop.exe" (
    start "" "C:\Program Files\Docker\Docker\Docker Desktop.exe"
) else (
    start "" "%LOCALAPPDATA%\Programs\DockerDesktop\Docker Desktop.exe"
)
call :wait_docker
if errorlevel 1 (
    echo [x] Docker Desktop ������ʱ ^(200 ��^)�����ֶ��������������б��ű�
    pause
    exit /b 1
)
:docker_ok
echo [ok] Docker Desktop ������

rem ========== 2. �˿�ռ�ü�� ==========
for %%p in (6872 8080 2112) do (
    netstat -ano | findstr ":%%p " | findstr "LISTENING" >nul 2>&1
    if not errorlevel 1 (
        echo [x] �˿� %%p �ѱ�ռ�ã��������з��������У��������� stop-oncall.bat
        pause
        exit /b 1
    )
)

rem ========== 3. ������������� ==========
if not exist "manifest\config\config.yaml" (
    echo [x] ȱ�� manifest\config\config.yaml���븴�� config.example.yaml ����д API Key
    pause
    exit /b 1
)
go version >nul 2>&1
if errorlevel 1 ( echo [x] δ�ҵ� Go���밲װ Go 1.26+ & pause & exit /b 1 )
python --version >nul 2>&1
if errorlevel 1 ( echo [x] δ�ҵ� Python���밲װ Python 3 & pause & exit /b 1 )
curl --version >nul 2>&1
if errorlevel 1 ( echo [x] δ�ҵ� curl & pause & exit /b 1 )

rem ========== 4. ���� Docker ����ջ��Milvus / Prometheus�� ==========
echo.
echo [1/6] ���� Docker ���� ^(Milvus / Prometheus^)...
cd /d "%~dp0manifest\docker"
docker compose up -d etcd minio standalone prometheus
if errorlevel 1 (
    echo [x] Docker ��������ʧ�ܣ����� Docker �Ƿ�����
    pause
    exit /b 1
)
rem ȷ�� Prometheus �����������ã�host.docker.internal ץȡ��������
docker compose restart prometheus >nul 2>&1
cd /d "%~dp0"

rem ========== 5. �ȴ� Milvus ������� 60 �룩 ==========
echo [2/6] �ȴ� Milvus ����...
set /a tries=0
:wait_milvus
set /a tries+=1
if !tries! gtr 12 (
    echo [x] Milvus ������ʱ����������״̬: docker ps
    pause
    exit /b 1
)
netstat -ano | findstr ":19530 " | findstr "LISTENING" >nul 2>&1
if errorlevel 1 (
    ping -n 6 127.0.0.1 >nul
    goto wait_milvus
)
echo [ok] Milvus �Ѿ���

rem ========== 6. �����澯ģ���� test-server��ԭ�����̣� ==========
echo [3/6] ׼���澯ģ���� test-server...
if not exist "manifest\docker\testserver.exe" (
    echo     �״����У����ڱ��� test-server��Լ 10 �룩...
    go build -o manifest\docker\testserver.exe manifest\docker\prometheusTestServer\main.go
    if errorlevel 1 ( echo [x] test-server ����ʧ�� & pause & exit /b 1 )
)
start "SuperBizAgent-testserver" /d "%~dp0manifest\docker" "%~dp0manifest\docker\testserver.exe"
set /a tries=0
:wait_testserver
set /a tries+=1
if !tries! gtr 10 (
    echo [x] test-server ����ʧ��
    pause
    exit /b 1
)
netstat -ano | findstr ":2112 " | findstr "LISTENING" >nul 2>&1
if errorlevel 1 (
    ping -n 3 127.0.0.1 >nul
    goto wait_testserver
)
echo [ok] �澯ģ���������� ^(Լ 20 ��� Prometheus ���� 3 ��ģ��澯^)

rem ========== 7. ��� Ollama����ѡ��ȱ��֪ʶ�ʴ𲻿��ã� ==========
curl -s --max-time 3 http://localhost:11434/api/tags >nul 2>&1
if errorlevel 1 (
    echo [!] Ollama δ���� ���� ֪ʶ�ʴ𽫲����ã����ֶ����� Ollama
) else (
    echo [ok] Ollama ������
)

    echo     ����Ԥ������ģ�ͣ��״�Լ 10-30 �룩...
    set /a tries=0
    :warmup_loop
    set /a tries+=1
    if !tries! gtr 30 (
        echo [!] ģ��Ԥ�ȳ�ʱ���ʴ���ܱ��������Ժ�����
        goto warmup_done
    )
    python -c "import urllib.request,json;o=urllib.request.build_opener(urllib.request.ProxyHandler({}));req=urllib.request.Request('http://localhost:11434/api/embed',data=json.dumps({'model':'nomic-embed-text','input':'warmup'}).encode(),headers={'Content-Type':'application/json'});o.open(req,timeout=120)" >nul 2>&1
    if errorlevel 1 (
        ping -n 3 127.0.0.1 >nul
        goto warmup_loop
    )
    echo [ok] ����ģ����Ԥ��
    :warmup_done
rem ========== 8. ������� ==========
echo [4/6] ������� http://localhost:6872 ...
start "SuperBizAgent-Backend" /d "%~dp0" cmd /k "go run main.go"

rem ========== 9. ����ǰ�� ==========
echo [5/6] ����ǰ�� http://localhost:8080 ...
start "SuperBizAgent-Frontend" /d "%~dp0frontend" cmd /k "python -m http.server 8080"

rem ========== 10. �ȴ���˾������������ ==========
echo [6/6] �ȴ���˾���...
set /a tries=0
:wait_backend
set /a tries+=1
if !tries! gtr 30 (
    echo [!] ����������������ֶ�ˢ�������
    goto done
)
curl -s --max-time 2 http://localhost:6872/api.json >nul 2>&1
if errorlevel 1 (
    ping -n 3 127.0.0.1 >nul
    goto wait_backend
)

:done
echo ������ɣ����ڴ������...
ping -n 3 127.0.0.1 >nul
start http://localhost:8080
echo.
echo ==========================================
echo   ������ɣ���ʾ��ɺ����� stop-oncall.bat
echo   ��ʾ���澯ģ��������Լ 20 ���
echo   Prometheus �Żᴥ��ģ��澯���ٵ�"AI Ops"
echo ==========================================
pause
exit /b 0

rem ========== �ӳ��򣺵ȴ� Docker ���� ==========
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

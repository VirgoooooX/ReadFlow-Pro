@echo off
echo ========================================
echo ReadFlow Gateway - Quick Start
echo ========================================
echo.

REM 检查 Docker 是否安装
where docker >nul 2>nul
if %ERRORLEVEL% NEQ 0 (
    echo [ERROR] Docker is not installed or not in PATH
    echo Please install Docker Desktop from: https://www.docker.com/products/docker-desktop
    pause
    exit /b 1
)

echo [INFO] Docker found
echo.

REM 检查 Docker Compose 是否可用
docker compose version >nul 2>nul
if %ERRORLEVEL% NEQ 0 (
    echo [ERROR] Docker Compose is not available
    pause
    exit /b 1
)

echo [INFO] Docker Compose found
echo.

REM 创建必要的目录
if not exist "data" mkdir data
if not exist "static\images" mkdir static\images

echo [INFO] Created data directories
echo.

REM 提示用户修改配置
echo ========================================
echo IMPORTANT: Please update the following
echo in docker-compose.yml before first run:
echo.
echo 1. SERVER_PASSWORD (default: change_me_in_production)
echo 2. JWT_SECRET (default: your_jwt_secret_key_change_in_production)
echo ========================================
echo.
echo Press any key to continue with current settings...
pause >nul

echo.
echo [INFO] Starting ReadFlow Gateway...
docker compose up -d

if %ERRORLEVEL% EQU 0 (
    echo.
    echo ========================================
    echo ReadFlow Gateway started successfully!
    echo ========================================
    echo.
    echo Server is running at: http://localhost:8080
    echo Health check: http://localhost:8080/health
    echo.
    echo To view logs:    docker compose logs -f
    echo To stop server:  docker compose down
    echo ========================================
) else (
    echo.
    echo [ERROR] Failed to start ReadFlow Gateway
    echo Please check the error messages above
)

echo.
pause

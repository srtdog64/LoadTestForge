@echo off
setlocal enabledelayedexpansion

echo LoadTestForge Quick Start Script
echo =================================
echo.

where go >nul 2>nul
if %ERRORLEVEL% NEQ 0 (
    echo Error: Go is not installed. Please install Go 1.21 or later.
    exit /b 1
)

echo Step 1: Installing dependencies...
go mod download
go mod tidy

echo Step 2: Running tests...
go test -v ./...

echo Step 3: Building binary...
go build -o loadtest.exe ./cmd/loadtest

echo.
echo Build complete!
echo.
echo Run a test:
echo   loadtest.exe --target http://httpbin.org/get --sessions 100 --rate 10 --duration 30s
echo.
echo For more options:
echo   loadtest.exe --help
echo.

pause

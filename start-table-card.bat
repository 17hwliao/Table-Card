@echo off
setlocal
cd /d "%~dp0"
where go >nul 2>nul
if errorlevel 1 (
  echo Go was not found. Install Go, then run this script again.
  pause
  exit /b 1
)
go run ./cmd/table-card %*
pause

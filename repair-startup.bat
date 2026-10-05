@echo off
setlocal
chcp 65001 >nul
powershell.exe -NoProfile -ExecutionPolicy Bypass -File "%~dp0repair-startup.ps1" %*
set "repairExit=%errorlevel%"
pause
exit /b %repairExit%

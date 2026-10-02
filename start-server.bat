@echo off
setlocal
chcp 65001 >nul
powershell.exe -NoProfile -ExecutionPolicy Bypass -File "%~dp0start-table-card.ps1" -ServerOnly %*
if errorlevel 1 goto failed
echo.
echo This window shows the connection addresses. The server runs in the background.
pause
exit /b 0
:failed
pause
exit /b 1

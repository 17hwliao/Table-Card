@echo off
setlocal
chcp 65001 >nul
powershell.exe -NoProfile -ExecutionPolicy Bypass -File "%~dp0join-table-card.ps1" %*
if errorlevel 1 pause

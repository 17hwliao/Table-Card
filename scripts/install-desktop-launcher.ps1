[CmdletBinding()]
param(
    [string]$DesktopPath = [Environment]::GetFolderPath('Desktop'),
    [switch]$Shortcut
)
$ErrorActionPreference = 'Stop'
$projectRoot = Split-Path $PSScriptRoot -Parent
$entry = Join-Path $projectRoot 'start-table-card.ps1'
if (-not (Test-Path -LiteralPath $entry)) { throw "找不到项目启动器：$entry" }
if (-not (Test-Path -LiteralPath $DesktopPath -PathType Container)) { throw "找不到桌面目录：$DesktopPath" }
$batchPath = Join-Path $DesktopPath '启动牌桌.bat'
$escapedEntry = $entry.Replace('%', '%%')
$content = "@echo off`r`nsetlocal`r`nchcp 65001 >nul`r`npowershell.exe -NoProfile -ExecutionPolicy Bypass -File `"$escapedEntry`" -PromptClients %*`r`nif errorlevel 1 pause`r`n"
[IO.File]::WriteAllText($batchPath, $content, [Text.UTF8Encoding]::new($false))
Write-Host "已创建桌面启动脚本：$batchPath"
if ($Shortcut) {
    $shell = New-Object -ComObject WScript.Shell
    $link = $shell.CreateShortcut((Join-Path $DesktopPath '牌桌 Card Table.lnk'))
    $link.TargetPath = "$env:SystemRoot\System32\WindowsPowerShell\v1.0\powershell.exe"
    $link.Arguments = "-NoProfile -ExecutionPolicy Bypass -File `"$entry`" -PromptClients"
    $link.WorkingDirectory = $projectRoot
    $link.Description = '牌桌终端游戏：选择本地客户端数量后启动'
    $link.Save()
    Write-Host '已创建牌桌 Card Table 快捷方式。'
}

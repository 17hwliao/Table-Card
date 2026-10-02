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
$testBatchPath = Join-Path $DesktopPath '牌桌本地测试.bat'
$testContent = "@echo off`r`nsetlocal`r`nchcp 65001 >nul`r`npowershell.exe -NoProfile -ExecutionPolicy Bypass -File `"$escapedEntry`" -Clients 4 -Port 18781 %*`r`nif errorlevel 1 pause`r`n"
[IO.File]::WriteAllText($testBatchPath, $testContent, [Text.UTF8Encoding]::new($false))
Write-Host "已创建四人本地测试脚本：$testBatchPath"
$tetrisBatchPath = Join-Path $DesktopPath '牌桌俄罗斯方块.bat'
# Use a separate port so an older eight-mode service can keep its current games.
$tetrisContent = "@echo off`r`nsetlocal`r`nchcp 65001 >nul`r`necho Start the terminal lobby and select mode 9 - Tetris.`r`npowershell.exe -NoProfile -ExecutionPolicy Bypass -File `"$escapedEntry`" -PromptClients -Port 19881 %*`r`nif errorlevel 1 pause`r`n"
[IO.File]::WriteAllText($tetrisBatchPath, $tetrisContent, [Text.UTF8Encoding]::new($false))
Write-Host "已创建俄罗斯方块启动入口：$tetrisBatchPath"
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

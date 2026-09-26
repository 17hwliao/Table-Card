[CmdletBinding()]
param([ValidateRange(1, 65535)][int]$Port = 1781)
$ErrorActionPreference = 'Stop'
$projectRoot = [IO.Path]::GetFullPath((Split-Path $PSScriptRoot -Parent))
$recordPath = Join-Path $projectRoot "runtime/server-$Port.json"
if (-not (Test-Path -LiteralPath $recordPath)) { Write-Host '当前目录没有该端口的服务进程记录。'; exit 0 }
$record = Get-Content -LiteralPath $recordPath -Raw | ConvertFrom-Json
$backend = Get-Process -Id $record.ProcessId -ErrorAction SilentlyContinue
if ($backend) {
    $expectedPath = [IO.Path]::GetFullPath($record.Executable)
    $actualPath = $backend.Path
    $expectedStart = [datetime]::Parse($record.Started).ToUniversalTime()
    if (-not $expectedPath.StartsWith($projectRoot + [IO.Path]::DirectorySeparatorChar, [StringComparison]::OrdinalIgnoreCase) -or
        $actualPath -ine $expectedPath -or [Math]::Abs(($backend.StartTime.ToUniversalTime() - $expectedStart).TotalSeconds) -gt 1) {
        throw '进程信息与本项目启动记录不符，未执行停止。'
    }
    Stop-Process -Id $backend.Id -Force
    Write-Host "已停止本项目的服务（端口 $Port）。"
} else { Write-Host '记录中的服务已经退出。' }
Remove-Item -LiteralPath $recordPath

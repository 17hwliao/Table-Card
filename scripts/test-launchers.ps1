[CmdletBinding()]
param()
$ErrorActionPreference = 'Stop'
$projectRoot = Split-Path $PSScriptRoot -Parent
$previousData = $env:TABLE_CARD_DATA_DIR
$env:TABLE_CARD_DATA_DIR = Join-Path $projectRoot 'runtime/startup-smoke/data'
$windowsPowerShell = Join-Path $env:SystemRoot 'System32/WindowsPowerShell/v1.0/powershell.exe'
try {
    foreach ($stopShell in @('current','windows51')) {
        $listener = [Net.Sockets.TcpListener]::new([Net.IPAddress]::Loopback,0)
        $listener.Start(); $port = $listener.LocalEndpoint.Port; $listener.Stop()
        $recordPath = Join-Path $projectRoot "runtime/server-$port.json"
        $originalRecord = $null
        try {
            & $windowsPowerShell -NoProfile -ExecutionPolicy Bypass -File (Join-Path $projectRoot 'start-table-card.ps1') -ServerOnly -Port $port
            if ($LASTEXITCODE -ne 0) { throw 'Windows PowerShell launcher failed' }
            $originalRecord = [IO.File]::ReadAllText($recordPath)
            $record = $originalRecord | ConvertFrom-Json
            $health = Invoke-RestMethod "http://127.0.0.1:$port/api/health" -TimeoutSec 3
            $modes = Invoke-RestMethod "http://127.0.0.1:$port/api/modes" -TimeoutSec 3
            $addresses = Invoke-RestMethod "http://127.0.0.1:$port/api/connection-info" -TimeoutSec 3
            if ($health.protocol -ne 2 -or $modes.Count -ne 9 -or $addresses.local -ne "127.0.0.1:$port") {
                throw 'Server health, mode list or connection address check failed'
            }
            # A mismatched timestamp must still refuse to terminate a process.
            $record.Started = '2000-01-01T00:00:00.0000000Z'
            [IO.File]::WriteAllText($recordPath, ($record | ConvertTo-Json), [Text.UTF8Encoding]::new($true))
            $refused = $false
            try { & (Join-Path $PSScriptRoot 'stop-local-server.ps1') -Port $port } catch { $refused = $true }
            if (-not $refused -or -not (Get-Process -Id $record.ProcessId -ErrorAction SilentlyContinue)) {
                throw 'Process identity guard failed'
            }
            [IO.File]::WriteAllText($recordPath, $originalRecord, [Text.UTF8Encoding]::new($true))
            if ($stopShell -eq 'windows51') {
                & $windowsPowerShell -NoProfile -ExecutionPolicy Bypass -File (Join-Path $PSScriptRoot 'stop-local-server.ps1') -Port $port
                if ($LASTEXITCODE -ne 0) { throw 'Windows PowerShell stop failed' }
            } else {
                & (Join-Path $PSScriptRoot 'stop-local-server.ps1') -Port $port
            }
            if ((Test-Path -LiteralPath $recordPath) -or (Get-Process -Id $record.ProcessId -ErrorAction SilentlyContinue)) {
                throw 'Test server retained after stop'
            }
            Write-Host "Startup and stop check passed: $stopShell"
        } finally {
            if ($originalRecord -and (Test-Path -LiteralPath $recordPath)) {
                [IO.File]::WriteAllText($recordPath, $originalRecord, [Text.UTF8Encoding]::new($true))
                & (Join-Path $PSScriptRoot 'stop-local-server.ps1') -Port $port
            }
        }
    }
} finally { $env:TABLE_CARD_DATA_DIR = $previousData }

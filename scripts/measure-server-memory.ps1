[CmdletBinding()]
param([Parameter(Mandatory=$true)][string]$BeforeBinary)
$ErrorActionPreference = 'Stop'
$projectRoot = Split-Path $PSScriptRoot -Parent
$outputDirectory = Join-Path $projectRoot 'runtime/memory-server'
New-Item -ItemType Directory -Path $outputDirectory -Force | Out-Null
$afterBinary = Join-Path $outputDirectory 'server-after.exe'
$rows = @()
function Sample-Server($Process, [string]$Version, [string]$Stage) {
    $working = @(); $private = @()
    for ($i=0; $i -lt 15; $i++) { Start-Sleep -Milliseconds 100; $Process.Refresh(); if ($Process.HasExited) { throw 'Server probe exited unexpectedly' }; $working += [long]$Process.WorkingSet64; $private += [long]$Process.PrivateMemorySize64 }
    $working = @($working | Sort-Object); $private = @($private | Sort-Object)
    return [pscustomobject]@{ Version=$Version; Stage=$Stage; WorkingSetMedianMiB=[math]::Round($working[7]/1MB,2); PrivateMedianMiB=[math]::Round($private[7]/1MB,2) }
}
Push-Location $projectRoot
try {
    & go build -trimpath -ldflags '-s -w' -o $afterBinary ./cmd/table-card-server
    if ($LASTEXITCODE -ne 0) { throw 'Server build failed' }
    $sha = [Security.Cryptography.SHA256]::Create(); $token = 'e' * 64
    try { $digest = $sha.ComputeHash([Text.Encoding]::UTF8.GetBytes($token)) } finally { $sha.Dispose() }
    $playerId = ([BitConverter]::ToString($digest,0,16)).Replace('-','').ToLowerInvariant()
    $headers = @{ 'X-Player-ID'=$playerId; 'X-Player-Token'=$token }
    foreach ($version in @('before','after')) {
        $binary = $BeforeBinary; if ($version -eq 'after') { $binary = $afterBinary }
        $listener = [Net.Sockets.TcpListener]::new([Net.IPAddress]::Loopback,0); $listener.Start(); $port = $listener.LocalEndpoint.Port; $listener.Stop()
        $previousData = $env:TABLE_CARD_DATA_DIR; $env:TABLE_CARD_DATA_DIR = Join-Path $outputDirectory $version
        $process = $null
        try {
            $process = Start-Process -FilePath $binary -ArgumentList @('-listen',"127.0.0.1:$port") -WindowStyle Hidden -PassThru -RedirectStandardOutput (Join-Path $outputDirectory "$version.out.txt") -RedirectStandardError (Join-Path $outputDirectory "$version.err.txt")
            $baseUrl = "http://127.0.0.1:$port"; $modes = $null
            for ($attempt=0; $attempt -lt 50; $attempt++) { try { $modes = Invoke-RestMethod -Uri "$baseUrl/api/modes" -TimeoutSec 1; break } catch { Start-Sleep -Milliseconds 100 } }
            if (-not $modes) { throw 'Server readiness failed' }
            $rows += Sample-Server $process $version 'idle'
            $codes = @()
            for ($repeat=0; $repeat -lt 2; $repeat++) { foreach ($mode in $modes) {
                $body = @{ mode=$mode.id; seats=$mode.maxSeats; bots=($mode.maxSeats-1); playerId=$playerId; name='Memory probe' } | ConvertTo-Json -Compress
                $room = Invoke-RestMethod -Uri "$baseUrl/api/rooms" -Method Post -Headers $headers -ContentType 'application/json' -Body $body
                $ready = @{ playerId=$playerId; ready=$true } | ConvertTo-Json -Compress
                Invoke-RestMethod -Uri "$baseUrl/api/rooms/$($room.code)/ready" -Method Post -Headers $headers -ContentType 'application/json' -Body $ready | Out-Null
                Invoke-RestMethod -Uri "$baseUrl/api/rooms/$($room.code)/start" -Method Post -Headers $headers -ContentType 'application/json' -Body '{}' | Out-Null
                $codes += $room.code
            } }
            $rows += Sample-Server $process $version '18-live-rooms'
            foreach ($code in $codes) { $body = @{ playerId=$playerId } | ConvertTo-Json -Compress; Invoke-RestMethod -Uri "$baseUrl/api/rooms/$code/leave" -Method Post -Headers $headers -ContentType 'application/json' -Body $body | Out-Null }
            $rows += Sample-Server $process $version 'rooms-released'
        } finally {
            if ($process) { $process.Refresh(); if (-not $process.HasExited) { Stop-Process -Id $process.Id }; $process.Dispose() }
            $env:TABLE_CARD_DATA_DIR = $previousData
        }
    }
    $rows | Export-Csv -LiteralPath (Join-Path $outputDirectory 'memory.csv') -Encoding UTF8 -NoTypeInformation
    $rows | Format-Table -AutoSize
} finally { Pop-Location }

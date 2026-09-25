[CmdletBinding()]
param(
    [ValidateRange(1, 16)]
    [int]$Clients = 1,

    [ValidateRange(1, 65535)]
    [int]$Port = 1781
)

$ErrorActionPreference = 'Stop'
$projectRoot = $PSScriptRoot
$binDir = Join-Path $projectRoot 'bin'
$serverPath = Join-Path $binDir 'table-card-server.exe'
$clientPath = Join-Path $binDir 'table-card.exe'
$serverLog = Join-Path $binDir 'server.log'
$serverErrorLog = Join-Path $binDir 'server-error.log'
$serverProcess = $null
$ownsServer = $false
$clientProcesses = @()

function Test-ServerReady {
    try {
        $response = Invoke-WebRequest -Uri "http://127.0.0.1:$Port/api/health" -TimeoutSec 2 -UseBasicParsing
        return $response.StatusCode -eq 200
    }
    catch {
        return $false
    }
}

function Test-PortListening {
    $probe = [System.Net.Sockets.TcpClient]::new()
    try {
        $task = $probe.ConnectAsync('127.0.0.1', $Port)
        return $task.Wait(800) -and $probe.Connected
    }
    catch {
        return $false
    }
    finally {
        $probe.Dispose()
    }
}

try {
    if (-not (Get-Command go -ErrorAction SilentlyContinue)) {
        throw '未找到 Go。请先安装 Go 1.26 或更高版本。'
    }

    New-Item -ItemType Directory -Force -Path $binDir | Out-Null
    Push-Location $projectRoot
    try {
        Write-Host '正在构建终端牌桌服务端和客户端……'
        go build -o $serverPath ./cmd/table-card-server
        if ($LASTEXITCODE -ne 0) { throw '服务端构建失败。' }
        go build -o $clientPath ./cmd/table-card
        if ($LASTEXITCODE -ne 0) { throw '终端客户端构建失败。' }
    }
    finally {
        Pop-Location
    }

    if (Test-ServerReady) {
        Write-Host "检测到 $Port 端口上的牌桌服务，直接连接。"
    }
    else {
        if (Test-PortListening) {
            throw "$Port 端口已被其他程序占用。可运行脚本时指定 -Port 使用其他端口。"
        }

        Write-Host "正在启动终端服务端（端口 $Port）……"
        $serverProcess = Start-Process `
            -FilePath $serverPath `
            -ArgumentList @('-listen', ":$Port") `
            -WorkingDirectory $projectRoot `
            -WindowStyle Hidden `
            -RedirectStandardOutput $serverLog `
            -RedirectStandardError $serverErrorLog `
            -PassThru
        $ownsServer = $true

        $deadline = (Get-Date).AddSeconds(30)
        while ((Get-Date) -lt $deadline -and -not (Test-ServerReady)) {
            $serverProcess.Refresh()
            if ($serverProcess.HasExited) {
                $details = Get-Content $serverErrorLog -Raw -ErrorAction SilentlyContinue
                throw "服务端已退出：$details"
            }
            Start-Sleep -Milliseconds 500
        }
        if (-not (Test-ServerReady)) {
            throw "服务端没有在 30 秒内启动。日志：$serverLog 和 $serverErrorLog"
        }
    }

    Write-Host "已连接牌桌服务。正在打开 $Clients 个终端客户端窗口……"
    for ($index = 1; $index -le $Clients; $index++) {
        $clientProcesses += Start-Process `
            -FilePath $clientPath `
            -ArgumentList @('-server', "localhost:$Port", '-name', "玩家$index") `
            -WorkingDirectory $projectRoot `
            -PassThru
        Write-Host "已打开客户端 $index / $Clients。"
        Start-Sleep -Milliseconds 250
    }

    Write-Host '关闭所有客户端窗口后，本脚本会停止本次启动的服务端。无需 Docker 或 Redis。'
    while (($clientProcesses | Where-Object { -not $_.HasExited }).Count -gt 0) {
        Start-Sleep -Seconds 1
    }
}
catch {
    Write-Host "启动失败：$($_.Exception.Message)" -ForegroundColor Red
    exit 1
}
finally {
    if ($ownsServer -and $serverProcess) {
        $serverProcess.Refresh()
        if (-not $serverProcess.HasExited) {
            Stop-Process -Id $serverProcess.Id -Force -ErrorAction SilentlyContinue
        }
    }
}

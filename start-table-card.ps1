[CmdletBinding()]
param(
    [ValidateRange(1, 16)][int]$Clients = 1,
    [ValidateRange(1, 65535)][int]$Port = 1781,
    [string]$Server = '',
    [switch]$PromptClients,
    [switch]$ServerOnly
)
$ErrorActionPreference = 'Stop'
$projectRoot = $PSScriptRoot
$binDir = Join-Path $projectRoot 'bin'
$runtimeDir = Join-Path $projectRoot 'runtime'

function Test-CompatibleServer([string]$Address) {
    try {
        $health = Invoke-RestMethod -Uri "http://$Address/api/health" -TimeoutSec 3
        return ($health.service -ceq 'table-card' -and $health.protocol -eq 2)
    } catch { return $false }
}

function Test-PortListening([int]$TargetPort) {
    $probe = [System.Net.Sockets.TcpClient]::new()
    try { return $probe.ConnectAsync('127.0.0.1', $TargetPort).Wait(800) -and $probe.Connected }
    catch { return $false }
    finally { $probe.Dispose() }
}

function Show-ConnectionAddresses([string]$LocalAddress, [int]$ListenPort) {
    $info = $null
    try {
        $info = Invoke-RestMethod -Uri "http://$LocalAddress/api/connection-info" -TimeoutSec 3
        if ($info.PSObject.Properties.Name -notcontains 'addresses') { $info = $null }
    } catch { $info = $null }
    if ($null -eq $info) {
        # Match actual TCP bindings before displaying addresses for an older service.
        $rows = @()
        try {
            $listeners = @([Net.NetworkInformation.IPGlobalProperties]::GetIPGlobalProperties().GetActiveTcpListeners() | Where-Object { $_.Port -eq $ListenPort })
            $wildcard = @($listeners | Where-Object { $_.Address.Equals([Net.IPAddress]::Any) -or $_.Address.Equals([Net.IPAddress]::IPv6Any) }).Count -gt 0
            foreach ($adapter in [Net.NetworkInformation.NetworkInterface]::GetAllNetworkInterfaces()) {
                if ($adapter.OperationalStatus -ne [Net.NetworkInformation.OperationalStatus]::Up -or $adapter.NetworkInterfaceType -eq [Net.NetworkInformation.NetworkInterfaceType]::Loopback) { continue }
                $properties = $adapter.GetIPProperties()
                $priority = 1
                if (@($properties.GatewayAddresses).Count -gt 0) { $priority = 0 }
                if ($adapter.Description -match '(?i)virtual|vmware|vpn|wsl|docker|tailscale|zerotier') { $priority = 2 }
                foreach ($entry in $properties.UnicastAddresses) {
                    $ip = $entry.Address
                    if ($ip.AddressFamily -ne [Net.Sockets.AddressFamily]::InterNetwork -or [Net.IPAddress]::IsLoopback($ip) -or $ip.ToString() -match '^(169\.254\.|0\.)') { continue }
                    $bound = @($listeners | Where-Object { $_.Address.Equals($ip) }).Count -gt 0
                    if ($wildcard -or $bound) { $rows += [pscustomobject]@{ address = "$($ip):$ListenPort"; interface = $adapter.Name; priority = $priority } }
                }
            }
        } catch { Write-Host '无法读取网卡地址，请检查当前网络连接。' -ForegroundColor Yellow }
        $info = [pscustomobject]@{ local = $LocalAddress; addresses = @($rows | Sort-Object priority, address -Unique) }
    }
    Write-Host ''
    Write-Host '========== 玩家连接地址 ==========' -ForegroundColor Green
    $lines = @('牌桌服务端 · 玩家连接地址')
    if ($info.local) {
        Write-Host "仅房主本机使用：$($info.local)"
        $lines += "仅房主本机使用：$($info.local)"
    }
    if (@($info.addresses).Count -gt 0) {
        Write-Host '同一局域网的玩家，在 join-table-card.bat 输入下方 IP:端口：'
        $lines += '同一局域网玩家输入以下地址：'
        foreach ($entry in $info.addresses) {
            Write-Host "  $($entry.address)  [$($entry.interface)]" -ForegroundColor Cyan
            $lines += "$($entry.address)  [$($entry.interface)]"
        }
        Write-Host '多个地址时，选择与玩家同一网络的 Wi-Fi / 以太网地址。'
    } else {
        Write-Host '没有可分享的局域网地址；请检查网络连接和监听范围。' -ForegroundColor Yellow
        $lines += '没有可分享的局域网地址。'
    }
    $lines += '客户端连接后，在大厅输入房间号加入。'
    $lines += '连接需处于可访问的网络，并允许防火墙通过所用端口。'
    New-Item -ItemType Directory -Path $runtimeDir -Force | Out-Null
    $addressFile = Join-Path $runtimeDir "server-$ListenPort-addresses.txt"
    [IO.File]::WriteAllText($addressFile, ($lines -join "`r`n"), [Text.UTF8Encoding]::new($true))
    Write-Host "地址已保存：$addressFile"
    Write-Host '连接后再用房间号加入；网络及防火墙需允许对应端口。'
    Write-Host '==================================' -ForegroundColor Green
    Write-Host ''
}

function Get-Binaries {
    $candidates = @($binDir)
    $buildsDir = Join-Path $binDir 'builds'
    if (Test-Path -LiteralPath $buildsDir) {
        $candidates += @(Get-ChildItem -LiteralPath $buildsDir -Directory | ForEach-Object { $_.FullName })
    }
    $available = @(foreach ($directory in $candidates) {
        $client = Join-Path $directory 'table-card.exe'
        $backend = Join-Path $directory 'table-card-server.exe'
        if ((Test-Path -LiteralPath $client) -and (Test-Path -LiteralPath $backend)) {
            $written = @((Get-Item -LiteralPath $client).LastWriteTimeUtc, (Get-Item -LiteralPath $backend).LastWriteTimeUtc) | Sort-Object | Select-Object -First 1
            [pscustomobject]@{ Directory = $directory; Written = $written }
        }
    })
    $latest = $available | Sort-Object Written -Descending | Select-Object -First 1
    $hasSource = Test-Path -LiteralPath (Join-Path $projectRoot 'go.mod')
    $sourceNewest = [datetime]::MinValue
    if ($hasSource) {
        $sourceFiles = @(Get-Item -LiteralPath (Join-Path $projectRoot 'go.mod'))
        foreach ($name in @('go.sum', 'cmd', 'internal')) {
            $sourcePath = Join-Path $projectRoot $name
            if (Test-Path -LiteralPath $sourcePath) {
                $sourceFiles += @(Get-ChildItem -LiteralPath $sourcePath -Recurse -File)
            }
        }
        $sourceNewest = ($sourceFiles | Sort-Object LastWriteTimeUtc -Descending | Select-Object -First 1).LastWriteTimeUtc
    }
    $needsBuild = (-not $latest) -or ($hasSource -and $sourceNewest -gt $latest.Written)
    if ($needsBuild -and $hasSource -and (Get-Command go -ErrorAction SilentlyContinue)) {
        # Versioned directories avoid replacing executables used by open windows.
        $buildDir = Join-Path $buildsDir ((Get-Date -Format 'yyyyMMdd-HHmmss-fff') + '-' + [guid]::NewGuid().ToString('N').Substring(0, 6))
        New-Item -ItemType Directory -Path $buildDir -Force | Out-Null
        Push-Location $projectRoot
        try {
            Write-Host '正在编译最新终端客户端和服务端……'
            & go build -o (Join-Path $buildDir 'table-card-server.exe') ./cmd/table-card-server
            if ($LASTEXITCODE -ne 0) { throw '服务端编译失败。' }
            & go build -o (Join-Path $buildDir 'table-card.exe') ./cmd/table-card
            if ($LASTEXITCODE -ne 0) { throw '客户端编译失败。' }
        } finally { Pop-Location }
        return $buildDir
    }
    if (-not $latest) { throw '没有可用程序。请使用完整 Windows 便携包，或安装 Go 1.26 后从源码启动。' }
    if ($needsBuild -and $hasSource) { Write-Host '源码已有更新，但未安装 Go；本次运行现有已编译版本。' -ForegroundColor Yellow }
    return $latest.Directory
}

try {
    if ($PromptClients -and -not $ServerOnly) {
        while ($true) {
            $answer = Read-Host '打开几个终端客户端？输入 1–16，直接回车打开 1 个'
            if ([string]::IsNullOrWhiteSpace($answer)) { $Clients = 1; break }
            $requestedClients = 0
            if ([int]::TryParse($answer, [ref]$requestedClients) -and $requestedClients -ge 1 -and $requestedClients -le 16) {
                $Clients = $requestedClients; break
            }
            Write-Host '请输入 1 到 16 之间的整数。' -ForegroundColor Yellow
        }
    }
    $remote = -not [string]::IsNullOrWhiteSpace($Server)
    if ($ServerOnly -and $remote) { throw '-ServerOnly 用于开启本地服务，不可同时指定远程 -Server。' }
    if ($remote) {
        $Server = $Server.Trim()
        $uri = $null
        if (-not [uri]::TryCreate("http://$Server", [System.UriKind]::Absolute, [ref]$uri) -or
            $uri.UserInfo -ne '' -or $uri.AbsolutePath -ne '/' -or $uri.Query -ne '' -or $uri.Fragment -ne '' -or
            $Server -match '[\s/\\"'']' -or $Server -notmatch ':\d+$' -or $uri.Port -lt 1) {
            throw '服务器地址格式应为主机:端口，例如 192.168.1.20:1781。'
        }
        $address = $Server
    } else { $address = "127.0.0.1:$Port" }

    $programDir = Get-Binaries
    $clientPath = Join-Path $programDir 'table-card.exe'
    $serverPath = Join-Path $programDir 'table-card-server.exe'
    if ($remote) {
        if (-not (Test-CompatibleServer $address)) { throw "无法连接兼容的牌桌服务：$address。服务需返回 service=table-card、protocol=2。" }
    } elseif (Test-CompatibleServer $address) {
        Write-Host "已发现本地牌桌服务（$Port），客户端将直接连接。"
    } else {
        if (Test-PortListening $Port) { throw "端口 $Port 已被其他程序或旧版牌桌服务占用。请关闭旧服务，或指定 -Port 使用其他端口。" }
        New-Item -ItemType Directory -Path $runtimeDir -Force | Out-Null
        $logPath = Join-Path $runtimeDir "server-$Port.log"
        $errorPath = Join-Path $runtimeDir "server-$Port-error.log"
        $backend = Start-Process -FilePath $serverPath -ArgumentList @('-listen', ":$Port") `
            -WorkingDirectory $projectRoot -WindowStyle Hidden -RedirectStandardOutput $logPath -RedirectStandardError $errorPath -PassThru
        [pscustomobject]@{ ProcessId = $backend.Id; Executable = $serverPath; Started = $backend.StartTime.ToUniversalTime().ToString('o'); Port = $Port } |
            ConvertTo-Json | Set-Content -LiteralPath (Join-Path $runtimeDir "server-$Port.json") -Encoding UTF8
        $deadline = (Get-Date).AddSeconds(30)
        while ((Get-Date) -lt $deadline -and -not (Test-CompatibleServer $address)) {
            $backend.Refresh()
            if ($backend.HasExited) { throw "服务端已退出。请查看日志：$errorPath" }
            Start-Sleep -Milliseconds 300
        }
        if (-not (Test-CompatibleServer $address)) { throw "服务启动超时。请查看 $errorPath；可运行 scripts/stop-local-server.ps1 -Port $Port 停止本次服务。" }
    }

    if (-not $remote) { Show-ConnectionAddresses $address $Port }
    if (-not $ServerOnly) {
        Write-Host "正在打开 $Clients 个终端窗口，服务器：$address"
        for ($index = 1; $index -le $Clients; $index++) {
            Start-Process -FilePath $clientPath -ArgumentList @('-server', $address, '-name', "玩家$index") -WorkingDirectory $projectRoot -WindowStyle Normal | Out-Null
            Start-Sleep -Milliseconds 150
        }
    }
    if (-not $remote) {
        Write-Host "服务保持后台运行，可再次双击脚本增加客户端。停止命令：.\scripts\stop-local-server.ps1 -Port $Port"
    }
} catch {
    Write-Host "启动失败：$($_.Exception.Message)" -ForegroundColor Red
    exit 1
}

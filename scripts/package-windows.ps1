[CmdletBinding()]
param([ValidateSet('amd64', 'arm64')][string]$Architecture = 'amd64')
$ErrorActionPreference = 'Stop'
$projectRoot = Split-Path $PSScriptRoot -Parent
if (-not (Get-Command go -ErrorAction SilentlyContinue)) { throw '制作便携包需要安装 Go；收到压缩包的玩家无需安装 Go。' }

$stamp = (Get-Date -Format 'yyyyMMdd-HHmmss') + '-' + [guid]::NewGuid().ToString('N').Substring(0, 6)
$distDir = Join-Path $projectRoot 'dist'
$hostName = "Table-Card-Host-Windows-$Architecture-$stamp"
$clientName = "Table-Card-Client-Windows-$Architecture-$stamp"
$hostStage = Join-Path $distDir $hostName
$clientStage = Join-Path $distDir $clientName
New-Item -ItemType Directory -Path (Join-Path $hostStage 'bin'), (Join-Path $hostStage 'scripts'), (Join-Path $clientStage 'bin') -Force | Out-Null

$previousGOOS, $previousGOARCH, $previousCGO = $env:GOOS, $env:GOARCH, $env:CGO_ENABLED
Push-Location $projectRoot
try {
    $env:GOOS = 'windows'; $env:GOARCH = $Architecture; $env:CGO_ENABLED = '0'
    & go build -trimpath -ldflags '-s -w' -o (Join-Path $hostStage 'bin/table-card.exe') ./cmd/table-card
    if ($LASTEXITCODE -ne 0) { throw '客户端打包构建失败。' }
    & go build -trimpath -ldflags '-s -w' -o (Join-Path $hostStage 'bin/table-card-server.exe') ./cmd/table-card-server
    if ($LASTEXITCODE -ne 0) { throw '服务端打包构建失败。' }
} finally {
    $env:GOOS = $previousGOOS; $env:GOARCH = $previousGOARCH; $env:CGO_ENABLED = $previousCGO
    Pop-Location
}
Copy-Item -LiteralPath (Join-Path $hostStage 'bin/table-card.exe') -Destination (Join-Path $clientStage 'bin/table-card.exe')

foreach ($name in @('start-table-card.ps1', 'start-table-card.bat', 'start-snake.ps1', 'start-snake.bat', 'join-table-card.ps1', 'join-table-card.bat', 'README.md', 'LICENSE', 'THIRD_PARTY_NOTICES.md')) {
    Copy-Item -LiteralPath (Join-Path $projectRoot $name) -Destination (Join-Path $hostStage $name)
}
foreach ($name in @('start-snake.ps1', 'start-snake.bat', 'join-table-card.ps1', 'join-table-card.bat', 'LICENSE', 'THIRD_PARTY_NOTICES.md')) {
    Copy-Item -LiteralPath (Join-Path $projectRoot $name) -Destination (Join-Path $clientStage $name)
}
foreach ($name in @('install-desktop-launcher.ps1', 'stop-local-server.ps1')) {
    Copy-Item -LiteralPath (Join-Path $PSScriptRoot $name) -Destination (Join-Path $hostStage "scripts/$name")
}

$hostInstructions = @'
牌桌 Card Table · 房主包

1. 完整解压后，双击 start-table-card.bat，输入要打开的本机客户端数量。
2. 一个客户端创建房间，记住房间号，发给加入者。
3. 将电脑的局域网 IP 和端口 1781 告诉同一局域网内的玩家，例如 192.168.1.20:1781。
4. 给其他玩家发送单独的 Table-Card-Client-Windows 压缩包；他们解压后双击 join-table-card.bat。
5. 服务端会在后台保持运行。结束后运行 .\scripts\stop-local-server.ps1 停止。
6. 公网游玩需要玩家能访问房主的网络和 TCP 端口；本项目没有云端中继。
7. 单机贪吃蛇双击 start-snake.bat，无需启动服务端或输入网络地址。

使用者无需安装 Go、Docker 或 Redis。
源码及许可证：https://github.com/17hwliao/Table-Card
'@
$clientInstructions = @'
牌桌 Card Table · 玩家包

1. 完整解压本文件夹，不要只复制 EXE。
2. 双击 join-table-card.bat，输入房主给出的 IP:端口和你的昵称。
3. 在终端大厅选择相同游戏模式，输入房主给的 6 位房间号加入。
4. 默认端口是 1781；只输入 IP 时会自动补上该端口。
5. 如果连接失败，确认房主正在运行服务、地址正确，且网络或防火墙允许连接。
6. 离线玩贪吃蛇可直接双击 start-snake.bat，无需房主、网络或服务器。

玩家无需安装 Go、Docker、Redis 或其他运行环境。
源码及许可证：https://github.com/17hwliao/Table-Card
'@
[IO.File]::WriteAllText((Join-Path $hostStage '开始使用.txt'), $hostInstructions, [Text.UTF8Encoding]::new($true))
[IO.File]::WriteAllText((Join-Path $clientStage '开始使用.txt'), $clientInstructions, [Text.UTF8Encoding]::new($true))

$hostZip = Join-Path $distDir "$hostName.zip"
$clientZip = Join-Path $distDir "$clientName.zip"
Compress-Archive -LiteralPath $hostStage -DestinationPath $hostZip -CompressionLevel Optimal
Compress-Archive -LiteralPath $clientStage -DestinationPath $clientZip -CompressionLevel Optimal
Write-Host "房主包：$hostZip"
Write-Host "玩家包：$clientZip"
Write-Output $hostZip
Write-Output $clientZip

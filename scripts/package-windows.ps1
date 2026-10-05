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

foreach ($name in @('start-table-card.ps1', 'start-table-card.bat', 'start-server.bat', 'start-snake.ps1', 'start-snake.bat', 'start-pokemon.ps1', 'start-pokemon.bat', 'join-table-card.ps1', 'join-table-card.bat', 'repair-startup.ps1', 'repair-startup.bat', 'README.md', 'LICENSE', 'THIRD_PARTY_NOTICES.md')) {
    Copy-Item -LiteralPath (Join-Path $projectRoot $name) -Destination (Join-Path $hostStage $name)
}
foreach ($name in @('start-snake.ps1', 'start-snake.bat', 'start-pokemon.ps1', 'start-pokemon.bat', 'join-table-card.ps1', 'join-table-card.bat', 'repair-startup.ps1', 'repair-startup.bat', 'LICENSE', 'THIRD_PARTY_NOTICES.md')) {
    Copy-Item -LiteralPath (Join-Path $projectRoot $name) -Destination (Join-Path $clientStage $name)
}
foreach ($name in @('install-desktop-launcher.ps1', 'stop-local-server.ps1')) {
    Copy-Item -LiteralPath (Join-Path $PSScriptRoot $name) -Destination (Join-Path $hostStage "scripts/$name")
}
foreach ($stage in @($hostStage, $clientStage)) {
    $checksums = foreach ($name in @('table-card.exe', 'table-card-server.exe')) {
        $executable = Join-Path $stage "bin/$name"
        if (Test-Path -LiteralPath $executable -PathType Leaf) {
            (Get-FileHash -LiteralPath $executable -Algorithm SHA256).Hash.ToLowerInvariant() + "  bin/$name"
        }
    }
    [IO.File]::WriteAllText((Join-Path $stage 'PACKAGE-SHA256SUMS.txt'), ($checksums -join "`n") + "`n", [Text.UTF8Encoding]::new($false))
    $licenses = Join-Path $stage 'licenses'
    New-Item -ItemType Directory -Path $licenses -Force | Out-Null
    Copy-Item -LiteralPath (Join-Path $projectRoot 'internal/pokemon/POKEAPI_LICENSE.md') -Destination (Join-Path $licenses 'POKEAPI_LICENSE.md')
    Copy-Item -LiteralPath (Join-Path $projectRoot 'licenses/CC0-1.0.txt') -Destination (Join-Path $licenses 'CC0-1.0.txt')
    Copy-Item -LiteralPath (Join-Path $projectRoot 'docs/terminal-controls.md') -Destination (Join-Path $stage '终端全部操作.md')
    Copy-Item -LiteralPath (Join-Path $projectRoot 'docs/pokemon-text-adventure.md') -Destination (Join-Path $stage '宝可梦指令与规则.md')
    Copy-Item -LiteralPath (Join-Path $projectRoot 'docs/memory-and-test-report.md') -Destination (Join-Path $stage 'memory-and-test-report.md')
    foreach ($guide in @('terminal-controls.md', 'pokemon-text-adventure.md', 'game-audio.md')) {
        Copy-Item -LiteralPath (Join-Path $projectRoot "docs/$guide") -Destination (Join-Path $stage $guide)
    }
}
$hostDocs = Join-Path $hostStage 'docs'
New-Item -ItemType Directory -Path $hostDocs -Force | Out-Null
foreach ($name in @('mode-migration-status.md', 'pokemon-text-adventure.md', 'memory-and-test-report.md', 'terminal-controls.md', 'game-audio.md')) {
    Copy-Item -LiteralPath (Join-Path $projectRoot "docs/$name") -Destination (Join-Path $hostDocs $name)
}

$hostInstructions = @'
牌桌 Card Table · 房主包

1. 完整解压后，双击 start-table-card.bat，输入要打开的本机客户端数量。
2. 一个客户端创建房间，记住房间号，发给加入者。
3. 启动窗口会显示可以发给玩家的局域网 IP:端口和网卡名。将与玩家同一网络的地址复制发给他们，例如 192.168.1.20:1781。
4. 给其他玩家发送单独的 Table-Card-Client-Windows 压缩包；他们解压后双击 join-table-card.bat。
5. 服务端会在后台保持运行。结束后运行 .\scripts\stop-local-server.ps1 停止。
6. 公网游玩需要玩家能访问房主的网络和 TCP 端口；本项目没有云端中继。
7. 单机贪吃蛇双击 start-snake.bat，无需启动服务端或输入网络地址。
8. 只开启服务端可双击 start-server.bat，窗口会保留连接地址；地址同时保存在 runtime/server-1781-addresses.txt。
9. 宝可梦文字冒险双击 start-pokemon.bat，纯单机离线，进度按昵称保存；全程数字菜单，详细选项见“宝可梦指令与规则.md”。

使用者无需安装 Go、Docker 或 Redis。
若启动提示“操作已被用户取消”(1223)，核对 GitHub 发布来源和 ZIP 的 SHA-256 后，可双击 repair-startup.bat，输入 1 仅解除当前包 EXE 的下载锁定，再运行原启动脚本。
也可在下载 ZIP 的属性中解除锁定并重新解压。包内校验不等同于数字签名；本工具不会关闭系统安全保护。
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
7. 离线宝可梦文字冒险双击 start-pokemon.bat；全部行动支持数字菜单和英文字母，M地图 / C中心 / H指南（字母后按Enter），自动本地存档。

玩家无需安装 Go、Docker、Redis 或其他运行环境。
若启动提示“操作已被用户取消”(1223)，核对 GitHub 发布来源和 ZIP 的 SHA-256 后，可双击 repair-startup.bat，输入 1 仅解除当前包 EXE 的下载锁定，再运行原启动脚本。
也可在下载 ZIP 的属性中解除锁定并重新解压。包内校验不等同于数字签名；本工具不会关闭系统安全保护。
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

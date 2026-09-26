[CmdletBinding()]
param([ValidateSet('amd64', 'arm64')][string]$Architecture = 'amd64')
$ErrorActionPreference = 'Stop'
$projectRoot = Split-Path $PSScriptRoot -Parent
if (-not (Get-Command go -ErrorAction SilentlyContinue)) { throw '制作便携包需要安装 Go；接收便携包的玩家无需安装 Go。' }
$stamp = (Get-Date -Format 'yyyyMMdd-HHmmss') + '-' + [guid]::NewGuid().ToString('N').Substring(0, 6)
$distDir = Join-Path $projectRoot 'dist'
$packageName = "Table-Card-Windows-$Architecture-$stamp"
$stage = Join-Path $distDir $packageName
New-Item -ItemType Directory -Path (Join-Path $stage 'bin'), (Join-Path $stage 'scripts') -Force | Out-Null
$previousGOOS, $previousGOARCH, $previousCGO = $env:GOOS, $env:GOARCH, $env:CGO_ENABLED
Push-Location $projectRoot
try {
    $env:GOOS = 'windows'; $env:GOARCH = $Architecture; $env:CGO_ENABLED = '0'
    & go build -trimpath -o (Join-Path $stage 'bin/table-card.exe') ./cmd/table-card
    if ($LASTEXITCODE -ne 0) { throw '客户端打包构建失败。' }
    & go build -trimpath -o (Join-Path $stage 'bin/table-card-server.exe') ./cmd/table-card-server
    if ($LASTEXITCODE -ne 0) { throw '服务端打包构建失败。' }
} finally {
    $env:GOOS = $previousGOOS; $env:GOARCH = $previousGOARCH; $env:CGO_ENABLED = $previousCGO
    Pop-Location
}
foreach ($name in @('start-table-card.ps1', 'start-table-card.bat', 'README.md', 'LICENSE', 'THIRD_PARTY_NOTICES.md')) {
    $source = Join-Path $projectRoot $name
    if (Test-Path -LiteralPath $source -PathType Leaf) { Copy-Item -LiteralPath $source -Destination (Join-Path $stage $name) }
}
foreach ($name in @('install-desktop-launcher.ps1', 'stop-local-server.ps1')) {
    Copy-Item -LiteralPath (Join-Path $PSScriptRoot $name) -Destination (Join-Path $stage "scripts/$name")
}
$instructions = @'
牌桌 Card Table · Windows 便携版

1. 完整解压本文件夹后，双击 start-table-card.bat。
2. 无需安装 Go、Docker 或 Redis。
3. 多开：PowerShell 运行 .\start-table-card.ps1 -Clients 4。
4. 连接朋友的服务器：.\start-table-card.ps1 -Server 192.168.1.20:1781 -Clients 1。
5. 桌面入口：右键使用 PowerShell 运行 scripts/install-desktop-launcher.ps1。
6. 本地服务会在后台保持运行，关闭客户端后可再次打开；手动停止：.\scripts\stop-local-server.ps1。
7. 音乐已包含在程序中；配置与战绩数据保存在本地，分享时请使用原始压缩包。
'@
[IO.File]::WriteAllText((Join-Path $stage '开始使用.txt'), $instructions, [Text.UTF8Encoding]::new($true))
$zipPath = Join-Path $distDir "$packageName.zip"
Compress-Archive -LiteralPath $stage -DestinationPath $zipPath -CompressionLevel Optimal
Write-Host "便携包已生成：$zipPath"
Write-Output $zipPath

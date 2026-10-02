[CmdletBinding()]
param()
$ErrorActionPreference = 'Stop'
$projectRoot = $PSScriptRoot
$binaryRoot = Join-Path $projectRoot 'bin'
try {
    $candidates = @()
    if (Test-Path -LiteralPath $binaryRoot) {
        $candidates = @(Get-ChildItem -LiteralPath $binaryRoot -Filter 'table-card.exe' -Recurse -File | Sort-Object LastWriteTimeUtc -Descending)
    }
    $clientBinary = $candidates | Select-Object -First 1
    $hasSource = Test-Path -LiteralPath (Join-Path $projectRoot 'go.mod')
    $sourceNewest = [datetime]::MinValue
    if ($hasSource) {
        $sources = @(Get-Item -LiteralPath (Join-Path $projectRoot 'go.mod'))
        foreach ($name in @('go.sum', 'cmd', 'internal')) {
            $sourcePath = Join-Path $projectRoot $name
            if (Test-Path -LiteralPath $sourcePath) { $sources += @(Get-ChildItem -LiteralPath $sourcePath -Recurse -File) }
        }
        $sourceNewest = ($sources | Sort-Object LastWriteTimeUtc -Descending | Select-Object -First 1).LastWriteTimeUtc
    }
    $needsBuild = (-not $clientBinary) -or ($hasSource -and $sourceNewest -gt $clientBinary.LastWriteTimeUtc)
    if ($needsBuild -and $hasSource -and (Get-Command go -ErrorAction SilentlyContinue)) {
        $buildDirectory = Join-Path $binaryRoot ('builds/snake-' + (Get-Date -Format 'yyyyMMdd-HHmmss-fff'))
        New-Item -ItemType Directory -Path $buildDirectory -Force | Out-Null
        $clientPath = Join-Path $buildDirectory 'table-card.exe'
        Push-Location $projectRoot
        try {
            & go build -trimpath -ldflags '-s -w' -o $clientPath ./cmd/table-card
            if ($LASTEXITCODE -ne 0) { throw '客户端编译失败。' }
        } finally { Pop-Location }
    } else {
        if (-not $clientBinary) { throw '没有客户端程序。请使用完整便携包，或安装 Go 后从源码启动。' }
        if ($needsBuild -and $hasSource) { throw '源码已更新但无法编译；请安装 Go 或使用包含贪吃蛇的新便携包。' }
        $clientPath = $clientBinary.FullName
    }
    & $clientPath -snake
    if ($LASTEXITCODE -ne 0) { throw '贪吃蛇客户端已异常退出。' }
} catch {
    Write-Host "启动失败：$($_.Exception.Message)" -ForegroundColor Red
    exit 1
}

$ErrorActionPreference = 'Stop'

if (-not (Get-Command go -ErrorAction SilentlyContinue)) {
    throw '未找到 Go。请先安装 Go，然后重新运行此脚本。'
}

Push-Location $PSScriptRoot
try {
    go run ./cmd/table-card @args
} finally {
    Pop-Location
}

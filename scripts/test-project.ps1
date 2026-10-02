[CmdletBinding()]
param([switch]$Race, [switch]$Benchmarks, [switch]$Launchers)
$ErrorActionPreference = 'Stop'
$projectRoot = Split-Path $PSScriptRoot -Parent
$reportDirectory = Join-Path $projectRoot 'runtime/tests'
New-Item -ItemType Directory -Path $reportDirectory -Force | Out-Null
Push-Location $projectRoot
try {
    & go test ./... -count=1 -coverprofile (Join-Path $reportDirectory 'coverage.out') 2>&1 | Tee-Object (Join-Path $reportDirectory 'full.txt')
    if ($LASTEXITCODE -ne 0) { throw 'Full test suite failed' }
    & go tool cover -func (Join-Path $reportDirectory 'coverage.out') | Select-Object -Last 1
    & go vet ./... 2>&1 | Tee-Object (Join-Path $reportDirectory 'vet.txt')
    if ($LASTEXITCODE -ne 0) { throw 'Go vet failed' }
    if ($Race) {
        $previousPath = $env:PATH
        $previousFlags = $env:CGO_CFLAGS
        $previousCGO = $env:CGO_ENABLED
        try {
            # GCC 16 defaults to C23; Go's Windows cgo sources require GNU C11.
            $localGcc = 'C:/msys64/ucrt64/bin'
            if (Test-Path -LiteralPath (Join-Path $localGcc 'gcc.exe')) {
                $env:PATH = $localGcc + ';' + $env:PATH
            }
            if (-not (Get-Command gcc -ErrorAction SilentlyContinue)) { throw 'Race tests require a working MinGW GCC compiler' }
            $env:CGO_ENABLED = '1'
            $env:CGO_CFLAGS = '-O2 -g -std=gnu11'
            & go test -race ./... -count=1 2>&1 | Tee-Object (Join-Path $reportDirectory 'race.txt')
            if ($LASTEXITCODE -ne 0) { throw 'Race test suite failed' }
        } finally {
            $env:PATH = $previousPath
            $env:CGO_CFLAGS = $previousFlags
            $env:CGO_ENABLED = $previousCGO
        }
    }
    if ($Benchmarks) {
        & go test ./internal/terminal/app ./internal/terminal/audio ./internal/goengine ./internal/pokemon ./internal/server -run '^$' -bench . -benchmem -benchtime=500ms 2>&1 | Tee-Object (Join-Path $reportDirectory 'benchmarks.txt')
        if ($LASTEXITCODE -ne 0) { throw 'Benchmarks failed' }
    }
    if ($Launchers) {
        # Do not pipe the native Windows PowerShell launcher: inherited pipe
        # handles in a background server can keep its output reader waiting.
        & (Join-Path $PSScriptRoot 'test-launchers.ps1')
    }
    Write-Host "Reports: $reportDirectory"
} finally { Pop-Location }

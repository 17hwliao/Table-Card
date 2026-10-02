[CmdletBinding()]
param([string]$Label = 'sample')
$ErrorActionPreference = 'Stop'
$projectRoot = Split-Path $PSScriptRoot -Parent
$outputDirectory = Join-Path $projectRoot ('runtime/memory-' + $Label)
New-Item -ItemType Directory -Path $outputDirectory -Force | Out-Null
$clientProbe = Join-Path $outputDirectory 'client-probe.exe'
Push-Location $projectRoot
try {
    & go test -c -o $clientProbe ./internal/terminal/app
    if ($LASTEXITCODE -ne 0) { throw 'Memory probe build failed.' }
    $measurements = @()
    foreach ($scene in @('home','snake','pokemon','mahjong','go','tetris')) {
        $ready = Join-Path $outputDirectory ($scene + '-ready.txt')
        if (Test-Path -LiteralPath $ready) { Remove-Item -LiteralPath $ready }
        $previousScene, $previousReady = $env:TABLE_CARD_MEMORY_SCENE, $env:TABLE_CARD_MEMORY_READY
        $env:TABLE_CARD_MEMORY_SCENE = $scene; $env:TABLE_CARD_MEMORY_READY = $ready
        $process = $null
        try {
            $process = Start-Process -FilePath $clientProbe -ArgumentList @('-test.run=^TestMemoryProcessHelper$','-test.count=1') -WindowStyle Hidden -PassThru -RedirectStandardOutput (Join-Path $outputDirectory ($scene + '.out.txt')) -RedirectStandardError (Join-Path $outputDirectory ($scene + '.err.txt'))
            $deadline = (Get-Date).AddSeconds(20)
            while (-not (Test-Path -LiteralPath $ready)) {
                $process.Refresh()
                if ($process.HasExited -or (Get-Date) -gt $deadline) { throw "Probe $scene failed; see logs in $outputDirectory" }
                Start-Sleep -Milliseconds 100
            }
            $working = @(); $private = @()
            for ($sample = 0; $sample -lt 20; $sample++) {
                Start-Sleep -Milliseconds 200
                $process.Refresh()
                if ($process.HasExited) { throw "Probe $scene ended too early" }
                $working += [long]$process.WorkingSet64; $private += [long]$process.PrivateMemorySize64
            }
            $working = @($working | Sort-Object); $private = @($private | Sort-Object)
            $measurements += [pscustomobject]@{ Scene=$scene; WorkingSetMedianMiB=[math]::Round($working[10]/1MB,2); PrivateMedianMiB=[math]::Round($private[10]/1MB,2); WorkingSetPeakMiB=[math]::Round($working[-1]/1MB,2); PrivatePeakMiB=[math]::Round($private[-1]/1MB,2) }
            $process.WaitForExit()
            if ($process.ExitCode -ne 0) { throw "Probe $scene exited with $($process.ExitCode)" }
        } finally {
            if ($process) { $process.Refresh(); if (-not $process.HasExited) { Stop-Process -Id $process.Id }; $process.Dispose() }
            $env:TABLE_CARD_MEMORY_SCENE = $previousScene; $env:TABLE_CARD_MEMORY_READY = $previousReady
        }
    }
    $measurements | Export-Csv -NoTypeInformation -Encoding UTF8 -LiteralPath (Join-Path $outputDirectory 'memory.csv')
    $measurements | Format-Table -AutoSize
} finally { Pop-Location }

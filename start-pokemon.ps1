[CmdletBinding()]
param([string]$Name = '玩家 1')
$ErrorActionPreference = 'Stop'
& (Join-Path $PSScriptRoot 'start-snake.ps1') -Mode pokemon -Name $Name
exit $LASTEXITCODE

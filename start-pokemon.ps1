[CmdletBinding()]
param([string]$Name = '玩家 1', [string]$Server = 'localhost:1781')
$ErrorActionPreference = 'Stop'
& (Join-Path $PSScriptRoot 'start-snake.ps1') -Mode pokemon -Name $Name -Server $Server
exit $LASTEXITCODE

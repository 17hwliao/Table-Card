[CmdletBinding()]
param([string]$Root = '')
$ErrorActionPreference = 'Stop'
if (-not $Root) { $Root = Split-Path $PSScriptRoot -Parent }
$paths = @(Get-ChildItem -LiteralPath $Root -File -Filter '*.ps1') + @(Get-ChildItem -LiteralPath (Join-Path $Root 'scripts') -File -Filter '*.ps1')
foreach ($path in $paths) {
    $bytes = [IO.File]::ReadAllBytes($path.FullName)
    $nonAscii = @($bytes | Where-Object { $_ -gt 127 }).Count -gt 0
    $bom = $bytes.Length -ge 3 -and $bytes[0] -eq 239 -and $bytes[1] -eq 187 -and $bytes[2] -eq 191
    if ($nonAscii -and -not $bom) { throw "Windows PowerShell 5.1 requires a UTF-8 BOM: $($path.Name)" }
    $tokens = $null
    $errors = $null
    [Management.Automation.Language.Parser]::ParseFile($path.FullName, [ref]$tokens, [ref]$errors) | Out-Null
    if (@($errors).Count -gt 0) { throw "PowerShell parse failed: $($path.Name): $($errors[0].Message)" }
}
Write-Host "Encoding and syntax passed ($($paths.Count) scripts; PowerShell $($PSVersionTable.PSVersion))."

# Rebuild version-specific factual data with Python 3.
$ErrorActionPreference = 'Stop'
$generator = Join-Path $PSScriptRoot 'rebuild-pokemon-data.py'
if (Get-Command python -ErrorAction SilentlyContinue) {
    & python $generator
} elseif (Get-Command py -ErrorAction SilentlyContinue) {
    & py -3 $generator
} else {
    throw '重建资料需要 Python 3；已打包游戏运行无需 Python 或网络。'
}
if ($LASTEXITCODE -ne 0) { throw '宝可梦资料重建失败，详见上方错误。' }

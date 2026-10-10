[CmdletBinding()]
param(
    [string]$Server = '',
    [string]$Name = ''
)
$ErrorActionPreference = 'Stop'
$clientPath = Join-Path $PSScriptRoot 'bin/table-card.exe'

try {
    if (-not (Test-Path -LiteralPath $clientPath -PathType Leaf)) {
        throw '客户端程序不存在。请完整解压收到的“Table-Card-Client”压缩包后再运行。'
    }
    if ([string]::IsNullOrWhiteSpace($Server)) {
        $Server = Read-Host '请输入房主地址，例如 192.168.1.20:1781'
    }
    $Server = $Server.Trim()
    if ($Server -notmatch ':\d+$') { $Server += ':1781' }
    $uri = $null
    if ($Server -match '[\s/\\@?#]' -or
        -not [uri]::TryCreate("http://$Server", [System.UriKind]::Absolute, [ref]$uri) -or
        $uri.UserInfo -ne '' -or $uri.Host -eq '' -or $uri.AbsolutePath -ne '/' -or $uri.Port -lt 1) {
        throw '房主地址格式应为“IP或主机名:端口”，例如 192.168.1.20:1781。'
    }
    if ([string]::IsNullOrWhiteSpace($Name)) {
        $Name = Read-Host '游戏昵称（字母/数字即可，例如 Player1；直接回车默认 Player1）'
    }
    $Name = $Name.Trim()
    if ([string]::IsNullOrWhiteSpace($Name)) { $Name = 'Player1' }
    if ($Name.Length -lt 1 -or $Name.Length -gt 24) {
        throw '昵称需要为 1 到 24 个字符。'
    }
    try {
        $health = Invoke-RestMethod -Uri "http://$Server/api/health" -TimeoutSec 5
    } catch {
        throw "连接不到房主 $Server。请核对地址，并确认房主服务正在运行、网络和防火墙允许连接。"
    }
    if ($health.service -cne 'table-card' -or $health.protocol -ne 2 -or $health.rulesVersion -cne '2026.10.10-social-pokemon') {
        throw '服务端与当前玩家包不匹配。请房主更新本次维护版并重启服务端，再重新加入。'
    }
    Write-Host "正在连接 $Server，昵称：$Name"
    & $clientPath -server $Server -name $Name
    exit $LASTEXITCODE
} catch {
    Write-Host "启动失败：$($_.Exception.Message)" -ForegroundColor Red
    exit 1
}

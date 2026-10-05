[CmdletBinding()]
param([switch]$ConfirmTrusted)
$ErrorActionPreference = 'Stop'

try {
    $manifestPath = Join-Path $PSScriptRoot 'PACKAGE-SHA256SUMS.txt'
    if (-not (Test-Path -LiteralPath $manifestPath -PathType Leaf)) {
        throw '缺少 PACKAGE-SHA256SUMS.txt，请从 GitHub 发布页重新下载并完整解压。'
    }
    $expected = @{}
    foreach ($line in [IO.File]::ReadAllLines($manifestPath)) {
        if ([string]::IsNullOrWhiteSpace($line)) { continue }
        if ($line -notmatch '^([0-9a-fA-F]{64})  (bin/table-card(?:-server)?\.exe)$') {
            throw '包内校验清单格式错误，已停止修复。'
        }
        $hash, $name = $Matches[1], $Matches[2]
        if ($expected.ContainsKey($name)) { throw '校验清单存在重复程序，已停止修复。' }
        $expected[$name] = $hash
    }
    if (-not $expected.ContainsKey('bin/table-card.exe')) { throw '校验清单缺少客户端。' }
    if ((Test-Path -LiteralPath (Join-Path $PSScriptRoot 'bin/table-card-server.exe')) -and
        -not $expected.ContainsKey('bin/table-card-server.exe')) { throw '服务端未列入校验清单。' }

    $verified = @()
    $binPath = Join-Path $PSScriptRoot 'bin'
    if ((Get-Item -LiteralPath $binPath).Attributes -band [IO.FileAttributes]::ReparsePoint) {
        throw 'bin 目录为链接，已停止修复。请完整解压原始压缩包。'
    }
    foreach ($name in ($expected.Keys | Sort-Object)) {
        $path = Join-Path $PSScriptRoot $name
        $file = Get-Item -LiteralPath $path
        if ($file.PSIsContainer -or ($file.Attributes -band [IO.FileAttributes]::ReparsePoint)) {
            throw "程序不是普通文件：$name"
        }
        if ((Get-FileHash -LiteralPath $path -Algorithm SHA256).Hash -ne $expected[$name]) {
            throw "程序校验失败：$name。请重新下载原包，不会解除任何文件的锁定。"
        }
        $verified += $path
        Write-Host "校验通过：$name" -ForegroundColor Green
    }
    Write-Host '此工具仅解除当前包内已校验 EXE 的互联网下载锁定，不修改 Windows 安全设置。'
    Write-Host '包内校验只能检查文件一致性；请同时核对 GitHub 发布来源及发布页的 ZIP SHA-256。'
    if (-not $ConfirmTrusted) {
        $choice = Read-Host '已核对来源并信任此包？输入 1 继续；其他输入取消'
        if ($choice -cne '1') { Write-Host '已取消，文件未修改。'; exit 0 }
    }
    foreach ($path in $verified) {
        # Recheck immediately before modifying the download marker.
        $relative = 'bin/' + [IO.Path]::GetFileName($path)
        if ((Get-FileHash -LiteralPath $path -Algorithm SHA256).Hash -ne $expected[$relative]) {
            throw '校验后文件发生变化，已停止修复。'
        }
        Unblock-File -LiteralPath $path
        if (Get-Item -LiteralPath $path -Stream Zone.Identifier -ErrorAction SilentlyContinue) {
            throw "无法解除下载锁定：$path"
        }
    }
    Write-Host '修复完成。请再次双击原来的游戏启动 BAT。' -ForegroundColor Green
    Write-Host '若仍有系统提示，请阅读提示；程序未做数字签名，本工具不能保证消除所有安全提示。'
} catch {
    Write-Host "修复失败：$($_.Exception.Message)" -ForegroundColor Red
    exit 1
}

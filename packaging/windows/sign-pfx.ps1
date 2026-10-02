# Подписывает build/bin/norka.exe сертификатом PFX из секретов CI.
# WINDOWS_PFX_BASE64 — содержимое .pfx в base64, одной строкой.
# WINDOWS_PFX_PASSWORD — пароль контейнера. Пустой пароль допустим.
# Если base64 пустой, шаг ничего не делает (вызывающий workflow и так его пропускает).
$ErrorActionPreference = 'Stop'

$pfxB64 = $env:WINDOWS_PFX_BASE64
if ([string]::IsNullOrWhiteSpace($pfxB64)) {
    Write-Host 'WINDOWS_PFX_BASE64 is empty; skip PFX signing'
    exit 0
}

$exe = Join-Path (Get-Location) 'build\bin\norka.exe'
if (-not (Test-Path -LiteralPath $exe)) {
    throw "norka.exe not found at $exe"
}

$signtool = Get-ChildItem -Path 'C:\Program Files (x86)\Windows Kits\10\bin\*\x64\signtool.exe' -ErrorAction SilentlyContinue |
    Sort-Object FullName -Descending |
    Select-Object -First 1
if (-not $signtool) {
    throw 'signtool.exe not found in Windows Kits'
}

$pfx = Join-Path $env:RUNNER_TEMP 'norka-codesign.pfx'
try {
    [IO.File]::WriteAllBytes($pfx, [Convert]::FromBase64String($pfxB64.Trim()))
    $args = @(
        'sign',
        '/fd', 'SHA256',
        '/td', 'SHA256',
        '/tr', 'http://timestamp.digicert.com',
        '/f', $pfx
    )
    if (-not [string]::IsNullOrEmpty($env:WINDOWS_PFX_PASSWORD)) {
        $args += @('/p', $env:WINDOWS_PFX_PASSWORD)
    }
    $args += $exe
    & $signtool.FullName @args
    if ($LASTEXITCODE -ne 0) {
        throw "signtool exited with $LASTEXITCODE"
    }
    Write-Host 'PFX signature applied to norka.exe'
}
finally {
    if (Test-Path -LiteralPath $pfx) {
        Remove-Item -LiteralPath $pfx -Force
    }
}

$ErrorActionPreference = "Stop"
Set-Location $PSScriptRoot
& (Join-Path $PSScriptRoot "wenda-tong.exe") @args
if ($LASTEXITCODE -ne 0) { exit $LASTEXITCODE }

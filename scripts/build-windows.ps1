param(
    [string]$Output = "Volvid.exe"
)

$ErrorActionPreference = "Stop"
$Root = Split-Path -Parent $PSScriptRoot
Set-Location $Root

cargo build --release --locked -p volvid-gui
Copy-Item "target\release\Volvid.exe" $Output -Force
Write-Host "Built $Output"

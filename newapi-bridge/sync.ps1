# Copies the canonical nambridge sources into the new-api checkout.
$ErrorActionPreference = 'Stop'
$src = Join-Path $PSScriptRoot 'nambridge'
$dst = Join-Path $PSScriptRoot '..\temp\clansty-new-api\nambridge'
New-Item -ItemType Directory -Force $dst | Out-Null
Get-ChildItem $dst -Filter *.go -ErrorAction SilentlyContinue | Remove-Item -Force
Copy-Item (Join-Path $src '*.go') $dst -Force
Write-Host "synced $((Get-ChildItem $dst -Filter *.go).Count) files -> $dst"

# Builds the Svelte UI, embeds it, and produces sonar-remote.exe
$ErrorActionPreference = "Stop"
$root = $PSScriptRoot

Write-Host "==> Building web UI (Vite/Svelte)..." -ForegroundColor Cyan
Push-Location (Join-Path $root "web")
if (-not (Test-Path "node_modules")) { bun install }
bun run build
Pop-Location

Write-Host "==> Building Go binary..." -ForegroundColor Cyan
Push-Location $root
# -ldflags "-H windowsgui" would hide the console; keep it visible for logs.
$env:CGO_ENABLED = "0"
go build -ldflags "-s -w" -o sonar-remote.exe .
Pop-Location

Write-Host "==> Done: $(Join-Path $root 'sonar-remote.exe')" -ForegroundColor Green

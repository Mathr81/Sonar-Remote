# Builds the web remote, embeds it, and produces sonar-remote.exe.
$ErrorActionPreference = "Stop"

Push-Location (Join-Path $PSScriptRoot "web")
try {
    bun install --frozen-lockfile
    bun run build
} finally { Pop-Location }

Push-Location $PSScriptRoot
try {
    $env:CGO_ENABLED = "0"
    # -H=windowsgui: no console window. Drop it (or use `go run .`) to see logs.
    go build -ldflags "-s -w -H=windowsgui" -o sonar-remote.exe .
} finally { Pop-Location }

Write-Host "Built $(Join-Path $PSScriptRoot 'sonar-remote.exe')" -ForegroundColor Green

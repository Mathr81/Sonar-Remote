# Registers sonar-remote.exe to launch at Windows logon (per-user, no admin).
# Runs hidden in the background. Re-run to update. Use -Remove to uninstall.
param(
    [int]$Port = 80,
    [string]$HostName = "sonar",
    [switch]$Remove
)
$ErrorActionPreference = "Stop"

$exe = Join-Path $PSScriptRoot "sonar-remote.exe"
$taskName = "SonarRemote"

if ($Remove) {
    Unregister-ScheduledTask -TaskName $taskName -Confirm:$false -ErrorAction SilentlyContinue
    Write-Host "Removed scheduled task '$taskName'." -ForegroundColor Yellow
    return
}

if (-not (Test-Path $exe)) { throw "sonar-remote.exe not found. Run build.ps1 first." }

$action  = New-ScheduledTaskAction -Execute $exe -Argument "--port $Port --host $HostName" -WorkingDirectory $PSScriptRoot
$trigger = New-ScheduledTaskTrigger -AtLogOn
$settings = New-ScheduledTaskSettingsSet -AllowStartIfOnBatteries -DontStopIfGoingOnBatteries -StartWhenAvailable -ExecutionTimeLimit ([TimeSpan]::Zero) -Hidden
$principal = New-ScheduledTaskPrincipal -UserId $env:USERNAME -LogonType Interactive -RunLevel Limited

Register-ScheduledTask -TaskName $taskName -Action $action -Trigger $trigger -Settings $settings -Principal $principal -Force | Out-Null
Start-ScheduledTask -TaskName $taskName
Write-Host "Installed '$taskName' -> http://$HostName.local:$Port (starts at logon, running now)." -ForegroundColor Green

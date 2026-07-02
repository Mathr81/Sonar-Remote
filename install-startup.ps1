# Registers sonar-remote.exe to launch at Windows logon (per-user, no admin).
# Runs fully headless (no console window) and waits for the LAN to be up
# (an IPv4 on $Subnet) so mDNS can advertise correctly. Re-run to update.
# Use -Remove to uninstall.
param(
    [int]$Port = 80,
    [string]$HostName = "sonar",
    [string]$Subnet = "192.168.1.",   # wait until this PC gets an IPv4 here (mDNS needs the LAN up)
    [int]$WaitTimeoutSec = 120,       # give up waiting after this long and start anyway
    [switch]$Remove
)
$ErrorActionPreference = "Stop"

$exe      = Join-Path $PSScriptRoot "sonar-remote.exe"
$vbs      = Join-Path $PSScriptRoot "launcher.vbs"
$waitPs1  = Join-Path $PSScriptRoot "wait-and-run.ps1"
$taskName = "SonarRemote"

if ($Remove) {
    Unregister-ScheduledTask -TaskName $taskName -Confirm:$false -ErrorAction SilentlyContinue
    Remove-Item $vbs, $waitPs1 -ErrorAction SilentlyContinue
    Write-Host "Removed scheduled task '$taskName' and helper scripts." -ForegroundColor Yellow
    return
}

if (-not (Test-Path $exe)) { throw "sonar-remote.exe not found. Run build.ps1 first." }

# --- 1. wait-and-run.ps1 : hold until the LAN is up, then start the exe hidden ---
$waitBody = @'
param(
    [int]$Port = __PORT__,
    [string]$HostName = "__HOST__",
    [string]$Subnet = "__SUBNET__",
    [int]$TimeoutSec = __TIMEOUT__
)
$dir = $PSScriptRoot
$exe = Join-Path $dir "sonar-remote.exe"

# mDNS can only advertise once an interface holds a LAN IPv4. Poll until it does.
$deadline = (Get-Date).AddSeconds($TimeoutSec)
while ((Get-Date) -lt $deadline) {
    $hasIp = Get-NetIPAddress -AddressFamily IPv4 -ErrorAction SilentlyContinue |
             Where-Object { $_.IPAddress -like "$Subnet*" }
    if ($hasIp) { break }
    Start-Sleep -Seconds 3
}

Start-Process -FilePath $exe `
    -ArgumentList "--port $Port --host $HostName" `
    -WorkingDirectory $dir -WindowStyle Hidden
'@
$waitBody = $waitBody.
    Replace('__PORT__',   [string]$Port).
    Replace('__HOST__',   $HostName).
    Replace('__SUBNET__', $Subnet).
    Replace('__TIMEOUT__',[string]$WaitTimeoutSec)
Set-Content -Path $waitPs1 -Value $waitBody -Encoding UTF8

# --- 2. launcher.vbs : run wait-and-run.ps1 with a truly invisible window (style 0) ---
$vbsBody = @'
' Headless launcher for Sonar Remote. WScript window is invisible; the "0"
' argument to Run keeps PowerShell (and the exe it spawns) with no window.
Set fso = CreateObject("Scripting.FileSystemObject")
scriptDir = fso.GetParentFolderName(WScript.ScriptFullName)
Set sh = CreateObject("WScript.Shell")
sh.Run "powershell -NoProfile -ExecutionPolicy Bypass -File """ & scriptDir & "\wait-and-run.ps1""", 0, False
'@
Set-Content -Path $vbs -Value $vbsBody -Encoding ASCII

# --- 3. scheduled task launches the vbs via wscript (no window at all) ---
$action  = New-ScheduledTaskAction -Execute "wscript.exe" -Argument "`"$vbs`"" -WorkingDirectory $PSScriptRoot
$trigger = New-ScheduledTaskTrigger -AtLogOn
$settings = New-ScheduledTaskSettingsSet -AllowStartIfOnBatteries -DontStopIfGoingOnBatteries -StartWhenAvailable -ExecutionTimeLimit ([TimeSpan]::Zero) -Hidden
$principal = New-ScheduledTaskPrincipal -UserId $env:USERNAME -LogonType Interactive -RunLevel Limited

Register-ScheduledTask -TaskName $taskName -Action $action -Trigger $trigger -Settings $settings -Principal $principal -Force | Out-Null
Start-ScheduledTask -TaskName $taskName
Write-Host "Installed '$taskName' -> http://$HostName.local:$Port" -ForegroundColor Green
Write-Host "  headless (no console), waits for an IPv4 on $Subnet* before starting (up to ${WaitTimeoutSec}s)." -ForegroundColor Green
Write-Host "  Stop a running instance with: Stop-Process -Name sonar-remote" -ForegroundColor DarkGray

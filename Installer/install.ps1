# SentinelEdge Agent - Installer
# Run as Administrator
$InstallDir = "C:\Program Files\SentinelEdge"
$ServerURL  = "https://saapi.ardepa.site"
$TenantID   = "tenant-sentineledge"
$VaultURL   = "https://pwd.ardepa.site"
$VaultClientID     = "user.f50ad073-3d5a-4bdd-8ce7-a4fed752c1e8"
$VaultClientSecret = "EKQOdtvqccwi2qasiIIjreKY20XOcZ"

Write-Host ""
Write-Host "======================================" -ForegroundColor Cyan
Write-Host "  SentinelEdge Agent - Installer      " -ForegroundColor Cyan
Write-Host "======================================" -ForegroundColor Cyan
Write-Host ""

if (-not ([Security.Principal.WindowsPrincipal] [Security.Principal.WindowsIdentity]::GetCurrent()).IsInRole([Security.Principal.WindowsBuiltInRole]::Administrator)) {
    Write-Host "ERROR: This script must be run as Administrator" -ForegroundColor Red
    Write-Host "       Right-click the script and select 'Run as administrator'" -ForegroundColor Yellow
    pause
    exit 1
}

$svcExists = Get-Service SentinelEdgeAgent -ErrorAction SilentlyContinue
if ($svcExists) {
    Write-Host "Previous installation detected, removing..." -ForegroundColor Yellow
    Stop-Service SentinelEdgeAgent -Force -ErrorAction SilentlyContinue
    & "$InstallDir\sentineledge-agent.exe" uninstall
    Start-Sleep -Seconds 2
}

Write-Host "Creating installation directory..." -ForegroundColor White
New-Item -ItemType Directory -Force -Path $InstallDir | Out-Null

Write-Host "Copying files..." -ForegroundColor White
Copy-Item -Path "$PSScriptRoot\sentineledge-agent.exe" -Destination "$InstallDir\sentineledge-agent.exe" -Force

Write-Host "Writing configuration..." -ForegroundColor White
@"
ServerURL: "$ServerURL"
TenantID: "$TenantID"
VaultURL: "$VaultURL"
VaultClientID: "$VaultClientID"
VaultClientSecret: "$VaultClientSecret"
PollInterval: 30
"@ | Out-File -FilePath "$InstallDir\agent.yaml" -Encoding UTF8 -Force

Write-Host "Installing Windows Service..." -ForegroundColor White
Set-Location $InstallDir
& ".\sentineledge-agent.exe" install
Start-Sleep -Seconds 1
& ".\sentineledge-agent.exe" start
Start-Sleep -Seconds 2

$svc = Get-Service SentinelEdgeAgent -ErrorAction SilentlyContinue
Write-Host ""
if ($svc -and $svc.Status -eq "Running") {
    Write-Host "Installation completed successfully" -ForegroundColor Green
    Write-Host "   Computer : $env:COMPUTERNAME" -ForegroundColor Green
    Write-Host "   Server   : $ServerURL" -ForegroundColor Green
    Write-Host "   Status   : Running" -ForegroundColor Green
} else {
    Write-Host "ERROR: Service did not start correctly" -ForegroundColor Red
    Write-Host "       Check files in: C:\Program Files\SentinelEdge\" -ForegroundColor Yellow
}
Write-Host ""
pause
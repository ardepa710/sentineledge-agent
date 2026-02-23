# SentinelEdge Agent - Installer
# Run as Administrator

$InstallDir = "C:\Program Files\SentinelEdge"
$ServerURL  = "https://saapi.ardepa.site"
$TenantID   = "tenant-sentineledge"
$APIKey     = "2187dc3fa065f7e9c5f7818028608ef44a6ed098c0c75141c0c3a5281f5df245"

Write-Host ""
Write-Host "======================================" -ForegroundColor Cyan
Write-Host "  SentinelEdge Agent - Installer      " -ForegroundColor Cyan
Write-Host "======================================" -ForegroundColor Cyan
Write-Host ""

# Check for Administrator privileges
if (-not ([Security.Principal.WindowsPrincipal] [Security.Principal.WindowsIdentity]::GetCurrent()).IsInRole([Security.Principal.WindowsBuiltInRole]::Administrator)) {
    Write-Host "ERROR: This script must be run as Administrator" -ForegroundColor Red
    Write-Host "       Right-click the script and select 'Run as administrator'" -ForegroundColor Yellow
    pause
    exit 1
}

# Uninstall previous version if exists
$svcExists = Get-Service SentinelEdgeAgent -ErrorAction SilentlyContinue
if ($svcExists) {
    Write-Host "Previous installation detected, removing..." -ForegroundColor Yellow
    Stop-Service SentinelEdgeAgent -Force -ErrorAction SilentlyContinue
    & "$InstallDir\sentineledge-agent.exe" uninstall
    Start-Sleep -Seconds 2
}

# Create install directory
Write-Host "Creating installation directory..." -ForegroundColor White
New-Item -ItemType Directory -Force -Path $InstallDir | Out-Null

# Copy executable
Write-Host "Copying files..." -ForegroundColor White
Copy-Item -Path "$PSScriptRoot\sentineledge-agent.exe" -Destination "$InstallDir\sentineledge-agent.exe" -Force

# Write config file
Write-Host "Writing configuration..." -ForegroundColor White
@"
ServerURL: "$ServerURL"
TenantID: "$TenantID"
APIKey: "$APIKey"
PollInterval: 30
"@ | Out-File -FilePath "$InstallDir\agent.yaml" -Encoding UTF8 -Force

# Install and start Windows Service
Write-Host "Installing Windows Service..." -ForegroundColor White
Set-Location $InstallDir
& ".\sentineledge-agent.exe" install
Start-Sleep -Seconds 1
& ".\sentineledge-agent.exe" start
Start-Sleep -Seconds 2

# Verify result
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
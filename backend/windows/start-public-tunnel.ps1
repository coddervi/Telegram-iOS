# Publish the local API through a temporary Cloudflare Quick Tunnel.
# The URL is temporary and changes when this process is restarted.
$ErrorActionPreference = "Stop"
$BackendDir = Split-Path -Parent $PSScriptRoot
Set-Location $BackendDir

if (-not (Get-Command cloudflared -ErrorAction SilentlyContinue)) {
    Write-Host "cloudflared is not installed." -ForegroundColor Yellow
    Write-Host "Install it with: winget install --id Cloudflare.cloudflared -e"
    throw "Install cloudflared, open a new PowerShell window, then rerun this script."
}
try {
    Invoke-RestMethod -Uri "http://127.0.0.1:8080/health" -TimeoutSec 3 | Out-Null
} catch {
    throw "The API is not responding. Run .\windows\start-server.ps1 first."
}

Write-Host "Starting public tunnel. Keep this window open." -ForegroundColor Cyan
Write-Host "Copy the https://....trycloudflare.com URL printed below into the iOS app configuration."
cloudflared tunnel --url http://127.0.0.1:8080

# Stop the server containers without deleting database or media volumes.
$ErrorActionPreference = "Stop"
$BackendDir = Split-Path -Parent $PSScriptRoot
Set-Location $BackendDir
docker compose down
if ($LASTEXITCODE -ne 0) { throw "Docker Compose could not stop the services." }
Write-Host "Server stopped. Database and uploaded media were preserved." -ForegroundColor Green

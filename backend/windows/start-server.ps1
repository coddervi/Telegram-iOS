# Start the private-platform API and database on Windows.
$ErrorActionPreference = "Stop"
$BackendDir = Split-Path -Parent $PSScriptRoot
Set-Location $BackendDir

if (-not (Get-Command docker -ErrorAction SilentlyContinue)) {
    throw "Docker is not installed. Install Docker Desktop for Windows, then run this script again."
}
try { docker info *> $null } catch { throw "Docker Desktop is not running. Start Docker Desktop and retry." }
if ($LASTEXITCODE -ne 0) { throw "Docker Desktop is not ready. Wait for it to finish starting, then retry." }

if (-not (Test-Path ".env")) {
    $template = Get-Content ".env.example" -Raw
    $bytes = New-Object byte[] 48
    $rng = [System.Security.Cryptography.RandomNumberGenerator]::Create()
    $rng.GetBytes($bytes)
    $rng.Dispose()
    $password = [Convert]::ToBase64String($bytes).TrimEnd("=").Replace("+", "-").Replace("/", "_")
    $template = $template -replace '(?m)^POSTGRES_PASSWORD=.*$', "POSTGRES_PASSWORD=$password"
    [System.IO.File]::WriteAllText((Join-Path $BackendDir ".env"), $template, (New-Object System.Text.UTF8Encoding($false)))
    Write-Host "Created backend/.env with a random database password." -ForegroundColor Green
}

docker compose up --build -d
if ($LASTEXITCODE -ne 0) { throw "Docker Compose failed. Check the output above." }

Write-Host "Waiting for API health check..." -ForegroundColor Cyan
$healthy = $false
for ($i = 0; $i -lt 30; $i++) {
    try {
        $response = Invoke-RestMethod -Uri "http://127.0.0.1:8080/health" -TimeoutSec 3
        $healthy = $true
        break
    } catch { Start-Sleep -Seconds 2 }
}
if (-not $healthy) {
    docker compose logs --tail 80 api
    throw "API did not become healthy. See logs above."
}
Write-Host "Private Platform API is running: http://127.0.0.1:8080" -ForegroundColor Green
Write-Host "Health endpoint: http://127.0.0.1:8080/health"
Write-Host "Next: run .\windows\start-public-tunnel.ps1 to create a temporary public URL."

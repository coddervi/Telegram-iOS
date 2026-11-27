# Run the Private Platform server on Windows

This folder runs the Go API and PostgreSQL on a Windows PC using Docker Desktop.
The API listens on loopback only (127.0.0.1:8080); do not open router ports.

## 1. Requirements

- Windows 10/11, 64-bit
- Docker Desktop for Windows, started with Linux containers
- PowerShell 5.1 or newer
- Internet connection for downloading container images

Install Docker Desktop from https://www.docker.com/products/docker-desktop/ and reboot if its installer requests it.

## 2. Start the server

Open PowerShell in the repository's `backend` folder and run:

```powershell
Set-ExecutionPolicy -Scope Process Bypass
.\windows\start-server.ps1
```

The script creates `.env` with a random database password on first launch, builds the API image, starts PostgreSQL/Redis/API and waits for `/health`.

Local health check: http://127.0.0.1:8080/health

## 3. Make it reachable outside home

Install the Cloudflare tunnel client:

```powershell
winget install --id Cloudflare.cloudflared -e
```

Open a second PowerShell window in `backend` and run:

```powershell
.\windows\start-public-tunnel.ps1
```

Cloudflare Quick Tunnel prints a public HTTPS address ending in `.trycloudflare.com`. Keep that PowerShell window open. The address is temporary and changes after a restart. No router port forwarding is required; the API port remains bound to 127.0.0.1.

Use the printed HTTPS base URL in the iOS client's server configuration. Do not publish the `.env` file or share its database password.

## 4. Stop the server

Run ` .\windows\stop-server.ps1 ` from the backend folder. This stops containers but preserves PostgreSQL data and media. To stop public access, also close the tunnel PowerShell window.

## 5. Important notes

- The PC must remain powered on, connected to the internet, and awake. Disable Windows sleep while hosting.
- Docker Desktop must be running after a Windows restart; then run `start-server.ps1` again.
- Quick Tunnel is intended for testing, not a permanent production hostname. For a stable address, configure a named Cloudflare Tunnel and a domain later.
- Current backend scope is documented in `README.md`. The API foundation currently covers account/session, basic conversations/messages and internal wallet/gift endpoints. Bot-based number verification, real-time delivery, push notifications and end-to-end encryption are not yet complete.
- Never expose PostgreSQL or Redis ports publicly. Do not add router port-forwarding rules for this setup.

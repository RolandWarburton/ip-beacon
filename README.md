<p align="center">
  <img src="assets/logo.png" width="200" alt="beacon logo">
</p>

# beacon

> [!NOTE]
> This is a 5min side quest vibe coded app. Use at your own risk!

A lightweight server to work around some corporate networks that block avahi via mDNS/DNS-SD.

A client may POST its local IP to a known server,
client B on the same network retrieves the IP via the server instead of conventional DNS.

## Usage

### Server

```bash
go run .
```

| Variable    | Default                  | Description                  |
|-------------|--------------------------|------------------------------|
| `PORT`      | `8080`                   | Port to listen on            |
| `DATA_PATH` | `data/registry.json`     | Path to persistence file     |

Or with Docker Compose:

```bash
docker compose up --build
```

### Client

## Files

| File | Purpose |
|---|---|
| `register.sh` | Posts hostname and IP to the registry |
| `ip-register.service` | Systemd service that runs `register.sh` |
| `ip-register.timer` | Systemd timer that triggers the service every 5 minutes |
| `install.sh` | Installs and enables the service and timer |


> [!NOTE]
> Edit ip-registry.service to set REGISTRY_HOST, then:

To install as a systemd service:

```bash
systemctl enable ip-registry.service
```

Or run `register.sh` once to post manually.

## API

| Method | Path        | Description                              |
|--------|-------------|------------------------------------------|
| `POST` | `/register` | Register or update a host: `{"host": "...", "ip": "..."}` |
| `GET`  | `/hosts`    | List all hosts as JSON                   |
| `GET`  | `/`         | Web UI                                   |

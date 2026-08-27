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
cp .env.example .env   # then edit DOMAIN
docker compose up --build
```

### Client

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

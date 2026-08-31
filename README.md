<p align="center">
  <img src="assets/logo.png" width="200" alt="beacon logo">
</p>

# beacon

> [!NOTE]
> This is a 5min side quest vibe coded app. Use at your own risk!

A lightweight server to work around some corporate networks that block avahi via mDNS/DNS-SD.

A client may POST its local IP to a known server,
client B on the same network retrieves the IP via the server instead of conventional DNS.

## Layout

```
main.go       entrypoint and HTTP server
registry.go   host storage, persisted as JSON
web.go        routes and the embedded client
client/       register.sh and the systemd units, served to clients
```

## Usage

### Server

```bash
BASE_URL=http://localhost:8080 go run .
```

| Variable    | Default              | Description                                  |
|-------------|----------------------|----------------------------------------------|
| `BASE_URL`  | *required*           | Public address clients reach this server on  |
| `PORT`      | `8080`               | Port to listen on                            |
| `DATA_PATH` | `data/registry.json` | Path to persistence file                     |

`BASE_URL` is what gets substituted into the client files the server hands out,
so it must be the address clients can actually reach — not the container's
internal one.

Or with Docker Compose:

```bash
cp .env.example .env   # then edit DOMAIN
docker compose up --build
```

### Client

The server serves its own client installer, so nothing needs to be cloned and no
host needs to be typed in. Every file it hands out has `BASE_URL` substituted in
first, so the installer copies the units into place verbatim:

```bash
curl -fsSL https://beacon.example.com/client/install.sh | sudo sh
```

It downloads `register.sh` and the systemd units from the same server, installs
`/usr/local/bin/beacon-register` plus `beacon.service` / `beacon.timer`, enables
the timer to report every 5 minutes, and reports once immediately.

To point a client at a different registry, set `REGISTRY_URL` in
`/etc/systemd/system/beacon.service`, or run
`REGISTRY_URL=https://beacon.example.com beacon-register` to post manually.

## API

| Method | Path                 | Description                                               |
|--------|----------------------|-----------------------------------------------------------|
| `POST` | `/register`          | Register or update a host: `{"host": "...", "ip": "..."}` |
| `GET`  | `/hosts`             | List all hosts as JSON, most recent first                 |
| `GET`  | `/client/{file}`     | Client installer and components, with this server's address filled in |
| `GET`  | `/`                  | Host list as plain text                                   |

To remove a host, delete it from `registry.json` and restart.

## Development

```bash
go test ./...
```

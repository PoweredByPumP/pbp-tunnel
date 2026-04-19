# pbp-tunnel

**pbp-tunnel** is a reverse SSH port-forwarding tool that lets you expose a local service on a remote host through SSH,
without opening inbound ports on the client side. It supports password and key-based authentication, dynamic port
assignment, IP whitelisting, and automatic cleanup on client disconnection.

---

## Features

* 🔒 **Secure by default**: SSH authentication via password or public key
* 🔁 **Reverse tunneling**: Expose local services through the server
* 🧠 **Automatic reconnection**: Client auto-retries on disconnection (//FIXME)
* 🔄 **Dynamic port assignment**: Server allocates or respects requested ports
* 🔀 **Protocol selection**: Choose `tcp` (default) or `udp` forwarding per client
* 🚧 **IP whitelisting**: Restrict which peers can connect to forwarded ports
* 🔌 **Multi-connection support**: Each incoming connection uses its own SSH channel
* 🛠️ **Configurable**: JSON config file (with `generate` mode), flags or environment variables
* 🎨 **User-friendly CLI**: Clear help, colored output

---

## Table of Contents

1. [Installation](#installation)
2. [Quickstart](#quickstart)
    - [Server](#server)
    - [Client](#client)
3. [Configuration](#configuration)
    - [Config File](#json-config-file)
    - [Environment Variables](#environment-variables)
4. [Help & Usage](#help--usage)
5. [Testing](#testing)
6. [Project Structure](#project-structure)
7. [Architecture Overview](#architecture-overview)
8. [Security Notes](#security-notes)
9. [License](#license)

## Installation

Grab the latest binary for your platform from the [Releases](https://github.com/PoweredByPumP/pbp-tunnel/releases) page,
or build from source:

```bash
git clone https://github.com/PoweredByPumP/pbp-tunnel.git
cd pbp-tunnel
go build -o out/pbp-tunnel ./cmd/pbp-tunnel
```

You can also use the provided Dockerfile to build a container image:

```bash
make build
```

We are also available on [Scoop](https://github.com/PoweredByPumP/scoop) if you're on Windows:

```bash
scoop bucket add pbp-scoop https://github.com/PoweredByPumP/scoop
scoop install pbp-scoop/pbp-tunnel
```

---

## Quickstart

### Server

Start the server (reads `config.json` by default):

```bash
./pbp-tunnel server
```

With flags:

```bash
./pbp-tunnel server \
  --bind 0.0.0.0 \
  --port 52135 \
  --port-range-start 49152 \
  --port-range-end 65535 \
  --username myuser \
  --password mypass \
  --private-rsa-path ./id_rsa \
  --allowed-ips 198.51.100.5,203.0.113.10
```

### Client

Launch the client (reads `config.json` by default):

```bash
./pbp-tunnel client
```

With flags:

```bash
./pbp-tunnel client \
  --endpoint myserver.com \
  --port 52135 \
  --username myuser \
  --password mypass \
  --local-host localhost \
  --local-port 8080 \
  --remote-host localhost \
  --remote-port 0 \
  --protocol tcp \
  --host-key-level 2 \
  --host-key ./host_key.pub
```

---

## Configuration

### JSON Config File

Create a `config.json` alongside the binary:

```json lines
// server mode
{
  "type": "server",
  "server": {
    "bind": "0.0.0.0",
    "port": 52135,
    "port_range_start": 49152,
    "port_range_end": 65535,
    "username": "myuser",
    "password": "mypass",
    "private_rsa_path": "./id_rsa",
    "allowed_ips": [
      "198.51.100.5",
      "203.0.113.10"
    ]
  }
}
```

```json lines
// client mode
{
  "type": "client",
  "client": {
    "host_key_level": 2,
    "endpoint": "myserver.com",
    "port": 52135,
    "username": "myuser",
    "password": "mypass",
    "local_host": "localhost",
    "local_port": 8080,
    "remote_host": "localhost",
    "remote_port": 0,
    "protocol": "tcp"
  }
}
```

Generate an interactive template with:

```bash
./pbp-tunnel generate
```

### Environment Variables

All settings can be overridden via environment variables prefixed `PBP_TUNNEL_`. For example:

| Variable                          | Description                                |
|-----------------------------------|--------------------------------------------|
| `PBP_TUNNEL_TYPE`                 | "client" or "server"                       |
| `PBP_TUNNEL_ENDPOINT`             | Server address (client mode)               |
| `PBP_TUNNEL_PORT`                 | Server port                                |
| `PBP_TUNNEL_USERNAME`             | SSH username                               |
| `PBP_TUNNEL_PASSWORD`             | SSH password                               |
| `PBP_TUNNEL_LOCAL_HOST`           | Local service address (client mode)        |
| `PBP_TUNNEL_LOCAL_PORT`           | Local service port (client mode)           |
| `PBP_TUNNEL_REMOTE_HOST`          | Remote host to expose (client mode)        |
| `PBP_TUNNEL_REMOTE_PORT`          | Remote port to request (0 for dynamic)     |
| `PBP_TUNNEL_PROTOCOL`             | Forward protocol: `tcp` (default) or `udp` |
| `PBP_TUNNEL_BIND`                 | Server bind address                        |
| `PBP_TUNNEL_BIND_PORT`            | Server listen port                         |
| `PBP_TUNNEL_PORT_RANGE_START`     | Start of server port range                 |
| `PBP_TUNNEL_PORT_RANGE_END`       | End of server port range                   |
| `PBP_TUNNEL_PRIVATE_RSA_PATH`     | Server private RSA key path                |
| `PBP_TUNNEL_PRIVATE_ECDSA_PATH`   | Server private ECDSA key path              |
| `PBP_TUNNEL_PRIVATE_ED25519_PATH` | Server private ED25519 key path            |
| `PBP_TUNNEL_ALLOWED_IPS`          | Comma-separated list of allowed client IPs |

---

## Help & Usage

Show top-level help:

```bash
./pbp-tunnel --help
```

Show client-mode flags:

```bash
./pbp-tunnel client --help
```

Show server-mode flags:

```bash
./pbp-tunnel server --help
```

---

## Testing

Run all unit tests:

```bash
go test ./... -v
```

Or just server package:

```bash
go test ./internal/server -v
```

---

## Project Structure

```text
.
├── cmd/pbp-tunnel/main.go
├── config.json.sample
├── Dockerfile
├── go.mod
├── go.sum
├── internal
│   ├── client
│   │   ├── client.go
│   │   └── client_test.go
│   ├── config
│   │   ├── constants.go
│   │   ├── constants_test.go
│   │   ├── loader.go
│   │   ├── loader_test.go
│   │   ├── provider.go
│   │   ├── provider_test.go
│   │   ├── template.go
│   │   ├── templates/config.json.tmpl
│   │   └── template_test.go
│   ├── server
│   │   ├── server.go
│   │   └── server_test.go
│   └── util
│       └── helper.go
├── Jenkinsfile
├── Makefile
├── out/pbp-tunnel
└── README.md
```

---

## Architecture Overview

```
[ Local Service ] ←─── SSH Tunnel ───→ [ pbp-tunnel Client ]
                                     │
                          Reverse port request (host:port)
                                     │
                              [ pbp-tunnel Server ]
                                     │
                              [ Exposed Public Port ]
```

1. **Client** connects to **Server** via SSH.
2. Client sends a port-forward request.
3. **Server** allocates or validates the port in its allowed range.
4. Incoming connections on that port are tunneled back to the **Client**, which forwards them to the **Local Service**.
5. On client disconnect, the server cleans up and frees the port.

---

## Security Notes

* **Host-key levels** range 0 (none) to 2 (strict).
* **IP whitelisting** protects forwarded ports from unwanted peers.
* **Automatic cleanup** prevents stale port reservations.
* **Key permissions**: private keys should be `0600`.

---

## License

Licensed under **MIT**. Feel free to use, modify, and distribute.

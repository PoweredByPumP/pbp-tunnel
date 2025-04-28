# pbp-tunnel

**pbp-tunnel** is a simple SSH-based reverse port forwarding system. It securely exposes a local service to a remote
server without the client opening any inbound ports. It uses SSH with optional password- or key-based authentication and
dynamically handles forwarded ports.

---

## Features

- 🔒 **Secure by design**: SSH authentication (password + public key)
- 🔁 **Reverse tunnel**: Expose local services through the server
- 🧠 **Automatic reconnection**: Client auto-retries on disconnection
- 🔢 **Dynamic port assignment**: Server allocates ports dynamically or respects requested ports
- ⚡ **Multi-connection support**: Each incoming connection uses its own SSH channel
- 📜 **Configurable**: JSON file or environment variables
- 🛠️ **Config generator**: Interactive template generator (`generate` mode)
- 🎨 **Nice CLI**: Colored output and clear help

---

## Table of Contents

1. [Installation](#installation)
2. [Quickstart](#quickstart)
    - [Server](#running-the-server)
    - [Client](#running-the-client)
3. [Configuration](#configuration)
    - [Config File](#config-file)
    - [Environment Variables](#environment-variables)
4. [Help & Usage](#help--usage)
5. [Architecture Overview](#architecture-overview)
6. [Project Structure](#project-structure)
7. [Security Notes](#security-notes)
8. [License](#license)

---

## Installation

```bash
git clone https://your-repository-url.git
cd pbp-tunnel
go build -o pbp-tunnel
```

---

## Quickstart

### Running the Server

Without flags (will read `config.json`):

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
  --password mypassword \
  --private-rsa ./dev/private_key.pem \
  --allowed-ips 192.0.2.10,198.51.100.5
```

### Running the Client

Without flags (will read `config.json`):

```bash
./pbp-tunnel client
```

With flags:

```bash
./pbp-tunnel client \
  --endpoint myserver.com \
  --port 52135 \
  --username myuser \
  --password mypassword \
  --local-host localhost \
  --local-port 8080 \
  --remote-host localhost \
  --remote-port 0 \
  --host-key-level 2 \
  --host-key ./dev/host_key
```

---

## Configuration

### Config File

Place a `config.json` file alongside the binary, for example:

```json
{
  "type": "server",
  "server": {
    "bind": "0.0.0.0",
    "port": 52135,
    "port_range_start": 49152,
    "port_range_end": 65535,
    "username": "myuser",
    "password": "mypassword",
    "private_rsa": "./dev/private_key.pem",
    "allowed_ips": [
      "192.0.2.10",
      "198.51.100.5"
    ]
  }
}
```

Or for a client:

```json
{
  "type": "client",
  "client": {
    "host_key_level": 2,
    "endpoint": "myserver.com",
    "port": 52135,
    "username": "myuser",
    "password": "mypassword",
    "local_host": "localhost",
    "local_port": 8080,
    "remote_host": "localhost",
    "remote_port": 0
  }
}
```

Generate a template interactively:

```bash
./pbp-tunnel generate
```

### Environment Variables

All config keys can be overridden via environment variables prefixed with `PBP_TUNNEL_`. For example:

| Variable                          | Description                           |
|-----------------------------------|---------------------------------------|
| `PBP_TUNNEL_TYPE`                 | `"client"` or `"server"`              |
| `PBP_TUNNEL_ENDPOINT`             | Server address (client mode)          |
| `PBP_TUNNEL_PORT`                 | Server port                           |
| `PBP_TUNNEL_USERNAME`             | SSH username                          |
| `PBP_TUNNEL_PASSWORD`             | SSH password                          |
| `PBP_TUNNEL_LOCAL_HOST`           | Local service address                 |
| `PBP_TUNNEL_LOCAL_PORT`           | Local service port                    |
| `PBP_TUNNEL_REMOTE_HOST`          | Remote host to expose (client mode)   |
| `PBP_TUNNEL_REMOTE_PORT`          | Remote port to request (0 for random) |
| `PBP_TUNNEL_BIND`                 | Server bind address                   |
| `PBP_TUNNEL_BIND_PORT`            | Server listen port                    |
| `PBP_TUNNEL_PORT_RANGE_START`     | Start of server port range            |
| `PBP_TUNNEL_PORT_RANGE_END`       | End of server port range              |
| `PBP_TUNNEL_PRIVATE_RSA_PATH`     | Server private RSA key path           |
| `PBP_TUNNEL_PRIVATE_ECDSA_PATH`   | Server private ECDSA key path         |
| `PBP_TUNNEL_PRIVATE_ED25519_PATH` | Server private ED25519 key path       |
| `PBP_TUNNEL_ALLOWED_IPS`          | Comma-separated list of allowed IPs   |

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

## Architecture Overview

```
[ Local Service ] ←─── SSH tunnel ───→ [ pbp-tunnel Client ]  
                                      │  
                                      │ Reverse port request  
                                      ↓  
                              [ pbp-tunnel Server ]  
                                      │  
                                      ↓  
                       [ Public Internet ←→ Exposed Port ]
```

1. **Client** connects to **Server** via SSH.
2. Client requests a remote port (or “0” for automatic assignment).
3. Server binds the requested port in its range and listens.
4. Incoming connections to that port are tunneled back to the local service.

---

## Project Structure

| File                   | Purpose                                                           |
|------------------------|-------------------------------------------------------------------|
| **main.go**            | Entry point: mode detection (`client`, `server`, `generate`).     |
| **client.go**          | Client implementation: SSH dial, port request, channel handling.  |
| **server.go**          | Server implementation: accept SSH, assign ports, forward traffic. |
| **config_provider.go** | Build `ssh.ClientConfig` & `ssh.ServerConfig` from parameters.    |
| **props.go**           | Load JSON/config + environment variables into `AppConfig`.        |
| **helper.go**          | Utilities: colored output, key generation, help message printing. |
| **template.go**        | Interactive config file template generator.                       |
| **constants.go**       | Shared constants & struct definitions (`ClientParameters`, etc.). |
| **config.json.tmpl**   | Embedded Go template for generating `config.json`.                |

---

## Security Notes

- **Host-key verification**: Default level is **strict** (`2`).
- **Authentication**: Supports both password and public-key methods.
- **IP whitelisting**: Restrict clients via `allowed_ips`.
- **Key protection**: Generated private keys use `0600` file permissions.

---

## License

This project is licensed under the **MIT License**. Feel free to use, modify, and distribute.

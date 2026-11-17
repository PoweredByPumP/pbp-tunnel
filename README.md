# pbp-tunnel

**pbp-tunnel** is a reverse SSH port-forwarding tool that lets you expose a local service on a remote host through SSH,
without opening inbound ports on the client side. It supports password and key-based authentication, dynamic port
assignment, IP allow lists, and automatic cleanup on client disconnection.

---

## Features

* **Security**: SSH authentication via password or public key
* **Reverse tunneling**: Expose local services through the server without inbound ports on the client
* **Automatic reconnection**: Client auto-retries on disconnection
* **Dynamic port assignment**: Server allocates or respects requested ports
* **Protocol selection**: Choose `tcp` (default) or `udp` forwarding per client
* **Allowed IPs**: Restrict which clients may open tunnels and which peers may use them
* **Multi-connection support**: Each incoming connection uses its own SSH channel
* **Configurable**: JSON config file (with `generate` mode), flags or environment variables
* **User-friendly CLI**: Clear help, colored output

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
  --remote-port 0 \
  --protocol tcp \
  --host-key-level 2 \
  --host-key ./known_hosts
```

By default the client only connects to a server listed in the `known_hosts` file
given by `--host-key` (see [Server identity](#server-identity)).

---

## Configuration

Settings come from four sources. Each one overrides the previous:

1. built-in defaults (see `pbp-tunnel client --help` and `pbp-tunnel server --help`)
2. the JSON config file
3. `PBP_TUNNEL_*` environment variables
4. command-line flags

### JSON Config File

Create a `config.json` in the working directory, or point `PBP_TUNNEL_CONFIG` to
another file:

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
    "host_key": "./known_hosts",
    "endpoint": "myserver.com",
    "port": 52135,
    "username": "myuser",
    "password": "mypass",
    "local_host": "localhost",
    "local_port": 8080,
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

Every flag has a matching variable: `PBP_TUNNEL_` followed by the flag name in
upper case with dashes turned into underscores (`--local-port` becomes
`PBP_TUNNEL_LOCAL_PORT`). Empty variables are ignored, and an invalid value stops
the program with an error.

| Variable                               | Description                                  |
|----------------------------------------|----------------------------------------------|
| `PBP_TUNNEL_TYPE`                      | "client" or "server", when run without a command |
| `PBP_TUNNEL_CONFIG`                    | Path of the JSON config file                 |
| `PBP_TUNNEL_ENDPOINT`                  | Server address (client mode)                 |
| `PBP_TUNNEL_PORT`                      | SSH port: listened on by the server, dialed by the client |
| `PBP_TUNNEL_USERNAME`                  | SSH username                                 |
| `PBP_TUNNEL_PASSWORD`                  | SSH password                                 |
| `PBP_TUNNEL_IDENTITY`                  | Private key path (client mode)               |
| `PBP_TUNNEL_HOST_KEY`                  | Known hosts file (client mode)               |
| `PBP_TUNNEL_HOST_KEY_LEVEL`            | Host key check level (client mode)           |
| `PBP_TUNNEL_LOCAL_HOST`                | Local service address (client mode)          |
| `PBP_TUNNEL_LOCAL_PORT`                | Local service port (client mode)             |
| `PBP_TUNNEL_REMOTE_PORT`               | Remote port to request (0 for dynamic)       |
| `PBP_TUNNEL_PROTOCOL`                  | Forward protocol: `tcp` (default) or `udp`   |
| `PBP_TUNNEL_BIND`                      | Server bind address                          |
| `PBP_TUNNEL_PORT_RANGE_START`          | Start of server port range                   |
| `PBP_TUNNEL_PORT_RANGE_END`            | End of server port range                     |
| `PBP_TUNNEL_PRIVATE_RSA_PATH`          | Server private RSA key path                  |
| `PBP_TUNNEL_PRIVATE_ECDSA_PATH`        | Server private ECDSA key path                |
| `PBP_TUNNEL_PRIVATE_ED25519_PATH`      | Server private ED25519 key path              |
| `PBP_TUNNEL_AUTHORIZED_KEYS_PATH`      | Server authorized_keys path                  |
| `PBP_TUNNEL_ALLOWED_IPS`               | Comma-separated list of allowed IPs          |

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

Global flags go before the command:

```bash
./pbp-tunnel --logging both --logfile /var/log/pbp-tunnel/pbp-tunnel.log server
```

| Flag        | Description                                                              |
|-------------|--------------------------------------------------------------------------|
| `--logging` | `console` (default), `file` or `both`                                    |
| `--logfile` | Log file for the `file` and `both` modes; default `/var/log/pbp-tunnel/pbp-tunnel.log`. A directory gets a `pbp-tunnel.log` inside. |
| `--debug`   | Log the number of goroutines and the memory used every 10 seconds        |
| `--version` | Print the version and exit                                               |

---

## Testing

Run every test, including the end-to-end tests that start a real server and client
on the loopback interface:

```bash
go test ./...
```

Check for data races (needs cgo and a C compiler):

```bash
go test -race ./...
```

Run only the end-to-end tests:

```bash
go test ./e2e
```

---

## Project Structure

```text
cmd/pbp-tunnel/      command line: global flags, commands, logging, help
internal/config/     parameters: defaults, config file, environment, flags, generate
internal/client/     reconnection loop, tunnel session, TCP and UDP relays
internal/server/     SSH server, port pool, tunnel requests, TCP and UDP relays
internal/protocol/   the tunnel request exchanged by the client and the server
internal/allowlist/  lists of allowed IP addresses and CIDR ranges
internal/hostkey/    generation of missing server host keys
internal/relay/      two-way copy between a connection and an SSH channel
e2e/                 end-to-end tests
```

---

## Architecture Overview

```
[ Remote peer ] ──→ [ Exposed port ]  pbp-tunnel server
                          ▲
                          │  SSH connection, opened by the client
                          ▼
                    pbp-tunnel client ──→ [ Local service ]
```

1. The **client** opens an SSH connection to the **server** and sends a tunnel request
   on a first channel: the IPs allowed to use the port, `tcp` or `udp`, and the port it
   wants (0 lets the server choose). The exchange is described in
   [`internal/protocol/handshake.go`](internal/protocol/handshake.go).
2. The **server** reserves the port in its range and listens on it.
3. Each connection on that port, or each UDP peer, gets its own SSH channel to the
   client, which relays it to the **local service**.
4. When the SSH connection ends, the server frees the port and the client reconnects.

---

## Security Notes

### Server identity

The client checks the server host key against an OpenSSH `known_hosts` file, set
with `--host-key` (`host_key` in the config file). `--host-key-level` decides what
happens when the key is not in that file:

| Level         | Behavior                                                            |
|---------------|---------------------------------------------------------------------|
| `2` (default) | Refuse the server. Without a `known_hosts` file the client does not start. |
| `1`           | Log a warning and connect anyway.                                   |
| `0`           | Connect without checking.                                           |

To build the `known_hosts` file, read the public key of the server host key on the
server, then copy the file to the client:

```bash
echo "[myserver.com]:52135 $(ssh-keygen -y -f id_rsa)" > known_hosts
```

A public key file such as `host_key.pub` is not a `known_hosts` file: it lacks the
server address at the start of the line.

### Other notes

* **Allowed IPs**: on the server, `allowed_ips` lists the clients that may open a
  tunnel; on the client, it lists the peers that may use the exposed port. An empty
  list allows everyone, and an invalid entry stops the program at startup.
* **Automatic cleanup** prevents stale port reservations.
* **Key permissions**: private keys should be `0600`.

---

## License

Licensed under **MIT**. Feel free to use, modify, and distribute.

# Cylon

Cylon is a **LoRaWAN simulator** — a web application that fabricates a fleet of
synthetic end-devices ("tags") and a gateway, drives them through a full product
cycle against **AWS IoT Core for LoRaWAN** using the **LoRa Basics Station**
protocol, and is managed through a web UI. No radio hardware: tags talk to the
gateway over TCP, and the gateway forwards to AWS over Basic Station (WebSocket).
All state lives in **SQLite**, and the SPA is embedded into a single Go binary.

> **Status:** early development. Phases 0–1 are in place (scaffolding,
> persistence, gateway identity, and the tag PHY core); the gateway/Basic
> Station client, TCP transport, REST/WebSocket API, and SPA land in subsequent
> phases.

## Features

**Phase 0 — scaffolding & persistence**
- Single static binary (`CGO_ENABLED=0`, pure-Go SQLite via `modernc.org/sqlite`).
- SQLite persistence with embedded, versioned migrations (goose).
- Gateway **EUI-64 generated and persisted on first run**, stable across
  restarts, overridable via config/env/flag.
- Bootstrap configuration via YAML + environment overrides.
- Minimal health endpoint and graceful shutdown.

**Phase 1 — tag PHY core**
- OTAA 1.0.3 join (request build, accept parse) with **NwkSKey/AppSKey
  derivation** via `brocaar/lorawan`.
- Data uplink build and downlink decode (FRMPayload decrypt + MAC-command
  surfacing).
- Payload generators: static, counter, random, ramp, sine.
- Per-device session persistence: **monotonic, never-reused DevNonce** and frame
  counters survive restarts.
- Sensitive columns (AppKey, session keys) **encrypted at rest** (AES-256-GCM,
  AAD-bound) via `CYLON_DB_KEY`.

## Quick start

```sh
# Build
go build -o cylon ./cmd/cylon

# Generate a starter config
./cylon gen-config > cylon.yaml

# Run (creates the DB, migrates, generates the gateway EUI, serves /healthz)
CYLON_STORE_PATH=./cylon.db ./cylon serve

# In another shell:
curl -s localhost:8080/healthz
# {"status":"ok","version":"dev","eui":"…"}
```

## CLI

| Command | Description |
|---|---|
| `cylon serve` | Run the web app (HTTP server + database). |
| `cylon migrate [up\|down\|status]` | Run database migrations. |
| `cylon gateway-eui [--set <eui>]` | Show or override the gateway EUI. |
| `cylon gen-config` | Print an example configuration to stdout. |
| `cylon version` | Print the build version. |

Global flag: `-c, --config <path>` selects a YAML config file.

## Configuration

Settings resolve in the order **environment (`CYLON_*`) → config file → built-in
default**. Only bootstrap settings live in config; runtime data (gateway, tags,
sessions) lives in the database.

| Setting | Env | Default | Description |
|---|---|---|---|
| `server.http_listen` | `CYLON_SERVER_HTTP_LISTEN` | `:8080` | UI/API + `/ws` listen address. |
| `server.metrics_listen` | `CYLON_SERVER_METRICS_LISTEN` | `:9100` | Prometheus listen address. |
| `server.log_level` | `CYLON_SERVER_LOG_LEVEL` | `info` | `debug`/`info`/`warning`/`error`. |
| `store.path` | `CYLON_STORE_PATH` | `/var/lib/cylon/cylon.db` | SQLite database file. |
| `gateway.tcp_listen` | `CYLON_GATEWAY_TCP_LISTEN` | `:6000` | Tag TCP listen address. |
| `gateway.eui` | `CYLON_GATEWAY_EUI` | _(generated)_ | Override the gateway EUI (16 hex). |
| `gateway.eui_prefix` | `CYLON_GATEWAY_EUI_PREFIX` | _(none)_ | Optional EUI prefix; a 3-byte OUI is expanded with `FFFE`. |
| `gateway.connection.creds_dir` | `CYLON_GATEWAY_CONNECTION_CREDS_DIR` | `/etc/cylon/creds` | Basic Station credential volume. |
| `sim.realtime` | `CYLON_SIM_REALTIME` | `true` | Real-time vs. accelerated clock. |

### Secrets at rest

Sensitive columns (AppKey, session keys) are encrypted with AES-256-GCM when
`CYLON_DB_KEY` is set to a 32-byte key (64 hex chars or base64):

```sh
export CYLON_DB_KEY="$(openssl rand -hex 32)"
```

If unset, the binary runs in dev mode and stores these values **unencrypted**
with a loud warning. The API never returns full keys (masked to the last 4).

## Docker

```sh
docker build -t cylon .
docker run --rm -p 8080:8080 \
  -v "$PWD/data:/var/lib/cylon" \
  -v "$PWD/creds:/etc/cylon/creds" \
  cylon
```

## Development

```sh
go test ./...        # unit tests
go vet ./...
```

## License

Apache-2.0. See [LICENSE](LICENSE).

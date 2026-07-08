# PulseOrPerish

Dead man switch in Go.

Protect what matters with a simple heartbeat: as long as you are alive, your data stays safe; if you stop checking in, **PulseOrPerish** automatically wipes the target directory. Lightweight, self-hosted, and ready in minutes with a web UI, API, and container support.

## Features
- HTTP UI to submit proof-of-life with a password
- REST API with same capabilities
- Persistent heartbeat state surviving container restarts
- Automatic data directory content wipe when deadline is exceeded
- Configurable via CLI flags and environment variables
- Distroless-compatible container image

![webui](./img/webui.png)

## Configuration
Priority: flags > environment variables > defaults.

| Description | Env variable | Flag | Default | Values / Example |
|---|---|---|---|---|
| Authentication password | `POP_PASSWORD` | `--password` | *(required)* | `mysecret` |
| Interval between proofs | `POP_INTERVAL` | `--interval` | `720h` | `1h`, `24h`, `720h` |
| Dry-run mode (no deletion) | `POP_DRY_RUN` | `--dry-run` | `false` | `true`, `false` |
| Directory to wipe on deadline | `POP_DATA_DIR` | `--data-dir` | *(required)* | `/data` (absolute path) |
| Directory for state persistence | `POP_STATE_DIR` | `--state-dir` | `/state` | `/var/lib/pop/state` |
| Log output path | `POP_LOG_PATH` | `--log-path` | stdout | `/var/log/pop.log` |
| Log level | `POP_LOG_LEVEL` | `--log-level` | `info` | `debug`, `info`, `warn`, `error`, `critical` |
| HTTP listen address | `POP_LISTEN` | `--listen` | `:8080` | `:8086`, `0.0.0.0:8080` |

## API
- `GET /health` no auth
- `GET /` no auth
- `POST /alive` auth required
- `GET /status` no auth
- `POST /api/v1/alive` auth required
- `GET /api/v1/status` auth required

Authentication: `Authorization: Bearer <password>`

## API examples
```bash
BASE_URL="http://localhost:8086"
PASSWORD="mysecret"
```

Health check:
```bash
curl -s "$BASE_URL/health"
```

Public status:
```bash
curl -s "$BASE_URL/status" | jq .
```

Proof of life (auth required):
```bash
curl -s -X POST "$BASE_URL/alive" \
  -H "Authorization: Bearer $PASSWORD"
```

Protected API v1 status:
```bash
curl -s "$BASE_URL/api/v1/status" \
  -H "Authorization: Bearer $PASSWORD" | jq .
```

## Run locally
```bash
go run ./cmd/pulseorperish \
  --listen=':8086' \
  --password='mysecret' \
  --data-dir='/tmp/pop-data' \
  --state-dir='/tmp/pop-state' \
  --dry-run='true' \
  --interval='1m'
```

## Tests
Unit tests:
```bash
go test ./...
```

End-to-end tests:
```bash
go test ./internal/testkit/e2e -v -timeout 30m
```

## Build container
```bash
docker build -t pulseorperish:local .
```

## Example docker run

You can also use the [docker-compose](./docker-compose.yml) example

```bash
docker run --rm -it -p 8086:8080 \
  -e POP_PASSWORD=mysecret \
  -e POP_DATA_DIR=/data \
  -e POP_STATE_DIR=/state \
  -v $(pwd)/demo-data:/data \
  -v $(pwd)/demo-state:/state \
  pulseorperish:local
```


# PulseOrPerish

Dead man switch in Go.

## Features
- HTTP UI to submit proof-of-life with a password
- REST API with same capabilities
- Persistent heartbeat state surviving container restarts
- Automatic data directory content wipe when deadline is exceeded
- Configurable via CLI flags and environment variables
- Distroless-compatible container image

## Configuration
Priority: flags > environment variables > defaults.

- `--password` / `POP_PASSWORD` (required)
- `--interval` / `POP_INTERVAL` (default: `720h`, one month)
- `--dry-run` / `POP_DRY_RUN` (default: `false`, logs actions but does not delete)
- `--data-dir` / `POP_DATA_DIR` (required, absolute path)
- `--state-dir` / `POP_STATE_DIR` (default: `/state`)
- `--log-path` / `POP_LOG_PATH` (default: stdout)
- `--log-level` / `POP_LOG_LEVEL` (`debug|info|warn|error|critical`, default: `info`)
- `--listen` / `POP_LISTEN` (default: `:8080`)

## API
- `GET /health` no auth
- `GET /` no auth
- `POST /alive` auth required
- `GET /status` no auth
- `POST /api/v1/alive` auth required
- `GET /api/v1/status` auth required

Authentication: `Authorization: Bearer <password>`

## Extension points
- Proof-of-life channel extension: the monitor accepts a source identifier when registering a proof (`RegisterProof(source)`), so future providers (webhook, CLI, messaging bot) can be added without changing deletion logic.
- Deletion policy extension: deletion is behind a `Deleter` interface. V1 provides safe directory-content wipe; V2 can add policy-specific implementations.
- Multi-directory roadmap: V1 manages one directory. V2 can evolve to map `directory -> policy` while reusing monitor/store/http layers.

## Run locally
```bash
go run ./cmd/pulseorperish \
  --listen=':8086' \
  --password='secret' \
  --data-dir='/tmp/pop-data' \
  --state-dir='/tmp/pop-state' \
  --dry-run='true' \
  --interval='1m'
```

## Build container
```bash
docker build -t pulseorperish:local .
```

## Example docker run
```bash
docker run --rm -it -p 8086:8080 \
  -e POP_PASSWORD=secret \
  -e POP_DATA_DIR=/data \
  -e POP_STATE_DIR=/state \
  -v $(pwd)/demo-data:/data \
  -v $(pwd)/demo-state:/state \
  pulseorperish:local
```


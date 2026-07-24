# PulseOrPerish

[![CI](https://github.com/jerome-labidurie/PulseOrPerish/actions/workflows/ci.yml/badge.svg)](https://github.com/jerome-labidurie/PulseOrPerish/actions/workflows/ci.yml)
[![Release](https://github.com/jerome-labidurie/PulseOrPerish/actions/workflows/release.yml/badge.svg)](https://github.com/jerome-labidurie/PulseOrPerish/actions/workflows/release.yml)
[![Latest release](https://img.shields.io/github/v/release/jerome-labidurie/PulseOrPerish)](https://github.com/jerome-labidurie/PulseOrPerish/releases/latest)
[![Docker image](https://img.shields.io/badge/ghcr.io-pulseorperish-blue?logo=docker)](https://github.com/jerome-labidurie/PulseOrPerish/pkgs/container/pulseorperish)

Dead man switch that deletes your precious data.

Protect what matters with a simple heartbeat: as long as you are alive, your data stays safe; if you stop checking in, **PulseOrPerish** automatically wipes or encrypts the target directories. Lightweight, self-hosted, and ready in minutes with a web UI, API, and container support.

## Quick start
```bash
mkdir /tmp/demo-data /tmp/demo-state
docker run --rm -it -p 8086:8080 \
  -e POP_PASSWORD=mysecret \
  -e POP_DATA_DIRS=/data \
  -v /tmp/demo-data:/data \
  -v /tmp/demo-state:/state \
  ghcr.io/jerome-labidurie/pulseorperish:latest
```
Then open [http://localhost:8086](http://localhost:8086) and send a proof-of-life by clicking the red button.

You can also use the [docker-compose](./docker-compose.yml) example

## Features
- HTTP User Interface to submit proof-of-life with a password (with Dark mode)
- REST API with same capabilities
- Persistent heartbeat state surviving container restarts
- Automatic data directories content wipe or encrypt when deadline is exceeded
- Simple or secure deletion using [wipe](https://wipe.sourceforge.net/)
- Configurable via CLI flags and environment variables
- Distroless-compatible container image

<p>
  <a href="./img/webui_light.png">
    <img src="./img/webui_light.png" alt="PulseOrPerish web UI light mode" width="320" />
  </a>
  <a href="./img/webui_dark.png">
    <img src="./img/webui_dark.png" alt="PulseOrPerish web UI dark mode" width="320" />
  </a>
</p>

## Configuration
Priority: flags > environment variables > defaults.

| Description | Env variable | Flag | Default | Values / Example |
|---|---|---|---|---|
| Authentication password | `POP_PASSWORD` | `--password` | *(required)* | `mysecret` |
| Directories to wipe on deadline | `POP_DATA_DIRS` | `--data-dirs` | *(required)* | `/data`, `/photos,/media/videos` |
| Max interval between proofs | `POP_INTERVAL` | `--interval` | `720h` | `24h`, `720h` ([format](https://pkg.go.dev/time#ParseDuration)) |
| Deletion method | `POP_DELETE_METHOD` | `--delete-method` | `rm` | `rm`, `wipe`, `crypt/rm`, `crypt/wipe` |
| Encrypt password | `POP_CRYPT_PASSWORD` | `--crypt-password` | (same as `POP_PASSWORD`) | See below |
| Directory for state persistence | `POP_STATE_DIR` | `--state-dir` | `/state` | `/var/lib/pop/state` |
| Dry-run mode (no deletion) | `POP_DRY_RUN` | `--dry-run` | `false` | `true`, `false` |
| Arguments for wipe | `POP_WIPE_ARGS` | `--wipe-args` | `-q -Q 1` | `-q -Q 3 -e` ([details](https://linux.die.net/man/1/wipe)) |
| Log directory | `POP_LOG_PATH` | `--log-path` | (stdout only) | `/var/log/pop/` (directory; if set, a timestamped file is also created) |
| Log level | `POP_LOG_LEVEL` | `--log-level` | `info` | `debug`, `info`, `warn`, `error` |
| HTTP listen address | `POP_LISTEN` | `--listen` | `:8080` | `:8086`, `0.0.0.0:8080` |

### Deletion methods
**`rm`** (default): uses Go's [`os.RemoveAll`](https://pkg.go.dev/os#RemoveAll); no external dependency.

**`wipe`**: invokes the [`wipe`](https://wipe.sourceforge.net/) utility to securely overwrite data before deletion. Defaults options (See `POP_WIPE_ARGS`) are *not very secure*. Execution can be **very** long.

Use `wipe` with care: effectiveness depends on storage and filesystem behavior (especially on SSDs).

**`crypt/rm`**, **`crypt/wipe`** encrypt the data before deleting the original with `rm` or `wipe` methods. So the data can be recovered later (assuming the password is known). Use `POP_CRYPT_PASSWORD` to provide it :
* if not provided, uses the value from `POP_PASSWORD`
* `mySecretPassword` directly provides a password
* `file:/data/password_in_file` gets the password from a file (recommended in production)
* `random` create a random password when needed, (**data will not be recoverable**)

The encryption is based on [libsodium](https://libsodium.gitbook.io/doc) and uses the [XChaCha20-Poly1305](https://en.wikipedia.org/wiki/ChaCha20-Poly1305) symetric algorithm. A companion tool is provided for encryption/decryption, see [popcrypt](./cmd/popcrypt/).

### Recovery
When using `crypt/rm` or `crypt/wipe`, encrypted archives (`*.pop`) are created in each configured data directory.
Keep the encryption password safe, then decrypt with [popcrypt](./cmd/popcrypt/README.md).

## Home Assistant
See [homeassistant.md](./homeassistant.md) for a REST sensor and notifications automation example.

## Run locally
```bash
./pulseorperish \
  --listen=':8086' \
  --password='mysecret' \
  --data-dirs="$(pwd)/demo-data" \
  --state-dir="$(pwd)/demo-state" \
  --dry-run='true' \
  --log-level='debug' \
  --interval='5m'
```

## Build

### Build container image
```bash
docker build -t pulseorperish:local .
```

### Build pulseorperish binary (Linux dependencies)
```bash
sudo apt-get update
sudo apt-get install -y wipe libsodium-dev
go build -o pulseorperish ./cmd/pulseorperish
```

## API
- `GET /` (public) HTTP UI
- `GET /health` (public)
- `GET /status` (public)
- `POST /alive` (authenticated)

Authentication can be done via 2 methods:
* Header: `Authorization: Bearer <password>`
* json data: `{"password":"<password>"}`

### API examples
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
# or
curl -s -X POST "$BASE_URL/alive" \
  -H "Content-Type: application/json" \
  --data '{"password":"'${PASSWORD}'"}'
```

## Tests
All tests:
```bash
go test -v -p 1 ./...
```

Unit tests only:
```bash
go test $(go list ./... | grep -v 'internal/testkit/e2e')
```

End-to-end tests only:
```bash
go test ./internal/testkit/e2e -v -timeout 30m
```

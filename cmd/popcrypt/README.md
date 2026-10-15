# popcrypt

`popcrypt` is the companion CLI for encrypted archives produced by PulseOrPerish.

It can:
- encrypt files or directories into `.pop` archives
- decrypt `.pop` archives back to a compressed tar stream

## Typical Recovery Flow (from PulseOrPerish)

When PulseOrPerish uses `crypt/rm` or `crypt/wipe`, it creates archives like:
- `file_0000.tar.gz.pop`
- `file_0001.tar.gz.pop`

Those files are your recoverable data.

Recovery has 2 steps:
1. Decrypt `.pop` to a compressed tar file.
2. Extract the tar file.

## Usage

### Show help

```bash
./popcrypt --help
```

### Build

```bash
go build -o popcrypt ./cmd/popcrypt
```

## Examples

### Decrypt and Restore
Decrypt (assuming `POP_CRYPT_PASSWORD` was configured with `mySecretPassword`) :
```bash
./popcrypt -p "mySecretPassword" -d file_0000.tar.gz.pop
```

This produces:
- `file_0000.tar.gz`

Extract:
```bash
tar -xvzf file_0000.tar.gz
```

### From a Docker container
If you prefer not to build locally, you can run `popcrypt` from a container.

```bash
# Use the published image from the repository:
# you may want to use your version instead of latest
IMAGE="ghcr.io/jerome-labidurie/pulseorperish:latest"
# Decrypt
docker run --rm -it \
  -v "$(pwd)":/work \
  -w /work \
  --entrypoint /popcrypt \
  "$IMAGE" -p "mySecretPassword" -d file_0000.tar.gz.pop
```

### Encrypt
Encrypt one or more input paths (file or directory):
```bash
./popcrypt -p "mySecretPassword" -e /path/to/data1 /path/to/data2
```

Output files are created in the current directory:
- `file_0000.tar.gz.pop`
- `file_0001.tar.gz.pop`

### lzw
*PulseOrPerish* will not produce `lzw` compressed archives. But `popcrypt` can

```bash
# encrypt
./popcrypt -p "mySecretPassword" -comp lzw -e /path/to/data
# decrypt
./popcrypt -p "mySecretPassword" -d file_0000.tar.lzw.pop
# extract
uncompress file_0000.tar.lzw
tar -xvf file_0000.tar
```

FROM golang:1.26-trixie AS builder
WORKDIR /src
COPY go.mod go.sum ./
# don't care about the exact versions
# hadolint ignore=DL3008
RUN go mod download; \
    apt-get update; \
    apt-get install -y --no-install-recommends git wipe
COPY . .

ARG VERSION=dev

RUN BUILD_DATE=$(date -u '+%Y-%m-%dT%H:%M:%SZ') && \
    COMMIT=$(git rev-parse --short HEAD) && \
    CGO_ENABLED=0 GOOS=linux GOARCH=amd64 go build -trimpath -ldflags="-s -w -X main.Version=${VERSION} -X main.BuildDate=${BUILD_DATE} -X main.CommitHash=${COMMIT}" -o /out/pulseorperish ./cmd/pulseorperish

# no tag for distroless
# hadolint ignore=DL3006
FROM gcr.io/distroless/static-debian13 AS runner

WORKDIR /
COPY --from=builder /out/pulseorperish /pulseorperish
COPY --from=builder /usr/bin/wipe /usr/bin/wipe
COPY --from=builder /lib/x86_64-linux-gnu/libc.so.6 /lib/x86_64-linux-gnu/libc.so.6
COPY --from=builder /lib64/ld-linux-x86-64.so.2 /lib64/ld-linux-x86-64.so.2

EXPOSE 8080

ARG VERSION=dev
ARG CREATED
ARG SOURCE_URL

LABEL org.opencontainers.image.title="PulseOrPerish" \
      org.opencontainers.image.description="Dead man switch in Go" \
      org.opencontainers.image.version="${VERSION}" \
      org.opencontainers.image.created="${CREATED}" \
      org.opencontainers.image.source="${SOURCE_URL}" \
      org.opencontainers.image.url="https://github.com/jerome-labidurie/PulseOrPerish" \
      org.opencontainers.image.licenses="GPL-3.0-only"

ENTRYPOINT ["/pulseorperish"]

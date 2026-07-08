FROM golang:1.26-alpine AS builder
WORKDIR /src
COPY go.mod go.sum ./
RUN go mod download
COPY . .
RUN CGO_ENABLED=0 GOOS=linux GOARCH=amd64 go build -trimpath -ldflags="-s -w" -o /out/pulseorperish ./cmd/pulseorperish

FROM gcr.io/distroless/static-debian13
WORKDIR /
COPY --from=builder /out/pulseorperish /pulseorperish
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

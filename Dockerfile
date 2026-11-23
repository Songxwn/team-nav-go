# syntax=docker/dockerfile:1.7

FROM golang:1.27-alpine AS build

WORKDIR /src

# Dependencies are cached in their own layer.
COPY go.mod go.sum ./
RUN go mod download

COPY . .

ARG VERSION=dev
RUN CGO_ENABLED=0 GOOS=linux go build \
    -trimpath \
    -ldflags "-s -w -X main.version=${VERSION}" \
    -o /out/teamnav ./cmd/teamnav

FROM alpine:3.22

RUN apk add --no-cache ca-certificates tzdata \
    && addgroup -g 10001 -S app \
    && adduser -u 10001 -S -G app app \
    && mkdir -p /data \
    && chown app:app /data

COPY --from=build /out/teamnav /usr/local/bin/teamnav

ENV TEAMNAV_ADDR=:8080 \
    TEAMNAV_DATA_DIR=/data \
    TEAMNAV_IMAGE_CACHE_MB=64

WORKDIR /app
USER app:app

EXPOSE 8080
VOLUME ["/data"]

HEALTHCHECK --interval=30s --timeout=3s --start-period=5s --retries=3 \
    CMD wget -qO- http://127.0.0.1:8080/api/healthz >/dev/null || exit 1

ENTRYPOINT ["/usr/local/bin/teamnav"]
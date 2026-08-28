# Build stage
FROM golang:1.27.0-alpine@sha256:4c9fe60190a2a3350ddc51de80d0224b8a6698d12bdfc999fee45ea9d6c46dbc AS builder

WORKDIR /app

# Install build dependencies
RUN apk add --no-cache \
    gcc=15.2.0-r5 \
    musl-dev=1.2.6-r2

# Copy module files
COPY go.mod go.sum ./
RUN go mod download

# Copy source code
COPY . .

# Build the application
RUN CGO_ENABLED=1 go test ./... && \
    CGO_ENABLED=1 go build -trimpath -ldflags="-s -w" -o image-resizer ./cmd/server/main.go

# Final stage
FROM alpine:3.24.1@sha256:28bd5fe8b56d1bd048e5babf5b10710ebe0bae67db86916198a6eec434943f8b

WORKDIR /app

# Install fonts for text overlay and ca-certificates for HTTPS
RUN apk add --no-cache \
    ca-certificates=20260611-r0 \
    font-dejavu=2.37-r6 \
    libcrypto3=3.5.8-r0 \
    libssl3=3.5.8-r0 && \
    addgroup -S -g 10001 app && \
    adduser -S -D -H -u 10001 -G app app

# Copy binary from builder
COPY --from=builder --chown=10001:10001 /app/image-resizer .

# BUG-16 FIX: Copy both web/ and static/ directories into the image.
# Previously, only web/ was copied, but the app also needs static/
# for uploads and processed file storage.
COPY --from=builder --chown=10001:10001 /app/web ./web
COPY --from=builder --chown=10001:10001 /app/static ./static

# Ensure directories exist with proper permissions
RUN mkdir -p static/uploads static/processed && \
    chown -R 10001:10001 static && \
    chmod 750 static/uploads static/processed

EXPOSE 5000

ENV PORT=5000
ENV ENV=production

HEALTHCHECK --interval=30s --timeout=5s --start-period=10s --retries=3 \
    CMD wget --quiet --spider http://127.0.0.1:5000/healthz || exit 1

USER 10001:10001

CMD ["./image-resizer"]

# Build stage
FROM golang:1.27.2-alpine3.24@sha256:f92b6ef800e499660581efdabdf25d9d817a9d124eaf900924f0504e7e27e12d AS builder

WORKDIR /app

# Install build dependencies
RUN apk add --no-cache gcc musl-dev

# Copy module files
COPY go.mod go.sum ./
RUN go mod download

# Copy source code
COPY . .

# Build the application
RUN CGO_ENABLED=1 go build -ldflags="-s -w" -o image-resizer ./cmd/server/main.go

# Final stage
FROM alpine:3.24@sha256:71e9b7bae160ba167fa17414401b94cbcdaf3c32feeab9298bd794a2a0614c35

WORKDIR /app

# Install fonts for text overlay and ca-certificates for HTTPS
RUN apk add --no-cache ttf-dejavu ca-certificates

# Copy binary from builder
COPY --from=builder /app/image-resizer .

# BUG-16 FIX: Copy both web/ and static/ directories into the image.
# Previously, only web/ was copied, but the app also needs static/
# for uploads and processed file storage.
COPY --from=builder /app/web ./web
COPY --from=builder /app/static ./static

# Run as an unprivileged user; only the runtime data directories are writable.
RUN addgroup -S app && adduser -S -G app -H -h /app app && \
    mkdir -p static/uploads static/processed && \
    chown app:app static/uploads static/processed && \
    chmod 750 static/uploads static/processed

EXPOSE 5000

ENV PORT=5000 \
    ENV=production

USER app

CMD ["./image-resizer"]

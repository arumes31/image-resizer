# Build stage
FROM golang:1.26.4-alpine AS builder

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
FROM alpine:latest

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

# Ensure directories exist with proper permissions
RUN mkdir -p static/uploads static/processed && \
    chmod 750 static/uploads static/processed

EXPOSE 5000

ENV PORT=5000

CMD ["./image-resizer"]

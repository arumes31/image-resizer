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
RUN go build -o image-resizer ./cmd/server/main.go

# Final stage
FROM alpine:latest

WORKDIR /app

# Install fonts for text overlay
RUN apk add --no-cache ttf-dejavu

# Copy binary and static assets
COPY --from=builder /app/image-resizer .
COPY --from=builder /app/web ./web

# Ensure directories exist
RUN mkdir -p static/uploads static/processed

EXPOSE 5000

ENV PORT=5000

CMD ["./image-resizer"]

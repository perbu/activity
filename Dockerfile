# Build stage
FROM golang:1.25-alpine AS builder

RUN apk add --no-cache git

WORKDIR /app

# Copy go mod files first for better caching
COPY go.mod go.sum ./
RUN go mod download

# Copy source code
COPY . .

# Build the binary
RUN CGO_ENABLED=0 GOOS=linux go build -ldflags="-w -s" -o activity .

# Runtime stage
FROM alpine:latest

LABEL org.opencontainers.image.source=https://github.com/perbu/activity
LABEL org.opencontainers.image.description="AI-powered git commit analyzer"
LABEL org.opencontainers.image.licenses=BSD-2-Clause

RUN apk add --no-cache git ca-certificates

# Create non-root user
RUN adduser -D -g '' appuser

COPY --from=builder /app/activity /usr/local/bin/activity

USER appuser

ENTRYPOINT ["activity"]

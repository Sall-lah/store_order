# ==============================================================================
# Stage 1: Build Stage
# ==============================================================================
FROM golang:1.26-alpine AS builder

WORKDIR /app

# Install system dependencies required for Go modules, Prisma engine fetching, and SSL
RUN apk add --no-cache git ca-certificates openssl

# Pre-copy module manifests to leverage layer caching
COPY go.mod go.sum ./
RUN go mod download

# Copy Prisma schema and generate Linux-compatible Prisma Client Go engine
COPY prisma/ ./prisma/
RUN go run github.com/steebchen/prisma-client-go generate --schema=./prisma/schema.prisma

# Copy application source code and documentation
COPY cmd/ ./cmd/
COPY internal/ ./internal/
COPY docs/ ./docs/

# Build static binary targeting Linux
RUN CGO_ENABLED=0 GOOS=linux go build \
    -ldflags="-w -s" \
    -o /app/bin/server \
    ./cmd/server/main.go

# ==============================================================================
# Stage 2: Minimal Runtime Stage
# ==============================================================================
FROM alpine:3.21 AS runner

# Install runtime dependencies required by Prisma engine and TLS connections
RUN apk add --no-cache ca-certificates openssl tzdata curl

# Create unprivileged application user
RUN addgroup -g 10001 -S appgroup && \
    adduser -u 10001 -S appuser -G appgroup

WORKDIR /app

# Copy compiled binary from builder stage
COPY --from=builder --chown=appuser:appgroup /app/bin/server /app/server
COPY --from=builder --chown=appuser:appgroup /app/docs /app/docs

# Switch to non-root user
USER appuser:appgroup

# Environment variable defaults
ENV PORT=8060 \
    DEV=false \
    ENABLE_DOCS=true

EXPOSE 8060

# Built-in health probe using the /health endpoint
HEALTHCHECK --interval=30s --timeout=5s --start-period=5s --retries=3 \
    CMD curl -f http://localhost:${PORT}/health || exit 1

ENTRYPOINT ["/app/server"]

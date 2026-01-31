# AI Swarm Prediction Hub - Backend
# Railway auto-detects this Dockerfile at root

FROM golang:1.25-alpine AS builder

WORKDIR /src

# Copy go mod files first for caching
COPY backend/go.mod backend/go.sum ./
RUN go mod download

# Copy source and build
COPY backend/ .
RUN CGO_ENABLED=0 GOOS=linux go build -ldflags="-s -w" -o /app/server .

# Production image
FROM alpine:3.20

# Security: non-root user
RUN addgroup -S app && adduser -S -G app app

# Copy binary
COPY --from=builder /app/server /usr/local/bin/server
RUN chown app:app /usr/local/bin/server

USER app

# Railway injects PORT env var
ENV PORT=8080
EXPOSE 8080

# Health check endpoint
HEALTHCHECK --interval=30s --timeout=3s --start-period=5s --retries=3 \
  CMD wget --no-verbose --tries=1 --spider http://localhost:8080/health || exit 1

ENTRYPOINT ["/usr/local/bin/server"]

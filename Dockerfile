# Build stage
FROM golang:alpine AS builder

WORKDIR /app

ENV GOTOOLCHAIN=auto

# Install build dependencies
RUN apk add --no-cache ca-certificates tzdata git

# Cache dependencies
COPY go.mod go.sum ./
RUN go mod download

# Copy source code
COPY . .

# Build lightweight static binary
RUN CGO_ENABLED=0 GOOS=linux go build -a -installsuffix cgo -ldflags="-w -s" -o /app/server ./cmd/server

# Final stage
FROM alpine:3.21

# Install CA certificates and timezone data
RUN apk add --no-cache ca-certificates tzdata

# Create non-root user for security
RUN adduser -D -g '' appuser

WORKDIR /app

# Copy binary and migrations from builder
COPY --from=builder /app/server /app/server
COPY --from=builder /app/internal/database/migrations /app/internal/database/migrations

# Set permissions
RUN chown -R appuser:appuser /app

USER appuser

EXPOSE 8080

ENTRYPOINT ["/app/server"]

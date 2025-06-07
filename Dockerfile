FROM golang:1.21-alpine AS builder

WORKDIR /app

# Install build dependencies
RUN apk add --no-cache git make

# Copy go.mod and go.sum first to leverage Docker cache
COPY go.mod go.sum ./
RUN go mod download

# Copy the rest of the source code
COPY . .

# Build the application
RUN make build

FROM alpine:3.18

LABEL org.opencontainers.image.title="fail2ban-notify"
LABEL org.opencontainers.image.description="A modern, multi-platform notification system for Fail2Ban"
LABEL org.opencontainers.image.source="https://github.com/eyeskiller/fail2ban-notify-go"
LABEL org.opencontainers.image.licenses="MIT"

# Install runtime dependencies
RUN apk add --no-cache ca-certificates tzdata bash curl jq

# Create directories
RUN mkdir -p /etc/fail2ban/action.d /etc/fail2ban/connectors

# Copy binary from builder stage
COPY --from=builder /app/dist/fail2ban-notify-linux-amd64 /usr/local/bin/fail2ban-notify

# Copy configuration files
COPY configs/notify.conf /etc/fail2ban/action.d/
COPY connectors/ /etc/fail2ban/connectors/

# Make connectors executable
RUN chmod +x /etc/fail2ban/connectors/*

# Set entrypoint
ENTRYPOINT ["/usr/local/bin/fail2ban-notify"]
CMD ["-help"]

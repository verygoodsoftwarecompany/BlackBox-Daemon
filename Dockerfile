# Build stage
FROM golang:1.25-alpine AS builder

WORKDIR /app

# Install dependencies
RUN apk add --no-cache git ca-certificates

# Copy go mod files
COPY go.mod go.sum ./
RUN go mod download

# Copy source code
COPY . .

# Build the application
RUN CGO_ENABLED=0 GOOS=linux go build -o blackbox-daemon ./cmd/blackbox-daemon

# Final stage
FROM alpine:3.18

# Install ca-certificates for HTTPS requests
RUN apk --no-cache add ca-certificates

WORKDIR /root/

# Copy the binary from builder stage
COPY --from=builder /app/blackbox-daemon .

# Create log directory
RUN mkdir -p /var/log/blackbox

# Expose ports
EXPOSE 8080 9090

# Run as non-root user
RUN adduser -D -s /bin/sh blackbox
USER blackbox

CMD ["./blackbox-daemon"]
# Build stage
FROM golang:1.24-alpine AS builder

WORKDIR /app

# Copy go mod files
COPY go.mod go.sum ./

# Download dependencies
RUN go mod download

# Copy source code
COPY . .

# Build the application
RUN CGO_ENABLED=0 GOOS=linux go build -a -installsuffix cgo -o tempo-s3-shard .

# Runtime stage
FROM alpine:3.21

# Install ca-certificates for SSL connections
RUN apk --no-cache add ca-certificates

WORKDIR /root/

# Copy the binary from builder
COPY --from=builder /app/tempo-s3-shard .

# Create config directory
RUN mkdir -p /etc/tempo-s3-shard

# Expose port
EXPOSE 8080

# Run the binary
CMD ["./tempo-s3-shard", "-config", "/etc/tempo-s3-shard/config.json"]
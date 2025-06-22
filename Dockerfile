# Build stage
FROM --platform=$BUILDPLATFORM golang:1.24-alpine AS builder

WORKDIR /app

# Copy go mod files
COPY go.mod go.sum ./

# Download dependencies
RUN go mod download

# Copy source code
COPY . .

# Build the application
ARG TARGETOS
ARG TARGETARCH
RUN CGO_ENABLED=0 GOOS=${TARGETOS} GOARCH=${TARGETARCH} go build -o tempo-s3-shard .

# Runtime stage
FROM alpine:3.21

# Install ca-certificates and timezone data for SSL connections
RUN apk --no-cache add ca-certificates tzdata

WORKDIR /root/

# Copy the binary from builder
COPY --from=builder /app/tempo-s3-shard .

# Create config directory
RUN mkdir -p /etc/tempo-s3-shard

# Set timezone environment variable (can be overridden)
ENV TZ=Asia/Taipei

# Expose ports
EXPOSE 8080
EXPOSE 9090

# Run the binary
CMD ["./tempo-s3-shard", "-config", "/etc/tempo-s3-shard/config.json"]
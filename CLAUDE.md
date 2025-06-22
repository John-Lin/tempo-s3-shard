# CLAUDE.md

This file provides guidance to Claude Code (claude.ai/code) when working with code in this repository.

## Development Commands

**Build the application:**
```bash
go build -o tempo-s3-shard
```

**Run with configuration:**
```bash
./tempo-s3-shard -config config.json
```

**Development dependencies:**
```bash
go mod tidy
```

## Architecture Overview

This is an S3-compatible shard server that distributes objects across multiple S3 buckets using path-prefix-based consistent hashing. The application is optimized for Grafana Tempo's trace storage patterns.

### Core Components

**Request Flow:** Client → TempoS3ShardServer → S3ClientManager → ConsistentHash → Backend S3 Buckets

1. **TempoS3ShardServer** (`internal/server/server.go`): HTTP server implementing S3 REST API endpoints. Routes requests based on HTTP method and path parsing, handles all standard S3 operations (GET, PUT, DELETE, HEAD, LIST, tagging). Includes bucket location endpoint support for client compatibility.

2. **S3ClientManager** (`internal/client/s3client.go`): Manages the MinIO client connection and integrates with the consistent hashing algorithm. Determines target bucket for each operation and handles bucket lifecycle.

3. **ConsistentHash** (`internal/hash/consistent.go`): SHA-256 based consistent hashing with virtual nodes (100 replicas per bucket). Uses **path prefix hashing** - extracts first two path segments for grouping related objects (e.g., `single-tenant/trace-id/file` → hash key: `single-tenant/trace-id`).

4. **Configuration** (`internal/config/config.go`): Manages all backend S3 connections. Supports both full URLs (`https://endpoint.com`) and hostnames (`endpoint.com`) for flexible SSL configuration.

5. **Metrics** (`internal/metrics/metrics.go`): Comprehensive Prometheus metrics for observability. Tracks HTTP requests, S3 operations, object sizes, hash distribution, and performance metrics.

6. **Structured Logging**: Uses Go's `log/slog` package with logfmt format for machine-readable logs. All log statements include structured fields for better observability.

### Key Design Patterns

- **Virtual Bucket**: Clients see a single bucket name (`proxy-bucket`), but objects are transparently distributed across multiple backend buckets
- **Path Prefix Grouping**: Related objects are grouped by path prefix (first two segments) for data locality
- **Aggregated Operations**: LIST operations query all backend buckets and merge results
- **Deterministic Routing**: Same path prefix always maps to the same backend bucket for consistency
- **Auto-provisioning**: Backend buckets are created automatically if they don't exist
- **Tempo Optimization**: Trace files for same trace ID are co-located in the same backend bucket

### S3 API Compatibility

Supports all operations required by Grafana Tempo:
- ListBuckets, ListObjects, PutObject, GetObject, DeleteObject, HeadObject
- GetObjectTagging, PutObjectTagging
- GetBucketLocation (for client compatibility)

Tempo S3 Shard presents a unified S3 interface while internally distributing data for scalability and load balancing.

### Path Prefix Hashing Examples

For Grafana Tempo paths like `single-tenant/{trace-id}/{file-type}`:

| Full Object Path | Extracted Hash Key | Target Bucket | Benefit |
|------------------|--------------------|--------------:|---------|
| `single-tenant/abc123/bloom-0` | `single-tenant/abc123` | shard1 | All trace data together |
| `single-tenant/abc123/bloom-1` | `single-tenant/abc123` | shard1 | ↳ Same trace, same bucket |
| `single-tenant/abc123/chunks-001` | `single-tenant/abc123` | shard1 | ↳ Query efficiency |
| `single-tenant/def456/bloom-0` | `single-tenant/def456` | shard2 | Different trace, load balance |

This ensures:
- **Query Performance**: Trace queries only hit one backend bucket
- **Load Distribution**: Different traces spread across buckets
- **Data Locality**: Related files stored together

## Configuration Examples

### Basic Configuration
```json
{
  "listen_addr": ":8080",
  "endpoint": "https://s3.amazonaws.com",
  "access_key_id": "AKIAIOSFODNN7EXAMPLE",
  "secret_access_key": "wJalrXUtnFEMI/K7MDENG/bPxRfiCYEXAMPLEKEY",
  "use_ssl": true,
  "region": "us-east-1",
  "buckets": ["tempo-shard1", "tempo-shard2", "tempo-shard3"]
}
```

### Endpoint Configuration Options
- **Full URL**: `"endpoint": "https://line-objects-internal.com"` (SSL determined by scheme)
- **Hostname only**: `"endpoint": "line-objects-internal.com"` (SSL determined by `use_ssl`)

### Testing with MinIO Client
```bash
# Setup alias for proxy
mc alias set proxy http://localhost:8080 "" ""

# Test Tempo-like paths
mc cp file.txt proxy/proxy-bucket/single-tenant/trace123/bloom-0
mc cp file.txt proxy/proxy-bucket/single-tenant/trace123/chunks-001
mc ls proxy/proxy-bucket/single-tenant/trace123/
```

## Observability & Monitoring

### Prometheus Metrics

The application exposes metrics at `/metrics` endpoint on port 8080. Key metrics include:

**HTTP Request Metrics:**
- `tempo_s3_shard_http_requests_total{method, path, status_code}` - Total HTTP requests
- `tempo_s3_shard_http_request_duration_seconds{method, path}` - Request latency

**S3 Operation Metrics:**
- `tempo_s3_shard_s3_operations_total{operation, bucket, status}` - S3 operation counters
- `tempo_s3_shard_s3_operation_duration_seconds{operation, bucket}` - S3 operation latency
- `tempo_s3_shard_object_size_bytes{operation}` - Object size distribution

**Consistent Hash & Distribution:**
- `tempo_s3_shard_hash_distribution_total{bucket}` - Object distribution across buckets
- `tempo_s3_shard_bucket_operations_total{bucket, operation}` - Per-bucket operation count

**LIST Operation Specifics:**
- `tempo_s3_shard_list_operations_total{prefix}` - LIST operations by prefix
- `tempo_s3_shard_list_objects_count{bucket}` - Objects returned per bucket in LIST

### Structured Logging

All logs use `log/slog` with logfmt format. Key log messages:

**Access Logs:**
```
level=INFO msg="HTTP request" method=GET path=/proxy-bucket/trace1/ status=200 duration_ms=12.4 remote_addr=127.0.0.1:45678
```

**S3 Operation Logs:**
```
level=ERROR msg="Error getting object" object_key=missing-file bucket=shard2 error="NoSuchKey"
level=DEBUG msg="List objects operation completed" bucket=proxy-bucket prefix=single-tenant/ object_count=42 duration_ms=156.7
```

**Startup Logs:**
```
level=INFO msg="Starting Tempo S3 Shard Server" listen_addr=:8080 endpoint=https://s3.amazonaws.com buckets=[shard1,shard2,shard3]
```

### Kubernetes Monitoring

Use the provided ServiceMonitor for Prometheus Operator:
- File: `deployments/servicemonitor.yaml`
- Scrape interval: 30 seconds
- Automatic service discovery via label selectors

## Important Implementation Notes

- Virtual bucket name is hardcoded as `"proxy-bucket"`
- Path prefix extraction uses `strings.SplitN(path, "/", 3)` to get first two segments
- Trailing slashes in URLs are automatically stripped for consistent routing
- Backend buckets are auto-created if they don't exist
- All S3 operations route through the same consistent hash for deterministic behavior
- Metrics collection has minimal performance impact (<1ms overhead per request)
- Structured logging provides detailed context for debugging and monitoring
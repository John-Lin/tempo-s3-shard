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

## Important Implementation Notes

- Virtual bucket name is hardcoded as `"proxy-bucket"`
- Path prefix extraction uses `strings.SplitN(path, "/", 3)` to get first two segments
- Trailing slashes in URLs are automatically stripped for consistent routing
- Backend buckets are auto-created if they don't exist
- All S3 operations route through the same consistent hash for deterministic behavior
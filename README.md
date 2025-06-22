# Tempo S3 Shard

A high-performance S3-compatible shard server that distributes objects across multiple S3 buckets using consistent hashing. Optimized for applications like Grafana Tempo that benefit from trace-locality and distributed storage.

## Features

- **S3 API Compatibility**: Full support for standard S3 operations
- **Smart Consistent Hashing**: Groups related objects by path prefix for optimal locality
- **Multi-bucket Support**: Aggregates objects from multiple backend buckets
- **MinIO Integration**: Uses minio-go client for robust S3 operations
- **Grafana Tempo Optimized**: Ensures trace data locality for better query performance
- **HTTPS Support**: Configurable SSL/TLS endpoints with automatic scheme detection

## Supported S3 Operations

- `ListBuckets` - Lists all buckets (returns virtual proxy bucket)
- `ListObjects` - Lists objects across all backend buckets
- `PutObject` - Stores objects using consistent hashing
- `GetObject` - Retrieves objects from correct bucket
- `DeleteObject` - Removes objects from correct bucket
- `HeadObject` - Gets object metadata
- `GetObjectTagging` - Retrieves object tags
- `PutObjectTagging` - Sets object tags

## Quick Start

### Option 1: Native Binary

1. **Build Tempo S3 Shard**:
   ```bash
   go build -o tempo-s3-shard
   ```

2. **Configure your backends** in `config.json`:
   ```json
   {
     "listen_addr": ":8080",
     "endpoint": "https://your-s3-endpoint.com",
     "access_key_id": "your-access-key",
     "secret_access_key": "your-secret-key",
     "use_ssl": true,
     "region": "us-east-1",
     "buckets": ["shard1", "shard2", "shard3"]
   }
   ```
   
   **Note**: The `endpoint` field supports both formats:
   - Full URL: `"https://s3.amazonaws.com"` (scheme determines SSL)
   - Host only: `"s3.amazonaws.com"` (uses `use_ssl` setting)

3. **Start Tempo S3 Shard**:
   ```bash
   ./tempo-s3-shard -config config.json
   ```

### Option 2: Docker (Recommended)

1. **Build Docker image**:
   ```bash
   # For x86_64/amd64 (Intel/AMD processors)
   docker build -t tempo-s3-shard .
   
   # For Apple Silicon M1/M2 (ARM64)
   docker build --platform linux/arm64 -t tempo-s3-shard .
   ```

2. **Create config.json** (same format as above)

3. **Run with Docker**:
   ```bash
   docker run -d \
     --name tempo-s3-shard \
     -p 8080:8080 \
     -v $(pwd)/config.json:/etc/tempo-s3-shard/config.json:ro \
     tempo-s3-shard
   ```

4. **Check logs**:
   ```bash
   docker logs tempo-s3-shard
   ```

## Usage with Grafana Tempo

Configure Tempo to use the Tempo S3 Shard:

```yaml
storage:
  trace:
    backend: s3
    s3:
      endpoint: localhost:8080        # Your Tempo S3 Shard address
      bucket: proxy-bucket           # Virtual bucket name (must be "proxy-bucket")
      forcepathstyle: true
      insecure: true
      access_key: ""                 # Leave empty for proxy
      secret_key: ""                 # Leave empty for proxy
```

**Why Tempo + Tempo S3 Shard is Perfect:**

Tempo's path structure: `single-tenant/{trace-id}/{file-type}`

Tempo S3 Shard groups all files for the same trace ID into the same backend bucket:
- ✅ `single-tenant/trace1/bloom-0` → bucket A
- ✅ `single-tenant/trace1/bloom-1` → bucket A  
- ✅ `single-tenant/trace1/chunks-000001` → bucket A
- ✅ `single-tenant/trace2/bloom-0` → bucket B

**Benefits:**
- **Query Performance**: All trace data in one bucket = faster queries
- **Load Distribution**: Different traces spread across buckets
- **Data Locality**: Related files stored together
- **Scalability**: Linear scaling by adding more backend buckets

## Configuration

| Field | Description | Example |
|-------|-------------|---------|
| `listen_addr` | Shard server listen address | `:8080` |
| `endpoint` | Backend S3 endpoint (URL or hostname) | `https://s3.amazonaws.com` or `s3.amazonaws.com` |
| `access_key_id` | S3 access key | `AKIAIOSFODNN7EXAMPLE` |
| `secret_access_key` | S3 secret key | `wJalrXUtnFEMI/K7MDENG/bPxRfiCYEXAMPLEKEY` |
| `use_ssl` | Enable SSL/TLS (ignored if endpoint has scheme) | `true` |
| `region` | S3 region | `us-east-1` |
| `buckets` | List of backend bucket names | `["tempo-shard1", "tempo-shard2", "tempo-shard3"]` |

## How It Works

### Smart Path-Based Hashing

Tempo S3 Shard uses **path prefix hashing** instead of full-path hashing for optimal data locality:

1. **Hash Key Extraction**: For path `single-tenant/trace-id/file-name`, only `single-tenant/trace-id` is used for hashing
2. **Object Distribution**: SHA-256 based consistent hashing on the path prefix determines the target bucket
3. **Object Retrieval**: Same prefix hashing ensures related objects are found in the same bucket
4. **List Operations**: Aggregates results from all backend buckets for unified view

### Path Examples

| Full Object Path | Hash Key Used | Result |
|------------------|---------------|---------|
| `single-tenant/trace1/bloom-0` | `single-tenant/trace1` | → bucket A |
| `single-tenant/trace1/bloom-1` | `single-tenant/trace1` | → bucket A |
| `single-tenant/trace1/chunks-001` | `single-tenant/trace1` | → bucket A |
| `single-tenant/trace2/bloom-0` | `single-tenant/trace2` | → bucket B |
| `hash/hello` | `hash/hello` | → bucket C |
| `simple-file` | `simple-file` | → bucket D |

**Consistent Hashing Features:**
- Uses virtual nodes (100 replicas per bucket) for even distribution
- Minimal redistribution when buckets are added/removed
- Deterministic routing ensures same prefix always maps to same bucket

## Architecture

```
Client (Tempo) → Tempo S3 Shard → Consistent Hash → Backend Buckets
                    ↓
               [bucket1, bucket2, bucket3]
```

- **Client**: Grafana Tempo or any S3-compatible client
- **Tempo S3 Shard**: This application providing S3 API compatibility  
- **Consistent Hash**: Algorithm determining object→bucket mapping
- **Backend Buckets**: Multiple S3 buckets for distributed storage

## Performance Considerations

- **Path-based grouping**: Related objects stored in same bucket for faster queries
- **Load balancing**: Different path prefixes distributed across buckets  
- **LIST operations**: Query all buckets concurrently for optimal performance
- **Consistent hashing**: Minimizes data movement when scaling buckets
- **Connection pooling**: Single S3 client shared across all operations
- **Tempo optimization**: Trace queries only hit one backend bucket

## Development

### Building
```bash
go mod tidy
go build -o tempo-s3-shard
```

### Testing
The proxy creates backend buckets automatically if they don't exist.

Use MinIO Client (mc) for testing:
```bash
# Configure mc to point to proxy
mc alias set proxy http://localhost:8080 "" ""

# Test operations
mc cp file.txt proxy/proxy-bucket/test/path
mc ls proxy/proxy-bucket/
mc cp proxy/proxy-bucket/test/path downloaded.txt
```

### Adding New Operations
To add new S3 operations:
1. Add handler method in `internal/server/server.go`
2. Update routing in `handleRequest()` method
3. Implement path prefix hashing logic for the operation
4. Ensure operation works with virtual bucket name "proxy-bucket"

## License

MIT License - see LICENSE file for details.
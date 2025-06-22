package metrics

import (
	"github.com/prometheus/client_golang/prometheus"
	"github.com/prometheus/client_golang/prometheus/promauto"
)

var (
	// HTTP request metrics
	HttpRequestsTotal = promauto.NewCounterVec(
		prometheus.CounterOpts{
			Name: "tempo_s3_shard_http_requests_total",
			Help: "Total number of HTTP requests",
		},
		[]string{"method", "path", "status_code"},
	)

	HttpRequestDuration = promauto.NewHistogramVec(
		prometheus.HistogramOpts{
			Name:    "tempo_s3_shard_http_request_duration_seconds",
			Help:    "HTTP request duration in seconds",
			Buckets: prometheus.DefBuckets,
		},
		[]string{"method", "path"},
	)

	// S3 operation metrics
	S3OperationsTotal = promauto.NewCounterVec(
		prometheus.CounterOpts{
			Name: "tempo_s3_shard_s3_operations_total",
			Help: "Total number of S3 operations",
		},
		[]string{"operation", "bucket", "status"},
	)

	S3OperationDuration = promauto.NewHistogramVec(
		prometheus.HistogramOpts{
			Name:    "tempo_s3_shard_s3_operation_duration_seconds",
			Help:    "S3 operation duration in seconds",
			Buckets: prometheus.DefBuckets,
		},
		[]string{"operation", "bucket"},
	)

	// Object size metrics
	ObjectSizeBytes = promauto.NewHistogramVec(
		prometheus.HistogramOpts{
			Name:    "tempo_s3_shard_object_size_bytes",
			Help:    "Size of objects processed in bytes",
			Buckets: []float64{1024, 10240, 102400, 1048576, 10485760, 104857600, 1073741824}, // 1KB to 1GB
		},
		[]string{"operation"},
	)

	// Consistent hash metrics
	HashDistribution = promauto.NewCounterVec(
		prometheus.CounterOpts{
			Name: "tempo_s3_shard_hash_distribution_total",
			Help: "Distribution of objects across buckets based on consistent hash",
		},
		[]string{"bucket"},
	)

	// Active connections
	ActiveConnections = promauto.NewGauge(
		prometheus.GaugeOpts{
			Name: "tempo_s3_shard_active_connections",
			Help: "Number of active connections",
		},
	)

	// Bucket operations
	BucketOperationsTotal = promauto.NewCounterVec(
		prometheus.CounterOpts{
			Name: "tempo_s3_shard_bucket_operations_total",
			Help: "Total number of operations per bucket",
		},
		[]string{"bucket", "operation"},
	)

	// List operations specific metrics (expensive operations)
	ListOperationsTotal = promauto.NewCounterVec(
		prometheus.CounterOpts{
			Name: "tempo_s3_shard_list_operations_total",
			Help: "Total number of list operations across all buckets",
		},
		[]string{"prefix"},
	)

	ListObjectsCount = promauto.NewHistogramVec(
		prometheus.HistogramOpts{
			Name:    "tempo_s3_shard_list_objects_count",
			Help:    "Number of objects returned in list operations",
			Buckets: []float64{1, 10, 50, 100, 500, 1000, 5000, 10000},
		},
		[]string{"bucket"},
	)
)
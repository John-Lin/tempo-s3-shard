package client

import (
	"context"
	"fmt"

	"github.com/minio/minio-go/v7"
	"github.com/minio/minio-go/v7/pkg/credentials"
	"tempo-s3-shard/internal/config"
	"tempo-s3-shard/internal/hash"
)

type S3ClientManager struct {
	client *minio.Client
	hasher *hash.ConsistentHash
	config *config.Config
}

func NewS3ClientManager(cfg *config.Config) (*S3ClientManager, error) {
	host, useSSL, err := cfg.ParsedEndpoint()
	if err != nil {
		return nil, fmt.Errorf("failed to parse endpoint: %w", err)
	}
	
	client, err := minio.New(host, &minio.Options{
		Creds:  credentials.NewStaticV4(cfg.AccessKeyID, cfg.SecretAccessKey, ""),
		Secure: useSSL,
		Region: cfg.Region,
	})
	if err != nil {
		return nil, fmt.Errorf("failed to create minio client: %w", err)
	}

	hasher := hash.NewConsistentHash(100, cfg.Buckets)

	return &S3ClientManager{
		client: client,
		hasher: hasher,
		config: cfg,
	}, nil
}

func (s *S3ClientManager) GetBucketForKey(key string) string {
	return s.hasher.GetBucket(key)
}

func (s *S3ClientManager) GetAllBuckets() []string {
	return s.hasher.GetAllBuckets()
}

func (s *S3ClientManager) GetClient() *minio.Client {
	return s.client
}

func (s *S3ClientManager) EnsureBucketsExist(ctx context.Context) error {
	for _, bucketName := range s.config.Buckets {
		exists, err := s.client.BucketExists(ctx, bucketName)
		if err != nil {
			return fmt.Errorf("failed to check bucket %s: %w", bucketName, err)
		}
		if !exists {
			err = s.client.MakeBucket(ctx, bucketName, minio.MakeBucketOptions{
				Region: s.config.Region,
			})
			if err != nil {
				return fmt.Errorf("failed to create bucket %s: %w", bucketName, err)
			}
		}
	}
	return nil
}
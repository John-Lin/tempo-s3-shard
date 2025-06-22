package server

import (
	"context"
	"io"
	"log"
	"net/http"
	"net/url"
	"strconv"
	"strings"
	"time"

	"github.com/minio/minio-go/v7"
	"github.com/minio/minio-go/v7/pkg/tags"
	"tempo-s3-shard/internal/client"
	"tempo-s3-shard/internal/config"
)

type TempoS3ShardServer struct {
	mux           *http.ServeMux
	clientManager *client.S3ClientManager
	config        *config.Config
}

func NewTempoS3ShardServer(cfg *config.Config) (*TempoS3ShardServer, error) {
	clientManager, err := client.NewS3ClientManager(cfg)
	if err != nil {
		return nil, err
	}

	ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
	defer cancel()
	
	if err := clientManager.EnsureBucketsExist(ctx); err != nil {
		log.Printf("Warning: failed to ensure buckets exist: %v", err)
	}

	s := &TempoS3ShardServer{
		mux:           http.NewServeMux(),
		clientManager: clientManager,
		config:        cfg,
	}
	s.setupRoutes()
	return s, nil
}

func (s *TempoS3ShardServer) ServeHTTP(w http.ResponseWriter, r *http.Request) {
	log.Printf("%s %s", r.Method, r.URL.Path)
	s.mux.ServeHTTP(w, r)
}

func (s *TempoS3ShardServer) setupRoutes() {
	s.mux.HandleFunc("/", s.handleRequest)
}

func (s *TempoS3ShardServer) handleRequest(w http.ResponseWriter, r *http.Request) {
	path := strings.TrimPrefix(r.URL.Path, "/")
	path = strings.TrimSuffix(path, "/") // Remove trailing slash
	pathParts := strings.Split(path, "/")
	
	if path == "" {
		pathParts = []string{}
	}
	
	switch r.Method {
	case "GET":
		if len(pathParts) == 0 || pathParts[0] == "" {
			s.handleListBuckets(w, r)
		} else if len(pathParts) == 1 {
			// Check if this is a bucket existence check (with location query param)
			_, hasLocation := r.URL.Query()["location"]
			if hasLocation {
				s.handleGetBucketLocation(w, r, pathParts[0])
			} else {
				s.handleListObjects(w, r, pathParts[0])
			}
		} else if len(pathParts) >= 2 {
			objectKey := strings.Join(pathParts[1:], "/")
			if r.URL.Query().Get("tagging") != "" {
				s.handleGetObjectTagging(w, r, pathParts[0], objectKey)
			} else {
				s.handleGetObject(w, r, pathParts[0], objectKey)
			}
		}
	case "PUT":
		if len(pathParts) >= 2 {
			objectKey := strings.Join(pathParts[1:], "/")
			if r.URL.Query().Get("tagging") != "" {
				s.handlePutObjectTagging(w, r, pathParts[0], objectKey)
			} else {
				s.handlePutObject(w, r, pathParts[0], objectKey)
			}
		}
	case "DELETE":
		if len(pathParts) >= 2 {
			objectKey := strings.Join(pathParts[1:], "/")
			s.handleDeleteObject(w, r, pathParts[0], objectKey)
		}
	case "HEAD":
		if len(pathParts) >= 2 {
			objectKey := strings.Join(pathParts[1:], "/")
			s.handleHeadObject(w, r, pathParts[0], objectKey)
		}
	default:
		http.Error(w, "Method not allowed", http.StatusMethodNotAllowed)
	}
}

func (s *TempoS3ShardServer) handleListBuckets(w http.ResponseWriter, r *http.Request) {
	w.Header().Set("Content-Type", "application/xml")
	w.WriteHeader(http.StatusOK)
	
	xml := `<?xml version="1.0" encoding="UTF-8"?>
<ListAllMyBucketsResult xmlns="http://s3.amazonaws.com/doc/2006-03-01/">
  <Owner>
    <ID>tempo-shard-owner</ID>
    <DisplayName>Tempo S3 Shard</DisplayName>
  </Owner>
  <Buckets>
    <Bucket>
      <Name>proxy-bucket</Name>
      <CreationDate>2024-01-01T00:00:00.000Z</CreationDate>
    </Bucket>
  </Buckets>
</ListAllMyBucketsResult>`
	
	w.Write([]byte(xml))
}

func (s *TempoS3ShardServer) handleGetBucketLocation(w http.ResponseWriter, r *http.Request, bucketName string) {
	// Only accept the virtual bucket name
	if bucketName != "proxy-bucket" {
		http.Error(w, "Bucket not found", http.StatusNotFound)
		return
	}
	
	w.Header().Set("Content-Type", "application/xml")
	w.WriteHeader(http.StatusOK)
	
	xml := `<?xml version="1.0" encoding="UTF-8"?>
<LocationConstraint xmlns="http://s3.amazonaws.com/doc/2006-03-01/">us-east-1</LocationConstraint>`
	
	w.Write([]byte(xml))
}

func (s *TempoS3ShardServer) handleListObjects(w http.ResponseWriter, r *http.Request, bucketName string) {
	ctx := context.Background()
	prefix := r.URL.Query().Get("prefix")
	delimiter := r.URL.Query().Get("delimiter")
	maxKeysStr := r.URL.Query().Get("max-keys")
	marker := r.URL.Query().Get("marker")
	
	maxKeys := 1000
	if maxKeysStr != "" {
		if mk, err := strconv.Atoi(maxKeysStr); err == nil && mk > 0 {
			maxKeys = mk
		}
	}

	allObjects := []minio.ObjectInfo{}
	allPrefixes := []string{}
	
	for _, realBucket := range s.clientManager.GetAllBuckets() {
		objCh := s.clientManager.GetClient().ListObjects(ctx, realBucket, minio.ListObjectsOptions{
			Prefix:    prefix,
			Recursive: delimiter == "",
		})
		
		for object := range objCh {
			if object.Err != nil {
				log.Printf("Error listing objects in bucket %s: %v", realBucket, object.Err)
				continue
			}
			allObjects = append(allObjects, object)
		}
	}

	w.Header().Set("Content-Type", "application/xml")
	w.WriteHeader(http.StatusOK)
	
	xml := `<?xml version="1.0" encoding="UTF-8"?>
<ListBucketResult xmlns="http://s3.amazonaws.com/doc/2006-03-01/">
  <Name>` + bucketName + `</Name>
  <Prefix>` + prefix + `</Prefix>
  <Marker>` + marker + `</Marker>
  <MaxKeys>` + strconv.Itoa(maxKeys) + `</MaxKeys>
  <IsTruncated>false</IsTruncated>`
	
	for _, obj := range allObjects {
		xml += `
  <Contents>
    <Key>` + obj.Key + `</Key>
    <LastModified>` + obj.LastModified.Format(time.RFC3339) + `</LastModified>
    <ETag>"` + obj.ETag + `"</ETag>
    <Size>` + strconv.FormatInt(obj.Size, 10) + `</Size>
    <StorageClass>STANDARD</StorageClass>
  </Contents>`
	}
	
	for _, prefix := range allPrefixes {
		xml += `
  <CommonPrefixes>
    <Prefix>` + prefix + `</Prefix>
  </CommonPrefixes>`
	}
	
	xml += `
</ListBucketResult>`
	
	w.Write([]byte(xml))
}

func (s *TempoS3ShardServer) handlePutObject(w http.ResponseWriter, r *http.Request, bucketName, objectKey string) {
	ctx := context.Background()
	targetBucket := s.clientManager.GetBucketForKey(objectKey)
	
	contentLength := r.ContentLength
	if contentLength < 0 {
		http.Error(w, "Content-Length required", http.StatusBadRequest)
		return
	}
	
	contentType := r.Header.Get("Content-Type")
	if contentType == "" {
		contentType = "application/octet-stream"
	}
	
	info, err := s.clientManager.GetClient().PutObject(ctx, targetBucket, objectKey, r.Body, contentLength, minio.PutObjectOptions{
		ContentType: contentType,
	})
	if err != nil {
		log.Printf("Error putting object %s to bucket %s: %v", objectKey, targetBucket, err)
		http.Error(w, "Internal Server Error", http.StatusInternalServerError)
		return
	}
	
	w.Header().Set("ETag", `"`+info.ETag+`"`)
	w.WriteHeader(http.StatusOK)
}

func (s *TempoS3ShardServer) handleGetObject(w http.ResponseWriter, r *http.Request, bucketName, objectKey string) {
	ctx := context.Background()
	targetBucket := s.clientManager.GetBucketForKey(objectKey)
	
	object, err := s.clientManager.GetClient().GetObject(ctx, targetBucket, objectKey, minio.GetObjectOptions{})
	if err != nil {
		log.Printf("Error getting object %s from bucket %s: %v", objectKey, targetBucket, err)
		http.Error(w, "Object not found", http.StatusNotFound)
		return
	}
	defer object.Close()
	
	info, err := object.Stat()
	if err != nil {
		log.Printf("Error getting object stat %s from bucket %s: %v", objectKey, targetBucket, err)
		http.Error(w, "Object not found", http.StatusNotFound)
		return
	}
	
	w.Header().Set("Content-Type", info.ContentType)
	w.Header().Set("Content-Length", strconv.FormatInt(info.Size, 10))
	w.Header().Set("ETag", `"`+info.ETag+`"`)
	w.Header().Set("Last-Modified", info.LastModified.Format(http.TimeFormat))
	
	w.WriteHeader(http.StatusOK)
	io.Copy(w, object)
}

func (s *TempoS3ShardServer) handleDeleteObject(w http.ResponseWriter, r *http.Request, bucketName, objectKey string) {
	ctx := context.Background()
	targetBucket := s.clientManager.GetBucketForKey(objectKey)
	
	err := s.clientManager.GetClient().RemoveObject(ctx, targetBucket, objectKey, minio.RemoveObjectOptions{})
	if err != nil {
		log.Printf("Error deleting object %s from bucket %s: %v", objectKey, targetBucket, err)
		http.Error(w, "Internal Server Error", http.StatusInternalServerError)
		return
	}
	
	w.WriteHeader(http.StatusNoContent)
}

func (s *TempoS3ShardServer) handleHeadObject(w http.ResponseWriter, r *http.Request, bucketName, objectKey string) {
	ctx := context.Background()
	targetBucket := s.clientManager.GetBucketForKey(objectKey)
	
	info, err := s.clientManager.GetClient().StatObject(ctx, targetBucket, objectKey, minio.StatObjectOptions{})
	if err != nil {
		log.Printf("Error getting object stat %s from bucket %s: %v", objectKey, targetBucket, err)
		http.Error(w, "Object not found", http.StatusNotFound)
		return
	}
	
	w.Header().Set("Content-Type", info.ContentType)
	w.Header().Set("Content-Length", strconv.FormatInt(info.Size, 10))
	w.Header().Set("ETag", `"`+info.ETag+`"`)
	w.Header().Set("Last-Modified", info.LastModified.Format(http.TimeFormat))
	
	w.WriteHeader(http.StatusOK)
}

func (s *TempoS3ShardServer) handleGetObjectTagging(w http.ResponseWriter, r *http.Request, bucketName, objectKey string) {
	ctx := context.Background()
	targetBucket := s.clientManager.GetBucketForKey(objectKey)
	
	tags, err := s.clientManager.GetClient().GetObjectTagging(ctx, targetBucket, objectKey, minio.GetObjectTaggingOptions{})
	if err != nil {
		log.Printf("Error getting object tags %s from bucket %s: %v", objectKey, targetBucket, err)
		http.Error(w, "Object not found", http.StatusNotFound)
		return
	}
	
	w.Header().Set("Content-Type", "application/xml")
	w.WriteHeader(http.StatusOK)
	
	xml := `<?xml version="1.0" encoding="UTF-8"?>
<Tagging xmlns="http://s3.amazonaws.com/doc/2006-03-01/">
  <TagSet>`
	
	for key, value := range tags.ToMap() {
		xml += `
    <Tag>
      <Key>` + key + `</Key>
      <Value>` + value + `</Value>
    </Tag>`
	}
	
	xml += `
  </TagSet>
</Tagging>`
	
	w.Write([]byte(xml))
}

func (s *TempoS3ShardServer) handlePutObjectTagging(w http.ResponseWriter, r *http.Request, bucketName, objectKey string) {
	ctx := context.Background()
	targetBucket := s.clientManager.GetBucketForKey(objectKey)
	
	body, err := io.ReadAll(r.Body)
	if err != nil {
		http.Error(w, "Bad Request", http.StatusBadRequest)
		return
	}
	
	queryTags, err := url.ParseQuery(string(body))
	if err != nil {
		http.Error(w, "Bad Request", http.StatusBadRequest)
		return
	}
	
	tagMap := make(map[string]string)
	for k, v := range queryTags {
		if len(v) > 0 {
			tagMap[k] = v[0]
		}
	}
	
	objectTags, err := tags.NewTags(tagMap, true)
	if err != nil {
		http.Error(w, "Bad Request", http.StatusBadRequest)
		return
	}
	
	err = s.clientManager.GetClient().PutObjectTagging(ctx, targetBucket, objectKey, objectTags, minio.PutObjectTaggingOptions{})
	if err != nil {
		log.Printf("Error putting object tags %s to bucket %s: %v", objectKey, targetBucket, err)
		http.Error(w, "Internal Server Error", http.StatusInternalServerError)
		return
	}
	
	w.WriteHeader(http.StatusOK)
}
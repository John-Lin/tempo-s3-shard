package hash

import (
	"crypto/sha256"
	"sort"
	"strconv"
	"strings"
)

type ConsistentHash struct {
	replicas int
	keys     []uint32
	hashMap  map[uint32]string
	buckets  []string
}

func NewConsistentHash(replicas int, buckets []string) *ConsistentHash {
	ch := &ConsistentHash{
		replicas: replicas,
		hashMap:  make(map[uint32]string),
		buckets:  buckets,
	}
	
	for _, bucket := range buckets {
		ch.addBucket(bucket)
	}
	
	return ch
}

func (ch *ConsistentHash) addBucket(bucket string) {
	for i := 0; i < ch.replicas; i++ {
		key := ch.hash(bucket + strconv.Itoa(i))
		ch.keys = append(ch.keys, key)
		ch.hashMap[key] = bucket
	}
	sort.Slice(ch.keys, func(i, j int) bool {
		return ch.keys[i] < ch.keys[j]
	})
}

func (ch *ConsistentHash) hash(key string) uint32 {
	h := sha256.Sum256([]byte(key))
	return uint32(h[0])<<24 | uint32(h[1])<<16 | uint32(h[2])<<8 | uint32(h[3])
}

// extractHashKey extracts the first two path segments for consistent hashing
// e.g., "single-tenant/0003b3c9-8689-41a6-835c-1374ce2d5879/bloom-0" -> "single-tenant/0003b3c9-8689-41a6-835c-1374ce2d5879"
func (ch *ConsistentHash) extractHashKey(key string) string {
	parts := strings.SplitN(key, "/", 3)
	if len(parts) >= 2 {
		return parts[0] + "/" + parts[1]
	}
	return key // fallback to original key if less than 2 segments
}

func (ch *ConsistentHash) GetBucket(key string) string {
	if len(ch.keys) == 0 {
		return ""
	}
	
	hashKey := ch.extractHashKey(key)
	hash := ch.hash(hashKey)
	
	idx := sort.Search(len(ch.keys), func(i int) bool {
		return ch.keys[i] >= hash
	})
	
	if idx == len(ch.keys) {
		idx = 0
	}
	
	return ch.hashMap[ch.keys[idx]]
}

func (ch *ConsistentHash) GetAllBuckets() []string {
	return ch.buckets
}
package github

import (
	"context"
	"crypto/sha1"
	"encoding/hex"
	"errors"
	"time"
)

var errCacheMiss = errors.New("cache miss")

type Cache interface {
	Get(ctx context.Context, key string) (string, error)
	Set(ctx context.Context, key, value string, ttl time.Duration) error
}

func cacheKey(path string) string {
	sum := sha1.Sum([]byte(path))
	return "github:" + hex.EncodeToString(sum[:])
}

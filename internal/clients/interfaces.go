package clients

import (
	"context"
	"io"
	"net/http"
	"time"

	"github.com/go-redis/redis/v8"
)

type RedisClient interface {
	IncrByFloat(ctx context.Context, key string, value float64) *redis.FloatCmd
	Scan(ctx context.Context, cursor uint64, match string, count int64) *redis.ScanCmd
	Get(ctx context.Context, key string) *redis.StringCmd
	Set(ctx context.Context, key string, value interface{}, expiration time.Duration) *redis.StatusCmd
	Del(ctx context.Context, keys ...string) *redis.IntCmd
}

type HTTPClient interface {
	Post(url, contentType string, body io.Reader) (*http.Response, error)
}

package clients

import (
	"context"
	"time"

	"github.com/go-redis/redis/v8"
	"github.com/rs/zerolog/log"
)

type RedisClient interface {
	IncrByFloat(ctx context.Context, key string, value float64) *redis.FloatCmd
	Scan(ctx context.Context, cursor uint64, match string, count int64) *redis.ScanCmd
	Get(ctx context.Context, key string) *redis.StringCmd
	Set(ctx context.Context, key string, value interface{}, expiration time.Duration) *redis.StatusCmd
	Del(ctx context.Context, keys ...string) *redis.IntCmd
	Ping(ctx context.Context) *redis.StatusCmd
	PoolStats() *redis.PoolStats
	Close() error
}

type redisClient struct {
	client RedisClient
}

func (r *redisClient) IncrByFloat(ctx context.Context, key string, value float64) *redis.FloatCmd {
	return r.client.IncrByFloat(ctx, key, value)
}

func (r *redisClient) Scan(ctx context.Context, cursor uint64, match string, count int64) *redis.ScanCmd {
	return r.client.Scan(ctx, cursor, match, count)
}

func (r *redisClient) Get(ctx context.Context, key string) *redis.StringCmd {
	return r.client.Get(ctx, key)
}

func (r *redisClient) Set(ctx context.Context, key string, value interface{}, expiration time.Duration) *redis.StatusCmd {
	return r.client.Set(ctx, key, value, expiration)
}

func (r *redisClient) Del(ctx context.Context, keys ...string) *redis.IntCmd {
	return r.client.Del(ctx, keys...)
}

func (r *redisClient) Ping(ctx context.Context) *redis.StatusCmd {
	return r.client.Ping(ctx)
}

func (r *redisClient) Close() error {
	return r.client.Close()
}

func (r *redisClient) PoolStats() *redis.PoolStats {
	return r.client.PoolStats()
}

type RedisClientOptions func(*redisClient)

// WithCustomRedisClient permite passar um cliente Redis customizado
func WithCustomRedisClient(client RedisClient) RedisClientOptions {
	return func(r *redisClient) {
		r.client = client
	}
}
// Cria um cliente Redis com as opções padrão
// DialTimeout: 5s, ReadTimeout: 3s, WriteTimeout: 3s, PoolSize: 100, MinIdleConns: 10, MaxRetries: 3
func createRedisClient(host, port string, opts ...RedisClientOptions) RedisClient {
	clientCreated := &redisClient{
		client: redis.NewClient(&redis.Options{
			Addr:         host + ":" + port,
			DialTimeout:  5 * time.Second,
			ReadTimeout:  3 * time.Second,
			WriteTimeout: 3 * time.Second,
			PoolSize:     100,
			MinIdleConns: 10,
			MaxRetries:   3,
		}),
	}

	for _, opt := range opts {
		opt(clientCreated)
	}
	return clientCreated.client
}
// Cria um cliente Redis com as opções padrão e verifica a conexão
func InitRedisClient(host, port string, opts ...RedisClientOptions) RedisClient {
	client := createRedisClient(host, port, opts...)

	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()
	_, err := client.Ping(ctx).Result()
	if err != nil {
		log.Panic().Msgf("Erro ao conectar ao Redis: %v\n", err)
	}

	log.Debug().Msg("Conexão com Redis estabelecida com sucesso!\n")
	log.Debug().Msgf("RedisClient PoolStats: %v\n", client.PoolStats())
	return client
}

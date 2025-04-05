package clients

import (
	"context"
	"fmt"
	"os"
	"time"

	"github.com/go-redis/redis/v8"
)

type RedisClient interface {
	IncrByFloat(ctx context.Context, key string, value float64) *redis.FloatCmd
	Scan(ctx context.Context, cursor uint64, match string, count int64) *redis.ScanCmd
	Get(ctx context.Context, key string) *redis.StringCmd
	Set(ctx context.Context, key string, value interface{}, expiration time.Duration) *redis.StatusCmd
	Del(ctx context.Context, keys ...string) *redis.IntCmd
	Close() error
}

func InitRedisClient(host, port string) RedisClient {
	client := redis.NewClient(&redis.Options{
		Addr:         host + ":" + port,
		DialTimeout:  5 * time.Second,
		ReadTimeout:  3 * time.Second,
		WriteTimeout: 3 * time.Second,
		PoolSize:     100,
		MinIdleConns: 10,
		MaxRetries:   3,
	})

	// Verificar se o Redis está disponível
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()
	_, err := client.Ping(ctx).Result()
	if err != nil {
		fmt.Printf("Erro ao conectar ao Redis: %v\n", err)
		os.Exit(1)
	}

	fmt.Printf("Conexão com Redis estabelecida com sucesso!\n")
	fmt.Printf("RedisClient PoolStats: %v\n", client.PoolStats())
	return client
}

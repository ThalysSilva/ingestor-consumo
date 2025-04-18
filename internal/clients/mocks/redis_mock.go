package mocks

import (
	"context"
	"time"
	"github.com/go-redis/redis/v8"
	"github.com/stretchr/testify/mock"
)

type MockRedisClient struct {
	mock.Mock
}
func (m *MockRedisClient) IncrByFloat(ctx context.Context, key string, value float64) *redis.FloatCmd {
	args := m.Called(ctx, key, value)
	cmd := redis.NewFloatCmd(ctx, key, value)
	if err := args.Error(0); err != nil {
		cmd.SetErr(err)
	} else {
		cmd.SetVal(value)
	}
	return cmd
}

func (m *MockRedisClient) Scan(ctx context.Context, cursor uint64, match string, count int64) *redis.ScanCmd {
	args := m.Called(ctx, cursor, match, count)
	scanCmd := redis.NewScanCmd(ctx, nil, "SCAN", cursor, "MATCH", match, "COUNT", count)
	if keys, ok := args.Get(0).([]string); ok {
		var nextCursor uint64
		if cursorVal, ok := args.Get(1).(uint64); ok {
			nextCursor = cursorVal
		}
		scanCmd.SetVal(keys, nextCursor)
	}
	if err := args.Error(2); err != nil {
		scanCmd.SetErr(err)
	}
	return scanCmd
}

func (m *MockRedisClient) Get(ctx context.Context, key string) *redis.StringCmd {
	args := m.Called(ctx, key)
	cmd := redis.NewStringCmd(ctx, key)
	if val, ok := args.Get(0).(string); ok {
		cmd.SetVal(val)
	}
	if err := args.Error(1); err != nil {
		cmd.SetErr(err)
	}
	return cmd
}

func (m *MockRedisClient) Set(ctx context.Context, key string, value interface{}, expiration time.Duration) *redis.StatusCmd {
	args := m.Called(ctx, key, value, expiration)
	cmd := redis.NewStatusCmd(ctx, key, value, expiration)
	if err := args.Error(0); err != nil {
		cmd.SetErr(err)
	} else {
		cmd.SetVal("OK")
	}
	return cmd
}

func (m *MockRedisClient) Del(ctx context.Context, keys ...string) *redis.IntCmd {
	args := m.Called(ctx, keys)
	cmd := redis.NewIntCmd(ctx, keys)
	if err := args.Error(0); err != nil {
		cmd.SetErr(err)
	} else {
		cmd.SetVal(int64(len(keys)))
	}
	return cmd
}

func (m *MockRedisClient) Close() error {
	args := m.Called()
	return args.Error(0)
}

func (m *MockRedisClient) Ping(ctx context.Context) *redis.StatusCmd {
	args := m.Called(ctx)
	cmd := redis.NewStatusCmd(ctx)
	if err := args.Error(0); err != nil {
		cmd.SetErr(err)
	} else {
		cmd.SetVal("PONG")
	}
	return cmd
}

func (m *MockRedisClient) PoolStats() *redis.PoolStats {
	args := m.Called()
	return args.Get(0).(*redis.PoolStats)
}
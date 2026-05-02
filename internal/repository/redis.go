package repository

import (
	"context"
	"fmt"
	"time"

	"github.com/redis/go-redis/v9"
)

type RedisClient interface {
	IncrWithTTL(ctx context.Context, key string, ttl time.Duration) (int64, error)
	Set(ctx context.Context, key string, value any, ttl time.Duration) error
	Get(ctx context.Context, key string) (string, error)
}

type RedisStore struct {
	client *redis.Client
}

func NewRedisStore(addr, password string, db int) *RedisStore {
	return &RedisStore{client: redis.NewClient(&redis.Options{Addr: addr, Password: password, DB: db})}
}

func (r *RedisStore) Ping(ctx context.Context) error {
	return r.client.Ping(ctx).Err()
}

func (r *RedisStore) IncrWithTTL(ctx context.Context, key string, ttl time.Duration) (int64, error) {
	n, err := r.client.Incr(ctx, key).Result()
	if err != nil {
		return 0, err
	}
	if n == 1 {
		if err := r.client.Expire(ctx, key, ttl).Err(); err != nil {
			return 0, err
		}
	}
	return n, nil
}

func (r *RedisStore) Set(ctx context.Context, key string, value any, ttl time.Duration) error {
	return r.client.Set(ctx, key, value, ttl).Err()
}

func (r *RedisStore) Get(ctx context.Context, key string) (string, error) {
	return r.client.Get(ctx, key).Result()
}

func RateLimitKey(userID, toolName string) string {
	if userID == "" {
		userID = "guest"
	}
	return fmt.Sprintf("agent:ratelimit:user:%s:tool:%s", userID, toolName)
}

func SessionKey(sessionID string) string { return "agent:session:" + sessionID + ":context" }
func TaskKey(taskID string) string       { return "agent:task:" + taskID + ":status" }

package cache

import (
	"context"
	"fmt"
	"sync"
	"time"

	"github.com/go-redis/redis/v8"
	"github.com/xIceArcher/go-leah/config"
)

var (
	ErrNotFound error = fmt.Errorf("not found")
)

type Cache interface {
	Set(ctx context.Context, key string, val interface{}) error
	SetWithExpiry(ctx context.Context, key string, val interface{}, expiration time.Duration) error
	SetKeepTTL(ctx context.Context, key string, val interface{}) error

	Get(ctx context.Context, key string) (interface{}, error)
	GetByPrefix(ctx context.Context, prefix string) (map[string]interface{}, error)

	GetWithTTL(ctx context.Context, key string) (*ValueWithTTL, error)
	GetByPrefixWithTTL(ctx context.Context, prefix string) (map[string]*ValueWithTTL, error)

	Clear(ctx context.Context, key ...string) error
}

type ValueWithTTL struct {
	Value interface{}
	TTL   time.Duration
}

var (
	redisCache          *redis.Client
	redisCacheSetupOnce sync.Once
)

type RedisCache struct{}

func NewRedisCache(cfg *config.RedisConfig) (Cache, error) {
	redisCacheSetupOnce.Do(func() {
		redisCache = redis.NewClient(&redis.Options{
			Addr:     fmt.Sprintf("%s:%v", cfg.Host, cfg.Port),
			Password: cfg.Password,
			DB:       cfg.DB,
		})
	})

	return &RedisCache{}, nil
}

func (RedisCache) Set(ctx context.Context, key string, val interface{}) error {
	return redisCache.Set(ctx, key, val, 0).Err()
}

func (RedisCache) SetWithExpiry(ctx context.Context, key string, val interface{}, expiration time.Duration) error {
	return redisCache.Set(ctx, key, val, expiration).Err()
}

func (RedisCache) SetKeepTTL(ctx context.Context, key string, val interface{}) error {
	return redisCache.Set(ctx, key, val, redis.KeepTTL).Err()
}

func (RedisCache) Get(ctx context.Context, key string) (interface{}, error) {
	results, err := redisCache.MGet(ctx, key).Result()
	if err != nil {
		return nil, err
	}

	if len(results) == 0 || results[0] == nil {
		return nil, ErrNotFound
	}

	return results[0], nil
}

func (c *RedisCache) GetWithTTL(ctx context.Context, key string) (*ValueWithTTL, error) {
	value, err := c.Get(ctx, key)
	if err != nil {
		return nil, err
	}

	ttl, err := redisCache.TTL(ctx, key).Result()
	if err != nil {
		return nil, err
	}

	return &ValueWithTTL{
		Value: value,
		TTL:   ttl,
	}, nil
}

func (RedisCache) Clear(ctx context.Context, key ...string) error {
	return redisCache.Del(ctx, key...).Err()
}

func (RedisCache) GetByPrefix(ctx context.Context, prefix string) (ret map[string]interface{}, err error) {
	const prefixFormat = "%s*"
	var cursor uint64

	keysMap := make(map[string]struct{})
	for {
		var keys []string
		keys, cursor, err = redisCache.Scan(ctx, cursor, fmt.Sprintf(prefixFormat, prefix), 0).Result()
		if err != nil {
			return nil, err
		}

		for _, key := range keys {
			keysMap[key] = struct{}{}
		}

		if cursor == 0 {
			break
		}
	}

	allKeys := make([]string, 0, len(keysMap))
	for key := range keysMap {
		allKeys = append(allKeys, key)
	}

	ret = make(map[string]interface{})
	if len(allKeys) == 0 {
		return ret, nil
	}

	values, err := redisCache.MGet(ctx, allKeys...).Result()
	if err != nil {
		return nil, err
	}

	for i := 0; i < len(values); i++ {
		ret[allKeys[i]] = values[i]
	}

	return ret, nil
}

func (c *RedisCache) GetByPrefixWithTTL(ctx context.Context, prefix string) (map[string]*ValueWithTTL, error) {
	mp, err := c.GetByPrefix(ctx, prefix)
	if err != nil {
		return nil, err
	}

	ret := make(map[string]*ValueWithTTL, len(mp))
	for key, val := range mp {
		ttl, err := redisCache.TTL(ctx, key).Result()
		if err != nil {
			return nil, err
		}

		ret[key] = &ValueWithTTL{
			Value: val,
			TTL:   ttl,
		}
	}

	return ret, nil
}

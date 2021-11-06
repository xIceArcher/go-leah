package cache

import (
	"context"
	"fmt"
	"sync"

	"github.com/go-redis/redis/v8"
	"github.com/xIceArcher/go-leah/config"
)

var (
	redisCache          *redis.Client
	redisCacheSetupOnce sync.Once
)

type Cache struct{}

func NewCache(cfg *config.RedisConfig) (*Cache, error) {
	redisCacheSetupOnce.Do(func() {
		redisCache = redis.NewClient(&redis.Options{
			Addr:     fmt.Sprintf("%s:%v", cfg.Host, cfg.Port),
			Password: cfg.Password,
			DB:       cfg.DB,
		})
	})

	return &Cache{}, nil
}

func (Cache) Set(ctx context.Context, key string, val interface{}) error {
	return redisCache.Set(ctx, key, val, 0).Err()
}

func (Cache) Clear(ctx context.Context, key ...string) error {
	return redisCache.Del(ctx, key...).Err()
}

func (Cache) GetByPrefix(ctx context.Context, prefix string) (ret map[string]interface{}, err error) {
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

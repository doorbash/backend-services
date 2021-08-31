package redis

import (
	"context"
	"log"
	"time"

	"github.com/doorbash/backend-services/api/domain"
	"github.com/go-redis/redis/v8"
)

type RemoteConfigRedisCache struct {
	rdb        *redis.Client
	dataExpiry time.Duration
}

func (c *RemoteConfigRedisCache) GetDataByProjectID(ctx context.Context, id string) (string, error) {
	return c.rdb.GetEx(ctx, id, c.dataExpiry).Result()
}

func (c *RemoteConfigRedisCache) Update(ctx context.Context, rc *domain.RemoteConfig) error {
	return c.rdb.Set(ctx, rc.ProjectID, rc.Data, c.dataExpiry).Err()
}

func NewRemoteConfigRedisCache(dataExpiry time.Duration) *RemoteConfigRedisCache {
	return &RemoteConfigRedisCache{
		rdb: redis.NewClient(&redis.Options{
			Addr:            REDIS_ADDR,
			Password:        "",
			DB:              REDIS_DATABASE_AUTH,
			MaxRetries:      0,
			MinRetryBackoff: REDIS_MIN_RETRY_BACKOFF,
			MaxRetryBackoff: REDIS_MAX_RETRY_BACKOFF,
			OnConnect: func(ctx context.Context, cn *redis.Conn) error {
				log.Println("redis:", "OnConnect()", "RemoteConfig")
				return nil
			},
		}),
		dataExpiry: dataExpiry,
	}
}

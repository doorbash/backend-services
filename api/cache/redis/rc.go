package redis

import (
	"context"
	"time"

	"github.com/doorbash/backend-services-api/api/domain"
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

func NewRemoteConfigRedisCache(rdb *redis.Client, dataExpiry time.Duration) *RemoteConfigRedisCache {
	return &RemoteConfigRedisCache{
		rdb:        rdb,
		dataExpiry: dataExpiry,
	}
}

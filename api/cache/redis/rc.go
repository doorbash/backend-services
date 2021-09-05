package redis

import (
	"context"
	"fmt"
	"log"
	"strconv"
	"time"

	"github.com/doorbash/backend-services/api/domain"
	"github.com/go-redis/redis/v8"
)

type RemoteConfigRedisCache struct {
	rdb        *redis.Client
	dataExpiry time.Duration
}

func (c *RemoteConfigRedisCache) GetVersionByProjectID(ctx context.Context, pid string) (*int, error) {
	v, err := c.rdb.GetEx(ctx, fmt.Sprintf("%s.v", pid), c.dataExpiry).Result()
	if err != nil {
		return nil, err
	}
	version, err := strconv.Atoi(v)
	if err != nil {
		return nil, err
	}
	return &version, nil
}

func (c *RemoteConfigRedisCache) GetDataByProjectID(ctx context.Context, pid string) (*string, error) {
	data, err := c.rdb.GetEx(ctx, fmt.Sprintf("%s.d", pid), c.dataExpiry).Result()
	if err != nil {
		return nil, err
	}
	return &data, nil
}

func (c *RemoteConfigRedisCache) Update(ctx context.Context, rc *domain.RemoteConfig) error {
	_, err := c.rdb.TxPipelined(ctx, func(pipe redis.Pipeliner) error {
		err := pipe.Set(ctx, fmt.Sprintf("%s.v", rc.ProjectID), rc.Version, c.dataExpiry).Err()
		if err != nil {
			return err
		}
		return pipe.Set(ctx, fmt.Sprintf("%s.d", rc.ProjectID), rc.Data, c.dataExpiry).Err()
	})
	return err
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

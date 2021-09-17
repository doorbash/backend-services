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

	scriptUpdateRC string
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
	return c.rdb.EvalSha(
		ctx,
		c.scriptUpdateRC,
		[]string{
			fmt.Sprintf("%s.v", rc.ProjectID),
			fmt.Sprintf("%s.d", rc.ProjectID),
		},
		rc.Version,
		rc.Data,
	).Err()
}

func (c *RemoteConfigRedisCache) LoadScripts(ctx context.Context) error {
	var err error
	c.scriptUpdateRC, err = c.rdb.ScriptLoad(ctx, "local v = tonumber(redis.call('GET', KEYS[1])); if v and tonumber(ARGV[1]) <= v then return nil else redis.call('SET', KEYS[1], ARGV[1]); return redis.call('SET', KEYS[2], ARGV[2]) end").Result()
	if err != nil {
		return err
	}
	return nil
}

func NewRemoteConfigRedisCache(dataExpiry time.Duration) *RemoteConfigRedisCache {
	rcCache := &RemoteConfigRedisCache{
		rdb: redis.NewClient(&redis.Options{
			Addr:            REDIS_ADDR,
			Password:        "",
			DB:              REDIS_DATABASE_AUTH,
			MaxRetries:      3,
			MinRetryBackoff: REDIS_MIN_RETRY_BACKOFF,
			MaxRetryBackoff: REDIS_MAX_RETRY_BACKOFF,
			OnConnect: func(ctx context.Context, cn *redis.Conn) error {
				log.Println("redis:", "OnConnect()", "RemoteConfig")
				return nil
			},
		}),
		dataExpiry: dataExpiry,
	}
	return rcCache
}

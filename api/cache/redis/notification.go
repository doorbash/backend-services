package redis

import (
	"context"
	"fmt"
	"log"
	"time"

	"github.com/go-redis/redis/v8"
)

type NotificationRedisCache struct {
	rdb *redis.Client
}

func (n *NotificationRedisCache) GetTimeByProjectID(ctx context.Context, pid string) (*time.Time, error) {
	t, err := n.rdb.Get(ctx, fmt.Sprintf("%s.t", pid)).Result()
	if err != nil {
		return nil, err
	}
	ret, err := time.Parse(time.RFC3339, t)
	if err != nil {
		return nil, err
	}
	return &ret, nil
}

func (n *NotificationRedisCache) GetDataByProjectID(ctx context.Context, pid string) (*string, error) {
	data, err := n.rdb.Get(ctx, fmt.Sprintf("%s.d", pid)).Result()
	return &data, err
}

func (n *NotificationRedisCache) UpdateProjectData(ctx context.Context, pid string, data string, t time.Time, expire time.Duration) error {
	_, err := n.rdb.TxPipelined(ctx, func(pipe redis.Pipeliner) error {
		if err := pipe.SetNX(ctx, fmt.Sprintf("%s.t", pid), t, expire).Err(); err != nil {
			return err
		}
		return pipe.SetNX(ctx, fmt.Sprintf("%s.d", pid), data, expire).Err()
	})
	return err
}

func (n *NotificationRedisCache) DeleteProjectData(ctx context.Context, pid string) error {
	return n.rdb.Del(ctx, fmt.Sprintf("%s.d", pid), fmt.Sprintf("%s.t", pid)).Err()
}

func (n *NotificationRedisCache) SetProjectDataExpire(ctx context.Context, pid string, expiration time.Duration) error {
	_, err := n.rdb.TxPipelined(ctx, func(pipe redis.Pipeliner) error {
		if err := pipe.Expire(ctx, fmt.Sprintf("%s.d", pid), expiration).Err(); err != nil {
			return err
		}
		return pipe.Expire(ctx, fmt.Sprintf("%s.t", pid), expiration).Err()
	})
	return err
}

func NewNotificationRedisCache() *NotificationRedisCache {
	return &NotificationRedisCache{
		rdb: redis.NewClient(&redis.Options{
			Addr:            REDIS_ADDR,
			Password:        "",
			DB:              REDIS_DATABASE_NOTIFICATOINS,
			MaxRetries:      0,
			MinRetryBackoff: REDIS_MIN_RETRY_BACKOFF,
			MaxRetryBackoff: REDIS_MAX_RETRY_BACKOFF,
			OnConnect: func(ctx context.Context, cn *redis.Conn) error {
				log.Println("redis:", "OnConnect()", "Notification")
				return nil
			},
		}),
	}
}

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

func (n *NotificationRedisCache) GetDataByProjectID(ctx context.Context, pid string) (*string, *time.Time, error) {
	t, err := n.rdb.Get(ctx, fmt.Sprintf("%s.time", pid)).Result()
	if err != nil {
		return nil, nil, err
	}
	activeTime, err := time.Parse(time.RFC3339, t)
	if err != nil {
		return nil, nil, err
	}
	data, err := n.rdb.Get(ctx, fmt.Sprintf("%s.data", pid)).Result()
	if err != nil {
		return nil, nil, err
	}
	return &data, &activeTime, nil
}

func (n *NotificationRedisCache) UpdateProjectData(ctx context.Context, pid string, data string, t time.Time, expire time.Duration) error {
	err := n.rdb.Set(ctx, fmt.Sprintf("%s.data", pid), data, expire).Err()
	if err != nil {
		return err
	}
	return n.rdb.Set(ctx, fmt.Sprintf("%s.time", pid), t, expire).Err()
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

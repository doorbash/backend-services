package redis

import (
	"context"
	"fmt"
	"log"
	"time"

	"github.com/go-redis/redis/v8"
)

type NotificationRedisCache struct {
	rdb         *redis.Client
	notifExpiry time.Duration
}

func (n *NotificationRedisCache) GetTimeByProjectID(ctx context.Context, pid string) (time.Time, error) {
	t, err := n.rdb.GetEx(ctx, fmt.Sprintf("%s.time", pid), n.notifExpiry).Result()
	if err != nil {
		return time.Unix(0, 0), err
	}
	ret, err := time.Parse(time.RFC3339, t)
	if err != nil {
		return time.Unix(0, 0), err
	}
	return ret, nil
}

func (n *NotificationRedisCache) GetDataByProjectID(ctx context.Context, pid string) (string, error) {
	return n.rdb.GetEx(ctx, fmt.Sprintf("%s.data", pid), n.notifExpiry).Result()
}

func NewNotificationRedisCache(notifExpiry time.Duration) *NotificationRedisCache {
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
		notifExpiry: notifExpiry,
	}
}

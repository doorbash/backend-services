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
	pipe := n.rdb.TxPipeline()

	cmd1 := pipe.Get(ctx, fmt.Sprintf("%s.time", pid))
	cmd2 := pipe.Get(ctx, fmt.Sprintf("%s.data", pid))

	_, err := pipe.Exec(ctx)

	if err != nil {
		return nil, nil, err
	}

	t, _ := cmd1.Result()
	data, _ := cmd2.Result()

	activeTime, err := time.Parse(time.RFC3339, t)
	if err != nil {
		return nil, nil, err
	}
	return &data, &activeTime, nil
}

func (n *NotificationRedisCache) UpdateProjectData(ctx context.Context, pid string, data string, t time.Time, expire time.Duration) error {
	_, err := n.rdb.TxPipelined(ctx, func(pipe redis.Pipeliner) error {
		if err := pipe.SetNX(ctx, fmt.Sprintf("%s.time", pid), t, expire).Err(); err != nil {
			return err
		}
		return pipe.SetNX(ctx, fmt.Sprintf("%s.data", pid), data, expire).Err()
	})
	return err
}

func (n *NotificationRedisCache) DeleteProjectData(ctx context.Context, pid string) error {
	return n.rdb.Del(ctx, fmt.Sprintf("%s.data", pid), fmt.Sprintf("%s.time", pid)).Err()
}

func (n *NotificationRedisCache) SetProjectDataExpire(ctx context.Context, pid string, expiration time.Duration) error {
	_, err := n.rdb.TxPipelined(ctx, func(pipe redis.Pipeliner) error {
		if err := pipe.Expire(ctx, fmt.Sprintf("%s.data", pid), expiration).Err(); err != nil {
			return err
		}
		return pipe.Expire(ctx, fmt.Sprintf("%s.time", pid), expiration).Err()
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

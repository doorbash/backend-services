package redis

import (
	"context"
	"fmt"
	"log"
	"strings"
	"time"

	"github.com/go-redis/redis/v8"
)

type NotificationRedisCache struct {
	rdb *redis.Client

	scriptIncrClicks string
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
	pipe := n.rdb.TxPipeline()
	cmd := pipe.Get(ctx, fmt.Sprintf("%s.d", pid))
	_ = pipe.Incr(ctx, fmt.Sprintf("%s.v", pid))
	_, err := pipe.Exec(ctx)
	if err != nil {
		return nil, err
	}
	data := cmd.Val()
	return &data, nil
}

func (n *NotificationRedisCache) UpdateProjectData(ctx context.Context, pid string, ids string, data string, t time.Time, expire time.Duration) error {
	_, err := n.rdb.TxPipelined(ctx, func(pipe redis.Pipeliner) error {
		err := pipe.SetNX(ctx, fmt.Sprintf("%s.t", pid), t, expire).Err()
		if err != nil {
			return err
		}
		err = pipe.SetNX(ctx, fmt.Sprintf("%s.v", pid), 0, expire).Err()
		if err != nil {
			return err
		}
		// log.Println("ids =", ids)
		idArr := strings.Split(ids, " ")
		args := make([]string, 2*len(idArr))
		for i, id := range idArr {
			args[i*2] = id
			args[i*2+1] = "0"
		}
		// log.Println("args:", args)
		err = pipe.HSet(ctx, fmt.Sprintf("%s.c", pid), args).Err()
		if err != nil {
			return err
		}
		return pipe.SetNX(ctx, fmt.Sprintf("%s.d", pid), data, expire).Err()
	})
	return err
}

func (n *NotificationRedisCache) DeleteProjectData(ctx context.Context, pid string) error {
	return n.rdb.Del(
		ctx,
		fmt.Sprintf("%s.t", pid),
		fmt.Sprintf("%s.v", pid),
		fmt.Sprintf("%s.c", pid),
		fmt.Sprintf("%s.d", pid),
	).Err()
}

func (n *NotificationRedisCache) SetProjectDataExpire(ctx context.Context, pid string, expiration time.Duration) error {
	_, err := n.rdb.TxPipelined(ctx, func(pipe redis.Pipeliner) error {
		err := pipe.Expire(ctx, fmt.Sprintf("%s.t", pid), expiration).Err()
		if err != nil {
			return err
		}
		err = pipe.Expire(ctx, fmt.Sprintf("%s.v", pid), expiration).Err()
		if err != nil {
			return err
		}
		err = pipe.Expire(ctx, fmt.Sprintf("%s.c", pid), expiration).Err()
		if err != nil {
			return err
		}
		return pipe.Expire(ctx, fmt.Sprintf("%s.d", pid), expiration).Err()
	})
	return err
}

func (n *NotificationRedisCache) GetViewsByProjectID(ctx context.Context, pid string) (string, error) {
	return n.rdb.GetSet(ctx, fmt.Sprintf("%s.v", pid), 0).Result()
}

func (n *NotificationRedisCache) GetClicksByProjectID(ctx context.Context, pid string) (map[string]string, error) {
	return n.rdb.HGetAll(ctx, fmt.Sprintf("%s.c", pid)).Result()
}

func (n *NotificationRedisCache) IncrClicks(ctx context.Context, pid string, id string) (bool, error) {
	ret, err := n.rdb.EvalSha(
		ctx,
		n.scriptIncrClicks,
		[]string{fmt.Sprintf("%s.c", pid), id},
	).Result()
	if err != nil {
		return false, err
	}
	return ret == "t", nil
}

func (n *NotificationRedisCache) LoadScripts(ctx context.Context) error {
	var err error
	n.scriptIncrClicks, err = n.rdb.ScriptLoad(ctx, "if redis.call('HEXISTS', KEYS[1], KEYS[2])==1 then redis.call('HINCRBY', KEYS[1], KEYS[2], 1); return 't' else return 'f' end").Result()
	if err != nil {
		return err
	}
	return nil
}

func NewNotificationRedisCache() *NotificationRedisCache {
	return &NotificationRedisCache{
		rdb: redis.NewClient(&redis.Options{
			Addr:            REDIS_ADDR,
			Password:        "",
			DB:              REDIS_DATABASE_NOTIFICATOINS,
			MaxRetries:      3,
			MinRetryBackoff: REDIS_MIN_RETRY_BACKOFF,
			MaxRetryBackoff: REDIS_MAX_RETRY_BACKOFF,
			OnConnect: func(ctx context.Context, cn *redis.Conn) error {
				log.Println("redis:", "OnConnect()", "Notification")
				return nil
			},
		}),
	}
}

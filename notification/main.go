package main

import (
	"context"
	"fmt"
	"log"
	"os"
	"time"

	_redis "github.com/doorbash/backend-services/api/cache/redis"
	"github.com/doorbash/backend-services/api/util"
	"github.com/go-redis/redis/v8"
	"github.com/jackc/pgx/v4"
	"github.com/jackc/pgx/v4/pgxpool"
)

const (
	DATA_EXPIRE = 5 * time.Minute
)

func UpdateNotifications(pool *pgxpool.Pool, rdb *redis.Client) error {
	ctx, cancel := util.GetContextWithTimeout(context.Background())
	defer cancel()
	cmd, err := pool.Exec(ctx, `UPDATE notifications SET status = 4 WHERE status = 1 AND expires_at <= $1`, time.Now())

	if err != nil {
		log.Println(err)
		return err
	}

	if cmd.RowsAffected() > 0 {
		log.Println("expired", cmd.RowsAffected(), "notifications")
	}

	ctx, cancel = util.GetContextWithTimeout(context.Background())
	defer cancel()
	rows, err := pool.Query(ctx, `SELECT MAX(notifications.id) AS id, pid, MAX(actived_at) AS time, '[' || STRING_AGG('{"id":' || id || ',"actived_at":"' || actived_at || '","title":"' || title || '","text":"' || text || '"}', ',') || ']' AS data FROM notifications WHERE status = 1 GROUP BY pid`)
	if err != nil {
		log.Println(err)
		return err
	}
	for rows.Next() {
		var id int
		var pid string
		var t time.Time
		var data string
		err := rows.Scan(
			&id,
			&pid,
			&t,
			&data,
		)
		if err != nil {
			return err
		}
		log.Println("pid:", pid)

		ctx, cancel := util.GetContextWithTimeout(context.Background())
		err = rdb.Set(ctx, fmt.Sprintf("%s.time", pid), t, DATA_EXPIRE).Err()
		cancel()
		if err != nil {
			log.Println(err)
			return err
		}

		ctx, cancel = util.GetContextWithTimeout(context.Background())
		err = rdb.Set(ctx, fmt.Sprintf("%s.data", pid), data, DATA_EXPIRE).Err()
		cancel()
		if err != nil {
			log.Println(err)
			return err
		}
	}
	return nil
}

func main() {
	poolConfig, err := pgxpool.ParseConfig(fmt.Sprintf(
		"postgres://%s:%s@db:5432/%s",
		os.Getenv("DATABASE_USER"),
		os.Getenv("DATABASE_PASSWORD"),
		os.Getenv("DATABASE_NAME"),
	))

	if err != nil {
		log.Fatalln("Unable to parse DATABASE_URL. error:", err)
	}

	poolConfig.HealthCheckPeriod = time.Minute
	poolConfig.ConnConfig.Logger = &util.DatabaseLogger{}
	poolConfig.ConnConfig.LogLevel = pgx.LogLevelDebug

	ctx, cancel := util.GetContextWithTimeout(context.Background())
	defer cancel()
	pool, err := pgxpool.ConnectConfig(ctx, poolConfig)
	if err != nil {
		log.Fatalln("Unable to create connection pool. error:", err)
	}

	rdb := redis.NewClient(&redis.Options{
		Addr:            _redis.REDIS_ADDR,
		Password:        "",
		DB:              _redis.REDIS_DATABASE_NOTIFICATOINS,
		MaxRetries:      0,
		MinRetryBackoff: _redis.REDIS_MIN_RETRY_BACKOFF,
		MaxRetryBackoff: _redis.REDIS_MAX_RETRY_BACKOFF,
		OnConnect: func(ctx context.Context, cn *redis.Conn) error {
			log.Println("redis:", "OnConnect()", "Notification")
			return nil
		},
	})

	for {
		UpdateNotifications(pool, rdb)
		time.Sleep(time.Minute)
	}
}

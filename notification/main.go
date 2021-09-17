package main

import (
	"context"
	"fmt"
	"log"
	"os"
	"strings"
	"time"

	"github.com/doorbash/backend-services/api/cache/redis"
	"github.com/doorbash/backend-services/api/domain"
	"github.com/doorbash/backend-services/api/util"
	"github.com/jackc/pgx/v4"
	"github.com/jackc/pgx/v4/pgxpool"
)

func UpdateNotifications(pool *pgxpool.Pool, noCache domain.NotificationCache) error {
	now := time.Now()

	// scheduled(2) -> active(1)
	ctx, cancel := util.GetContextWithTimeout(context.Background())
	defer cancel()
	cmd, err := pool.Exec(ctx, "UPDATE notifications SET status = 1, active_time = $1 WHERE status = 2 AND schedule_time <= $1", now)
	if err != nil {
		return err
	}

	if cmd.RowsAffected() > 0 {
		log.Println("just set", cmd.RowsAffected(), "notifications as active")
	}

	// active(1), scheduled(2) -> finished(4)
	ctx, cancel = util.GetContextWithTimeout(context.Background())
	defer cancel()
	cmd, err = pool.Exec(ctx, "UPDATE notifications SET status = 4 WHERE (status = 1 OR status = 2) AND expire_time <= $1", now)
	if err != nil {
		return err
	}

	if cmd.RowsAffected() > 0 {
		log.Println("just set", cmd.RowsAffected(), "notifications as finished")
	}

	// udpate notification views_count, clicks_count
	ctx, cancel = util.GetContextWithTimeout(context.Background())
	defer cancel()
	rows, err := pool.Query(ctx, "SELECT STRING_AGG(CONCAT(id::TEXT, ':', clicks_count::TEXT), ' ' ORDER BY id ASC) as clicks_count, pid FROM notifications WHERE status = 1 GROUP BY pid")
	if err != nil {
		return err
	}

	for rows.Next() {
		var c string
		var pid string
		err := rows.Scan(&c, &pid)
		if err != nil {
			return err
		}

		cArr := strings.Split(c, " ")
		clicks := make(map[string]string)
		for _, v := range cArr {
			parts := strings.Split(v, ":")
			clicks[parts[0]] = parts[1]
		}

		ctx, cancel = util.GetContextWithTimeout(context.Background())
		defer cancel()
		views, err := noCache.GetViewsByProjectID(ctx, pid)
		if err != nil {
			return err
		}

		if views != "0" {
			ctx, cancel = util.GetContextWithTimeout(context.Background())
			defer cancel()
			cmd, err = pool.Exec(ctx, "UPDATE notifications SET views_count = views_count + $1 WHERE status = 1 AND pid = $2", views, pid)
			if err != nil {
				return err
			}

			if cmd.RowsAffected() > 0 {
				log.Println("just added", views, "views for", cmd.RowsAffected(), "notifications of project:", pid)
			}
		}

		rClicks, err := noCache.GetClicksByProjectID(ctx, pid)

		if err != nil {
			return err
		}

		for k, v := range clicks {
			cc, ok := rClicks[k]
			if !ok {
				continue
			}
			if v == cc {
				continue
			}
			ctx, cancel = util.GetContextWithTimeout(context.Background())
			defer cancel()
			_, err := pool.Exec(ctx, "UPDATE notifications SET clicks_count = $1 WHERE status = 1 AND id = $2", cc, k)
			if err != nil {
				return err
			}
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

	noCache := redis.NewNotificationRedisCache()

	for {
		err := UpdateNotifications(pool, noCache)
		if err != nil {
			log.Println(err)
		}
		time.Sleep(time.Minute)
	}
}

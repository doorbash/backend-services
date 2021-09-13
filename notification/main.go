package main

import (
	"context"
	"fmt"
	"log"
	"os"
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

	// udpate notification views_count
	ctx, cancel = util.GetContextWithTimeout(context.Background())
	defer cancel()
	rows, err := pool.Query(ctx, "SELECT MAX(id), pid FROM notifications WHERE status = 1 GROUP BY pid")
	if err != nil {
		return err
	}

	for rows.Next() {
		var id int
		var pid string
		err := rows.Scan(&id, &pid)
		if err != nil {
			return err
		}

		ctx, cancel = util.GetContextWithTimeout(context.Background())
		defer cancel()
		views, err := noCache.GetViewByProjectID(ctx, pid)
		if err != nil {
			return err
		}

		if views != "0" {
			ctx, cancel = util.GetContextWithTimeout(context.Background())
			defer cancel()
			cmd, err = pool.Exec(ctx, "UPDATE notifications SET views_count = views_count + $1 WHERE pid = $2", views, pid)
			if err != nil {
				return err
			}

			if cmd.RowsAffected() > 0 {
				log.Println("just added", views, "views for", cmd.RowsAffected(), "notifications of project:", pid)
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

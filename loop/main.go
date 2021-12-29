package main

import (
	"context"
	"errors"
	"fmt"
	"log"
	"os"
	"time"

	"github.com/doorbash/backend-services/api/cache"
	_redis "github.com/doorbash/backend-services/api/cache/redis"
	"github.com/doorbash/backend-services/api/domain"
	_pg "github.com/doorbash/backend-services/api/repository/pg"
	"github.com/doorbash/backend-services/api/util"
	"github.com/go-redis/redis/v8"
	"github.com/jackc/pgtype"
	"github.com/jackc/pgx/v4"
	"github.com/jackc/pgx/v4/pgxpool"
)

func UpdateRemoteConfigs(
	pool *pgxpool.Pool,
	rcRepo domain.RemoteConfigRepository,
	rcCache domain.RemoteConfigCache,
) error {
	log.Println("UpdateRemoteConfigs()")
	ctx, cancel := util.GetContextWithTimeout(context.Background())
	defer cancel()
	rows, err := pool.Query(ctx, "SELECT pid, version FROM remote_configs WHERE data IS NOT NULL")
	if err != nil {
		return err
	}
	for rows.Next() {
		var pid string
		var version int
		err := rows.Scan(&pid, &version)
		if err != nil {
			return err
		}

		log.Println("pid =", pid, "version =", version)

		ctx, cancel := util.GetContextWithTimeout(context.Background())
		defer cancel()
		v, err := rcCache.GetVersionByProjectID(ctx, pid)
		if err != nil && err != redis.Nil {
			return err
		}

		if err == redis.Nil || version > *v {
			ctx, cancel := util.GetContextWithTimeout(context.Background())
			defer cancel()
			remoteConfig, err := rcRepo.GetByProjectID(ctx, pid)
			if err != nil {
				log.Println(err)
				continue
			}
			ctx, cancel = util.GetContextWithTimeout(context.Background())
			defer cancel()
			err = rcCache.Update(ctx, remoteConfig)
			if err != nil {
				log.Println(err)
			}
		}
	}
	return nil
}

func UpdateNotifications(pool *pgxpool.Pool, noCache domain.NotificationCache) error {
	log.Println("UpdateNotifications()")
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
	rows, err := pool.Query(ctx, "SELECT DISTINCT pid FROM notifications WHERE status = 1")
	if err != nil {
		return err
	}

	for rows.Next() {
		var pid string
		err := rows.Scan(&pid)
		if err != nil {
			return err
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

		for k, v := range rClicks {
			if v == "0" {
				continue
			}
			ctx, cancel = util.GetContextWithTimeout(context.Background())
			defer cancel()
			_, err := pool.Exec(ctx, "UPDATE notifications SET clicks_count = clicks_count + $1 WHERE status = 1 AND id = $2", v, k)
			if err != nil {
				return err
			}
		}

		err = updateNotificationData(pool, noCache, pid)

		if err != nil {
			log.Println(err)
		}
	}

	return nil
}

func updateNotificationData(pool *pgxpool.Pool, noCache domain.NotificationCache, pid string) error {
	ctx, cancel := util.GetContextWithTimeout(context.Background())
	defer cancel()

	row := pool.QueryRow(ctx, "SELECT _active_time, _ids, _data FROM notifications_data($1)", pid)
	var activeTime pgtype.Timestamptz
	var ids pgtype.Text
	var data pgtype.Text
	err := row.Scan(
		&activeTime,
		&ids,
		&data,
	)
	if err != nil {
		return err
	}
	if activeTime.Status == pgtype.Null {
		return errors.New("active_time is null")
	}
	if ids.Status == pgtype.Null {
		return errors.New("ids is null")
	}
	if data.Status == pgtype.Null {
		return errors.New("data is null")
	}

	log.Println(">>>>>>>>>>>>", "ids=", ids.String, ",data=", data.String, ",t=", activeTime.Time)

	ctx, cancel = util.GetContextWithTimeout(context.Background())
	defer cancel()
	return noCache.UpdateProjectData(ctx, pid, ids.String, data.String, activeTime.Time, 15*time.Minute)
}

func main() {
	log.SetFlags(log.LstdFlags | log.Lshortfile)

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

	rcRepo := _pg.NewRemoteConfigPostgresRepository(pool)

	noCache := _redis.NewNotificationRedisCache()
	rcCache := _redis.NewRemoteConfigRedisCache(24 * time.Hour)

	if err := cache.InitCacheScripts(rcCache, noCache); err != nil {
		log.Fatalln(err)
	}

	go func() {
		for {
			err := UpdateNotifications(pool, noCache)
			if err != nil {
				log.Println(err)
			}
			time.Sleep(10 * time.Minute)
		}
	}()

	for {
		err := UpdateRemoteConfigs(pool, rcRepo, rcCache)
		if err != nil {
			log.Println(err)
		}
		time.Sleep(5 * time.Minute)
	}
}

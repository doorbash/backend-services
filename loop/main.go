package main

import (
	"context"
	"fmt"
	"log"
	"os"
	"strings"
	"time"

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
	rows, err := pool.Query(ctx, "SELECT pid, data, version, modified FROM remote_config WHERE data IS NOT NULL")
	if err != nil {
		return err
	}
	for rows.Next() {
		var pid string
		var data string
		var version int
		var modified bool
		err := rows.Scan(&pid, &data, &version, &modified)
		if err != nil {
			return err
		}
		log.Println("pid =", pid, "data =", data, "version =", version, "modified =", modified)
		ctx, cancel := util.GetContextWithTimeout(context.Background())
		defer cancel()
		exists, err := rcCache.GetDataExistsByProjectID(ctx, pid)
		if err != nil {
			return err
		}

		if !exists || modified {
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
			if err != nil && err != redis.Nil {
				log.Println(err)
				continue
			}
			ctx, cancel = util.GetContextWithTimeout(context.Background())
			defer cancel()
			_, err = pool.Exec(ctx, "UPDATE remote_configs SET modified = FALSE where pid = $1", pid)
			if err != nil {
				log.Println(err)
			}
		}
	}
	return nil
}

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

	// update notifications cache
	ctx, cancel = util.GetContextWithTimeout(context.Background())
	defer cancel()
	rows, err = pool.Query(ctx, "SELECT pid, bool_or(modified) AS modified FROM notifications WHERE status = 1 GROUP BY pid")
	if err != nil {
		return err
	}

	for rows.Next() {
		var pid string
		var modified bool
		rows.Scan(&pid, &modified)
		err := rows.Scan(&pid, &modified)
		if err != nil {
			return err
		}
		log.Println("pid =", pid, "modified =", modified)
		ctx, cancel := util.GetContextWithTimeout(context.Background())
		defer cancel()
		exists, err := noCache.GetDataExistsByProjectID(ctx, pid)
		if err != nil {
			return err
		}
		if !exists || modified {
			ctx, cancel := util.GetContextWithTimeout(context.Background())
			defer cancel()

			row := pool.QueryRow(ctx, `WITH schedules AS (SELECT MIN(schedule_time) AS schedule_min FROM notifications WHERE pid = $1 AND status = 2)
SELECT
MAX(active_time) AS active_time,
EXTRACT(EPOCH FROM LEAST(MIN(expire_time), (select schedule_min from schedules)) - CURRENT_TIMESTAMP)::INT AS expire,
STRING_AGG(id::TEXT, ' ' ORDER BY id ASC) AS ids,
'[' || STRING_AGG(CONCAT('{"id":', id, ',"title":"', title, '","text":"', text, '","image":"', image, '","priority":"', priority, '","style":"', style, '","action":"', action, '","extra":"', extra, '","active_time":"', to_char((active_time::timestamp), 'YYYY-MM-DD"T"HH24:MI:SS"Z"'), '"}'), ',') || ']' AS data
FROM notifications
WHERE pid = $1 AND status = 1
ORDER BY active_time ASC`, pid)
			var activeTime pgtype.Timestamptz
			var expire pgtype.Int4
			var ids pgtype.Text
			var data pgtype.Text
			err := row.Scan(
				&activeTime,
				&expire,
				&ids,
				&data,
			)
			if err != nil {
				log.Println(err)
				continue
			}
			if activeTime.Status == pgtype.Null {
				log.Println("active_time is null")
				continue
			}
			if expire.Status == pgtype.Null {
				log.Println("expire is null")
				continue
			}
			if ids.Status == pgtype.Null {
				log.Println("ids is null")
				continue
			}
			if data.Status == pgtype.Null {
				log.Println("data is null")
				continue
			}
			if expire.Int < 0 {
				log.Println(fmt.Sprint("expire < 0, expire:", &expire.Int, "seconds"))
				continue
			}
			ctx, cancel = util.GetContextWithTimeout(context.Background())
			defer cancel()
			err = noCache.UpdateProjectData(ctx, pid, ids.String, data.String, activeTime.Time, time.Duration(expire.Int)*time.Second)
			if err != nil {
				log.Println(err)
				continue
			}
			ctx, cancel = util.GetContextWithTimeout(context.Background())
			defer cancel()
			_, err = pool.Exec(ctx, "UPDATE notifications SET modified = FALSE WHERE pid = $1", pid)
			if err != nil {
				log.Printf("notifications cache update: (pid = %s) error: %s\n", pid, err)
			}
		}
	}

	return nil
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

	for {
		err := UpdateRemoteConfigs(pool, rcRepo, rcCache)
		if err != nil {
			log.Println(err)
		}
		err = UpdateNotifications(pool, noCache)
		if err != nil {
			log.Println(err)
		}
		time.Sleep(time.Minute)
	}
}

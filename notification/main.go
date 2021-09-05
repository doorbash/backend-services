package main

import (
	"context"
	"fmt"
	"log"
	"os"
	"time"

	"github.com/doorbash/backend-services/api/util"
	"github.com/jackc/pgx/v4"
	"github.com/jackc/pgx/v4/pgxpool"
)

func UpdateNotifications(pool *pgxpool.Pool) error {
	now := time.Now()

	// scheduled(2) -> active(1)
	ctx, cancel := util.GetContextWithTimeout(context.Background())
	defer cancel()
	cmd, err := pool.Exec(ctx, `UPDATE notifications SET status = 1, active_time = $1 WHERE status = 2 AND schedule_time <= $1`, now)

	if err != nil {
		log.Println(err)
		return err
	}

	if cmd.RowsAffected() > 0 {
		log.Println("just set", cmd.RowsAffected(), "notifications as active")
	}

	// -> active(1), scheduled(2) -> finished(4)
	ctx, cancel = util.GetContextWithTimeout(context.Background())
	defer cancel()
	cmd, err = pool.Exec(ctx, `UPDATE notifications SET status = 4 WHERE (status = 1 OR status = 2) AND expire_time <= $1`, now)

	if err != nil {
		log.Println(err)
		return err
	}

	if cmd.RowsAffected() > 0 {
		log.Println("just set", cmd.RowsAffected(), "notifications as finished")
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

	for {
		UpdateNotifications(pool)
		time.Sleep(time.Minute)
	}
}

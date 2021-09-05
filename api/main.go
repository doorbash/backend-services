package main

import (
	"context"
	"fmt"
	"log"
	"net/http"
	"os"
	"time"

	"github.com/gorilla/mux"
	"github.com/jackc/pgx/v4"
	"github.com/jackc/pgx/v4/pgxpool"

	_redis "github.com/doorbash/backend-services/api/cache/redis"
	handler "github.com/doorbash/backend-services/api/handler"
	auth "github.com/doorbash/backend-services/api/handler/auth"
	_pg "github.com/doorbash/backend-services/api/repository/pg"
	util "github.com/doorbash/backend-services/api/util"
	"github.com/doorbash/backend-services/api/util/middleware"
)

func initDatabase() *pgxpool.Pool {
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

	queries := []string{
		_pg.CreateUserTable(),
		_pg.CreateProjectTable(),
		_pg.CreateRemoteConfigTable(),
		_pg.CreateNotificationsTable(),
	}

	for _, q := range queries {
		ctx, cancel = util.GetContextWithTimeout(context.Background())
		defer cancel()
		pool.Exec(ctx, q)
		if err != nil {
			log.Fatalln(err)
		}
	}

	return pool
}

func main() {
	pool := initDatabase()
	defer pool.Close()

	userRepo := _pg.NewUserPostgresRepository(pool)
	rcRepo := _pg.NewRemoteConfigPostgresRepository(pool)
	projectRepo := _pg.NewProjectPostgresRepository(pool)
	noRepo := _pg.NewNotificationPostgresRepository(pool)

	r := mux.NewRouter()
	r.Use(middleware.LoggerMiddleware)

	authHandler := auth.NewGithubOAuth2Handler(
		r,
		userRepo,
		_redis.NewAuthRedisCache(1*time.Hour),
		os.Getenv("AUTH_CLIENT_SECRET"),
		os.Getenv("AUTH_CLIENT_ID"),
		os.Getenv("AUTH_SESSION_KEY"),
		os.Getenv("API_ADMIN_EMAIL"),
		os.Getenv("API_PATH"),
		os.Getenv("API_MODE") == "private",
		"/oauth2",
	)

	handler.NewUserHandler(
		r,
		authHandler.Middleware,
		userRepo,
	)

	handler.NewRemoteConfigHandler(
		r,
		authHandler.Middleware,
		rcRepo,
		projectRepo,
		_redis.NewRemoteConfigRedisCache(24*time.Hour))

	handler.NewNotificationHandler(
		r,
		authHandler.Middleware,
		noRepo, projectRepo,
		_redis.NewNotificationRedisCache(),
	)

	handler.NewProjectHandler(
		r,
		authHandler.Middleware,
		projectRepo,
		userRepo,
	)

	http.Handle("/", r)
	log.Fatal(http.ListenAndServe(os.Getenv("API_LISTEN_ADDR"), r))
}

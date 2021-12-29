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

	"github.com/doorbash/backend-services/api/cache"
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

	queries := make([]string, 0)

	queries = append(queries, _pg.CreateUsers()...)
	queries = append(queries, _pg.CreateProjects()...)
	queries = append(queries, _pg.CreateRemoteConfigs()...)
	queries = append(queries, _pg.CreateNotifications()...)

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

	authCache := _redis.NewAuthRedisCache(6 * time.Hour)
	rcCache := _redis.NewRemoteConfigRedisCache(24 * time.Hour)
	noCache := _redis.NewNotificationRedisCache()

	if err := cache.InitCacheScripts(rcCache, noCache); err != nil {
		log.Fatalln(err)
	}

	r := mux.NewRouter()
	r.Use(middleware.LoggerMiddleware)

	authHandler := auth.NewGithubOAuth2Handler(
		r,
		userRepo,
		authCache,
		os.Getenv("AUTH_CLIENT_SECRET"),
		os.Getenv("AUTH_CLIENT_ID"),
		os.Getenv("AUTH_SESSION_KEY"),
		os.Getenv("API_ADMIN_EMAIL"),
		os.Getenv("API_MODE") == "private",
		"/api",
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
		rcCache,
	)

	handler.NewNotificationHandler(
		r,
		authHandler.Middleware,
		noRepo, projectRepo,
		noCache,
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

package main

import (
	"context"
	"fmt"
	"log"
	"net/http"
	"os"
	"time"

	"github.com/go-redis/redis/v8"
	"github.com/gorilla/mux"
	"github.com/jackc/pgx/v4"
	"github.com/jackc/pgx/v4/pgxpool"

	_redis "github.com/doorbash/backend-services-api/api/cache/redis"
	handler "github.com/doorbash/backend-services-api/api/handler"
	auth "github.com/doorbash/backend-services-api/api/handler/auth"
	_pg "github.com/doorbash/backend-services-api/api/repository/pg"
	util "github.com/doorbash/backend-services-api/api/util"
)

const (
	REDIS_ADDR                   = "redis:6379"
	REDIS_EXPIRE_TIME_AUTH_TOKEN = 1 * time.Hour
	REDIS_EXPIRE_TIME_RC_DATA    = 24 * time.Hour
	REDIS_MIN_RETRY_BACKOFF      = 3 * time.Second
	REDIS_MAX_RETRY_BACKOFF      = 5 * time.Second
	REDIS_DATABASE_AUTH          = 0
	REDIS_DATABASE_RC            = 1
)

func initDatabase() *pgxpool.Pool {
	poolConfig, err := pgxpool.ParseConfig(fmt.Sprintf(
		"postgres://%s:%s@%s:5432/%s",
		os.Getenv("DATABASE_USER"),
		os.Getenv("DATABASE_PASSWORD"),
		os.Getenv("DATABASE_HOST"),
		os.Getenv("DATABASE_NAME"),
	))

	if err != nil {
		log.Fatalln("Unable to parse DATABASE_URL. error:", err)
	}

	poolConfig.HealthCheckPeriod = time.Minute
	poolConfig.ConnConfig.Logger = &util.DatabaseLogger{}
	poolConfig.ConnConfig.LogLevel = pgx.LogLevelDebug

	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()
	pool, err := pgxpool.ConnectConfig(ctx, poolConfig)
	if err != nil {
		log.Fatalln("Unable to create connection pool. error:", err)
	}

	queries := []string{
		_pg.CreateUserTable(),
		_pg.CreateProjectTable(),
		_pg.CreateRemoteConfigTable(),
	}

	for _, q := range queries {
		ctx, cancel = context.WithTimeout(context.Background(), 5*time.Second)
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

	userRepo := _pg.NewUserPostgressRepository(pool)
	rcRepo := _pg.NewRemoteConfigPostgressRepository(pool)
	projectRepo := _pg.NewProjectPostgressRepository(pool)

	r := mux.NewRouter()
	r.Use(util.LoggerMiddleware)

	authHandler := auth.NewGithubOAuth2Handler(
		r,
		userRepo,
		_redis.NewAuthRedisCache(redis.NewClient(&redis.Options{
			Addr:            REDIS_ADDR,
			Password:        "",
			DB:              REDIS_DATABASE_RC,
			MaxRetries:      0,
			MinRetryBackoff: REDIS_MIN_RETRY_BACKOFF,
			MaxRetryBackoff: REDIS_MAX_RETRY_BACKOFF,
			OnConnect: func(ctx context.Context, cn *redis.Conn) error {
				log.Println("redis:", "OnConnect()", "Auth")
				return nil
			},
		}), REDIS_EXPIRE_TIME_AUTH_TOKEN),
		os.Getenv("AUTH_CLIENT_SECRET"),
		os.Getenv("AUTH_CLIENT_ID"),
		os.Getenv("AUTH_SESSION_KEY"),
		os.Getenv("API_ADMIN_EMAIL"),
		os.Getenv("API_PATH"),
		os.Getenv("API_MODE") == "private",
		"/oauth2",
	)
	handler.NewUserHandler(r, authHandler.Middleware, userRepo, "/users")
	handler.NewRemoteConfigHandler(
		r,
		authHandler.Middleware,
		rcRepo,
		projectRepo,
		_redis.NewRemoteConfigRedisCache(redis.NewClient(&redis.Options{
			Addr:            REDIS_ADDR,
			Password:        "",
			DB:              REDIS_DATABASE_AUTH,
			MaxRetries:      0,
			MinRetryBackoff: REDIS_MIN_RETRY_BACKOFF,
			MaxRetryBackoff: REDIS_MAX_RETRY_BACKOFF,
			OnConnect: func(ctx context.Context, cn *redis.Conn) error {
				log.Println("redis:", "OnConnect()", "RemoteConfig")
				return nil
			},
		}), REDIS_EXPIRE_TIME_RC_DATA), "/rc")

	handler.NewProjectHandler(r, authHandler.Middleware, projectRepo, userRepo, "/projects")

	http.Handle("/", r)
	log.Fatal(http.ListenAndServe(os.Getenv("API_LISTEN_ADDR"), r))
}

package redis

import (
	"context"
	"fmt"
	"log"
	"strconv"
	"strings"
	"time"

	"github.com/doorbash/backend-services/api/util"
	"github.com/go-redis/redis/v8"
)

type AuthRedisCache struct {
	rdb         *redis.Client
	tokenExpiry time.Duration
}

func (a *AuthRedisCache) GetUserByToken(ctx context.Context, token string) (string, int, error) {
	user, err := a.rdb.GetEx(ctx, token, a.tokenExpiry).Result()
	if err != nil {
		return "", 0, err
	}
	parts := strings.Split(user, " ")
	if len(parts) != 2 {
		return "", 0, ErrRedisBadValue
	}
	id, err := strconv.Atoi(parts[1])
	if err != nil {
		return "", 0, ErrRedisBadValue
	}
	return parts[0], id, nil
}

func (a *AuthRedisCache) GenerateAndSaveToken(ctx context.Context, email string, id int) (string, error) {
	token := util.RandomString(50)
	err := a.rdb.SetNX(ctx, token, fmt.Sprintf("%s %d", email, id), a.tokenExpiry).Err()
	if err != nil {
		return "", err
	}
	return token, nil
}

func (a *AuthRedisCache) DeleteToken(ctx context.Context, token string) error {
	return a.rdb.Del(ctx, token).Err()
}

func (a *AuthRedisCache) GetTokenExpiry() time.Duration {
	return a.tokenExpiry
}

func NewAuthRedisCache(tokenExpiry time.Duration) *AuthRedisCache {
	return &AuthRedisCache{
		rdb: redis.NewClient(&redis.Options{
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
		}),
		tokenExpiry: tokenExpiry,
	}
}

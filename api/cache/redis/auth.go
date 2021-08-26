package redis

import (
	"context"
	"time"

	"github.com/doorbash/backend-services-api/api/util"
	"github.com/go-redis/redis/v8"
)

type AuthRedisCache struct {
	rdb         *redis.Client
	tokenExpiry time.Duration
}

func (a *AuthRedisCache) GetEmailByToken(ctx context.Context, token string) (string, error) {
	email, err := a.rdb.GetEx(ctx, token, a.tokenExpiry).Result()
	if err != nil {
		return "", err
	}
	return email, nil
}

func (a *AuthRedisCache) GenerateAndSaveToken(ctx context.Context, email string) (string, error) {
	token := util.RandomString(50)
	err := a.rdb.Set(ctx, token, email, a.tokenExpiry).Err()
	if err != nil {
		return "", err
	}
	return token, nil
}

func (a *AuthRedisCache) DeleteToken(ctx context.Context, token string) error {
	return a.rdb.Del(ctx, token).Err()
}

func (a *AuthRedisCache) UpdateTokenExpiry(ctx context.Context, token string) error {
	return a.rdb.Expire(ctx, token, a.tokenExpiry).Err()
}

func (a *AuthRedisCache) GetTokenExpiry() time.Duration {
	return a.tokenExpiry
}

func NewAuthRedisCache(rdb *redis.Client, tokenExpiry time.Duration) *AuthRedisCache {
	return &AuthRedisCache{
		rdb:         rdb,
		tokenExpiry: tokenExpiry,
	}
}

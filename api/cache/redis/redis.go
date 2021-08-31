package redis

import (
	"errors"
	"time"
)

var (
	ErrRedisBadValue = errors.New("Bad value")
)

const (
	REDIS_ADDR                   = "redis:6379"
	REDIS_MIN_RETRY_BACKOFF      = 3 * time.Second
	REDIS_MAX_RETRY_BACKOFF      = 5 * time.Second
	REDIS_DATABASE_AUTH          = 0
	REDIS_DATABASE_RC            = 1
	REDIS_DATABASE_NOTIFICATOINS = 2
)

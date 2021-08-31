package domain

import (
	"context"
	"time"
)

const (
	NOTIFICATION_STATUS_PENDING   = 0
	NOTIFICATION_STATUS_ACTIVE    = 1
	NOTIFICATION_STATUS_SCHEDULED = 2
	NOTIFICATION_STATUS_CANCELED  = 3
	NOTIFICATION_STATUS_FINISHED  = 4
)

type Notification struct {
	ID        int       `json:"id"`
	PID       string    `json:"pid"`
	Status    int       `json:"status"`
	Title     string    `json:"title"`
	Text      string    `json:"text"`
	CreatedAt time.Time `json:"created_at"`
	ActivedAt time.Time `json:"actived_at"`
	ExpiresAt time.Time `json:"expires_at"`
}

type NotificationCache interface {
	GetTimeByProjectID(ctx context.Context, pid string) (time.Time, error)
	GetDataByProjectID(ctx context.Context, pid string) (string, error)
}

type NotificationRepository interface {
	GetByID(ctx context.Context, id string) (*Notification, error)
	GetByPID(ctx context.Context, pid string) ([]Notification, error)
	Insert(ctx context.Context, n *Notification) error
	Update(ctx context.Context, n *Notification) error
	Delete(ctx context.Context, n *Notification) error
}

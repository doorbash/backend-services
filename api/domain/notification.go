package domain

import (
	"context"
	"time"

	"github.com/jackc/pgtype"
)

const (
	NOTIFICATION_STATUS_PENDING   = 0
	NOTIFICATION_STATUS_ACTIVE    = 1
	NOTIFICATION_STATUS_SCHEDULED = 2
	NOTIFICATION_STATUS_CANCELED  = 3
	NOTIFICATION_STATUS_FINISHED  = 4
)

type Notification struct {
	ID           int                `json:"id"`
	PID          string             `json:"pid"`
	Status       int                `json:"status"`
	Title        string             `json:"title"`
	Text         string             `json:"text"`
	CreateTime   pgtype.Timestamptz `json:"create_time"`
	ActiveTime   pgtype.Timestamptz `json:"active_time"`
	ExpireTime   pgtype.Timestamptz `json:"expire_time"`
	ScheduleTime pgtype.Timestamptz `json:"schedule_time"`
}

type NotificationCache interface {
	GetDataByProjectID(ctx context.Context, pid string) (*string, *time.Time, error)
	UpdateProjectData(ctx context.Context, pid string, data string, t time.Time, expire time.Duration) error
}

type NotificationRepository interface {
	GetByID(ctx context.Context, id string) (*Notification, error)
	GetByPID(ctx context.Context, pid string) ([]Notification, error)
	Insert(ctx context.Context, n *Notification) error
	Update(ctx context.Context, n *Notification) error
	Delete(ctx context.Context, n *Notification) error
	GetDataByPID(ctx context.Context, pid string) (*time.Time, *int32, *string, error)
}

package domain

import (
	"context"
	"time"
)

const (
	NOTIFICATION_STATUS_ACTIVE    = 1
	NOTIFICATION_STATUS_SCHEDULED = 2
	NOTIFICATION_STATUS_CANCELED  = 3
	NOTIFICATION_STATUS_FINISHED  = 4
)

type Notification struct {
	ID           int        `json:"id"`
	PID          string     `json:"pid"`
	Status       int        `json:"status"`
	Title        string     `json:"title"`
	Text         string     `json:"text"`
	BigText      *string    `json:"big-text,omitempty"`
	Image        *string    `json:"image,omitempty"`
	BigImage     *string    `json:"big-image,omitempty"`
	Priority     string     `json:"priority"`
	Style        string     `json:"style"`
	Action       *string    `json:"action,omitempty"`
	Extra        *string    `json:"extra,omitempty"`
	ViewsCount   int        `json:"views"`
	ClicksCount  int        `json:"clicks"`
	CreateTime   *time.Time `json:"create_time"`
	ActiveTime   *time.Time `json:"active_time"`
	ExpireTime   *time.Time `json:"expire_time"`
	ScheduleTime *time.Time `json:"schedule_time"`
	ClickReport  bool       `json:"click_report"`
}

type NotificationRepository interface {
	GetByID(ctx context.Context, id int) (*Notification, error)
	GetByPID(ctx context.Context, pid string, limit int, offset int) ([]Notification, error)
	Insert(ctx context.Context, n *Notification) error
	Update(ctx context.Context, n *Notification) error
	Delete(ctx context.Context, n *Notification) error
}

type NotificationCache interface {
	LoadScripts(ctx context.Context) error
	GetTimeByProjectID(ctx context.Context, pid string) (*time.Time, error)
	GetDataExistsByProjectID(ctx context.Context, pid string) (bool, error)
	GetDataByProjectID(ctx context.Context, pid string) (*string, error)
	GetViewsByProjectID(ctx context.Context, pid string) (string, error)
	UpdateProjectData(ctx context.Context, pid string, ids string, data string, t time.Time, expire time.Duration) error
	DeleteProjectData(ctx context.Context, pid string) error
	SetProjectDataExpire(ctx context.Context, pid string, expiration time.Duration) error
	GetClicksByProjectID(ctx context.Context, pid string) (map[string]string, error)
	IncrClicks(ctx context.Context, pid string, id string) error
	IncrClicksIds(ctx context.Context, pid string, ids []string) error
}

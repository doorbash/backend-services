package pg

import (
	"context"

	"github.com/doorbash/backend-services/api/domain"
	"github.com/jackc/pgx/v4/pgxpool"
)

type NotificationPostgresRepository struct {
	pool *pgxpool.Pool
}

func CreateNotifications() []string {
	return []string{
		`CREATE TABLE IF NOT EXISTS notifications
(
	id SERIAL NOT NULL PRIMARY KEY,
	pid VARCHAR(30) NOT NULL REFERENCES projects(id) ON DELETE CASCADE,
	status SMALLINT NOT NULL DEFAULT 1 CHECK (status IN (1, 2, 3, 4)),
	title VARCHAR(100) NOT NULL,
	text VARCHAR(200) NOT NULL,
	image VARCHAR(100),
	priority VARCHAR(7) NOT NULL DEFAULT 'default' CHECK(priority IN ('default', 'low', 'high', 'min', 'max')),
	style VARCHAR(20) NOT NULL DEFAULT 'normal' CHECK(style IN ('normal', 'big')),
	action VARCHAR(30),
	extra VARCHAR(200),
	views_count INTEGER DEFAULT 0,
	clicks_count INTEGER DEFAULT 0,
	create_time TIMESTAMP WITH TIME ZONE DEFAULT CURRENT_TIMESTAMP,
	active_time TIMESTAMP WITH TIME ZONE,
	expire_time TIMESTAMP WITH TIME ZONE,
	schedule_time TIMESTAMP WITH TIME ZONE
);`,
		`CREATE FUNCTION notifications_data(p VARCHAR(30))
RETURNS TABLE(_active_time TIMESTAMP WITH TIME ZONE, _ids TEXT, _data TEXT)
LANGUAGE 'plpgsql'

AS $BODY$
BEGIN
	RETURN QUERY
		SELECT
		MAX(active_time) AS active_time,
		STRING_AGG(id::TEXT, ' ' ORDER BY id ASC) AS ids,
		'[' || STRING_AGG(CONCAT('{"id":', id, ',"title":"', title, '","text":"', text, '","image":"', image, '","priority":"', priority, '","style":"', style, '","action":"', action, '","extra":"', extra, '","active_time":"', to_char((active_time::timestamp), 'YYYY-MM-DD"T"HH24:MI:SS"Z"'), '"}'), ',') || ']' AS data
		FROM notifications
		WHERE pid = $1 AND status = 1
		ORDER BY active_time ASC;
	END
$BODY$;`,
	}
}

func (n *NotificationPostgresRepository) GetByID(ctx context.Context, id int) (*domain.Notification, error) {
	row := n.pool.QueryRow(ctx, "SELECT id, pid, status, title, text, image, priority, style, action, extra, views_count, clicks_count, create_time, active_time, expire_time, schedule_time FROM notifications WHERE id = $1", id)
	notification := &domain.Notification{}
	if err := row.Scan(
		&notification.ID,
		&notification.PID,
		&notification.Status,
		&notification.Title,
		&notification.Text,
		&notification.Image,
		&notification.Priority,
		&notification.Style,
		&notification.Action,
		&notification.Extra,
		&notification.ViewsCount,
		&notification.ClicksCount,
		&notification.CreateTime,
		&notification.ActiveTime,
		&notification.ExpireTime,
		&notification.ScheduleTime,
	); err != nil {
		return nil, err
	}
	return notification, nil
}

func (n *NotificationPostgresRepository) GetByPID(ctx context.Context, pid string, limit int, offset int) ([]domain.Notification, error) {
	rows, err := n.pool.Query(ctx, "SELECT id, pid, status, title, text, image, priority, style, action, extra, views_count, clicks_count, create_time, active_time, expire_time, schedule_time FROM notifications WHERE pid = $1 ORDER BY create_time DESC LIMIT $2 OFFSET $3", pid, limit, offset)
	if err != nil {
		return nil, err
	}
	ret := make([]domain.Notification, 0)
	for rows.Next() {
		notification := domain.Notification{}
		err := rows.Scan(
			&notification.ID,
			&notification.PID,
			&notification.Status,
			&notification.Title,
			&notification.Text,
			&notification.Image,
			&notification.Priority,
			&notification.Style,
			&notification.Action,
			&notification.Extra,
			&notification.ViewsCount,
			&notification.ClicksCount,
			&notification.CreateTime,
			&notification.ActiveTime,
			&notification.ExpireTime,
			&notification.ScheduleTime,
		)
		if err != nil {
			return nil, err
		}
		ret = append(ret, notification)
	}
	return ret, nil
}

func (n *NotificationPostgresRepository) Insert(ctx context.Context, notification *domain.Notification) error {
	row := n.pool.QueryRow(
		ctx,
		"INSERT INTO notifications (pid, status, title, text, image, priority, style, action, extra, create_time, active_time, expire_time, schedule_time) VALUES ($1, $2, $3, $4, $5, $6, $7, $8, $9, $10, $11, $12, $13) RETURNING id, pid, status, title, text, image, priority, style, action, extra, create_time, active_time, expire_time, schedule_time",
		notification.PID,
		notification.Status,
		notification.Title,
		notification.Text,
		notification.Image,
		notification.Priority,
		notification.Style,
		notification.Action,
		notification.Extra,
		notification.CreateTime,
		notification.ActiveTime,
		notification.ExpireTime,
		notification.ScheduleTime,
	)
	return row.Scan(
		&notification.ID,
		&notification.PID,
		&notification.Status,
		&notification.Title,
		&notification.Text,
		&notification.Image,
		&notification.Priority,
		&notification.Style,
		&notification.Action,
		&notification.Extra,
		&notification.CreateTime,
		&notification.ActiveTime,
		&notification.ExpireTime,
		&notification.ScheduleTime,
	)
}

func (n *NotificationPostgresRepository) Update(ctx context.Context, notification *domain.Notification) error {
	_, err := n.pool.Exec(
		ctx,
		"UPDATE notifications SET status = $1, title = $2, text = $3, image = $4, priority = $5, style = $6, action = $7, extra = $8, active_time = $9, expire_time = $10, schedule_time = $11 WHERE id = $12",
		notification.Status,
		notification.Title,
		notification.Text,
		notification.Image,
		notification.Priority,
		notification.Style,
		notification.Action,
		notification.Extra,
		notification.ActiveTime,
		notification.ExpireTime,
		notification.ScheduleTime,
		notification.ID,
	)
	return err
}

func (n *NotificationPostgresRepository) Delete(ctx context.Context, notification *domain.Notification) error {
	_, err := n.pool.Exec(ctx, "DELETE FROM notifications WHERE id = $1", notification.ID)
	return err
}

func NewNotificationPostgresRepository(pool *pgxpool.Pool) *NotificationPostgresRepository {
	return &NotificationPostgresRepository{
		pool: pool,
	}
}

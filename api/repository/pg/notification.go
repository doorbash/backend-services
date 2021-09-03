package pg

import (
	"context"
	"errors"
	"time"

	"github.com/doorbash/backend-services/api/domain"
	"github.com/jackc/pgtype"
	"github.com/jackc/pgx/v4/pgxpool"
)

type NotificationPostgresRepository struct {
	pool *pgxpool.Pool
}

func CreateNotificationsTable() (query string) {
	return `CREATE TABLE IF NOT EXISTS notifications
(
	id SERIAL NOT NULL PRIMARY KEY,
	pid VARCHAR(30) NOT NULL REFERENCES projects(id) ON DELETE CASCADE,
	status SMALLINT NOT NULL DEFAULT 1 CHECK (status IN (0, 1, 2, 3, 4)),
	title VARCHAR(100) NOT NULL,
	text VARCHAR(200) NOT NULL,
	create_time TIMESTAMP WITH TIME ZONE DEFAULT CURRENT_TIMESTAMP,
	active_time TIMESTAMP WITH TIME ZONE,
	expire_time TIMESTAMP WITH TIME ZONE,
	schedule_time TIMESTAMP WITH TIME ZONE

);`
}

func (n *NotificationPostgresRepository) GetByID(ctx context.Context, id int) (*domain.Notification, error) {
	row := n.pool.QueryRow(ctx, "SELECT id, pid, status, title, text, create_time, active_time, expire_time, schedule_time FROM notifications WHERE id = $1", id)
	notification := &domain.Notification{}
	if err := row.Scan(
		&notification.ID,
		&notification.PID,
		&notification.Status,
		&notification.Title,
		&notification.Text,
		&notification.CreateTime,
		&notification.ActiveTime,
		&notification.ExpireTime,
		&notification.ScheduleTime,
	); err != nil {
		return nil, err
	}
	return notification, nil
}

func (n *NotificationPostgresRepository) GetByPID(ctx context.Context, pid string) ([]domain.Notification, error) {
	rows, err := n.pool.Query(ctx, "SELECT id, pid, status, title, text, create_time, active_time, expire_time, schedule_time FROM notifications WHERE pid = $1 ORDER BY create_time ASC", pid)
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
		"INSERT INTO notifications (pid, status, title, text, create_time, active_time, expire_time, schedule_time) VALUES ($1, $2, $3, $4, $5, $6, $7, $8) RETURNING id, pid, status, title, text, create_time, active_time, expire_time, schedule_time",
		notification.PID,
		notification.Status,
		notification.Title,
		notification.Text,
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
		&notification.CreateTime,
		&notification.ActiveTime,
		&notification.ExpireTime,
		&notification.ScheduleTime,
	)
}

func (n *NotificationPostgresRepository) Update(ctx context.Context, notification *domain.Notification) error {
	_, err := n.pool.Exec(
		ctx,
		"UPDATE notifications SET status = $1, title = $2, text = $3, active_time = $4, expire_time = $5, schedule_time = $6 WHERE id = $7",
		notification.Status,
		notification.Title,
		notification.Text,
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

func (n *NotificationPostgresRepository) GetDataByPID(ctx context.Context, pid string) (*time.Time, *int32, *string, error) {
	row := n.pool.QueryRow(ctx, `WITH schedules AS (SELECT MIN(schedule_time) AS schedule_min FROM notifications WHERE pid = $1 AND status = 2)
	SELECT
		MAX(active_time) AS active_time,
		EXTRACT(EPOCH FROM LEAST(MIN(expire_time), (select schedule_min from schedules)) - CURRENT_TIMESTAMP)::int AS expire,
		'[' || STRING_AGG('{"id":' || id || ',"active_time":"' || active_time || '","title":"' || title || '","text":"' || text || '"}', ',') || ']' AS data
		FROM notifications
		WHERE pid = $1 AND status = 1`, pid)
	var activeTime pgtype.Timestamptz
	var expire pgtype.Int4
	var data pgtype.Text
	err := row.Scan(
		&activeTime,
		&expire,
		&data,
	)
	if err != nil {
		return nil, nil, nil, err
	}
	if activeTime.Status == pgtype.Null {
		return nil, nil, nil, errors.New("active_time is null")
	}
	if expire.Status == pgtype.Null {
		return nil, nil, nil, errors.New("expire is null")
	}
	if data.Status == pgtype.Null {
		return nil, nil, nil, errors.New("data is null")
	}
	return &activeTime.Time, &expire.Int, &data.String, nil
}

func NewNotificationPostgresRepository(pool *pgxpool.Pool) *NotificationPostgresRepository {
	return &NotificationPostgresRepository{
		pool: pool,
	}
}

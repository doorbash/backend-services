package pg

import (
	"context"

	"github.com/doorbash/backend-services/api/domain"
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
	created_at TIMESTAMP WITH TIME ZONE DEFAULT CURRENT_TIMESTAMP,
	actived_at TIMESTAMP WITH TIME ZONE,
	expires_at TIMESTAMP WITH TIME ZONE

);`
}

func (n *NotificationPostgresRepository) GetByID(ctx context.Context, id string) (*domain.Notification, error) {
	row := n.pool.QueryRow(ctx, "SELECT id, pid, status, title, text, created_at, actived_at, expires_at FROM notifications WHERE id = $1", id)
	notification := &domain.Notification{}
	if err := row.Scan(
		&notification.ID,
		&notification.PID,
		&notification.Status,
		&notification.Title,
		&notification.Text,
		&notification.CreatedAt,
		&notification.ActivedAt,
		&notification.ExpiresAt,
	); err != nil {
		return nil, err
	}
	return notification, nil
}

func (n *NotificationPostgresRepository) GetByPID(ctx context.Context, pid string) ([]domain.Notification, error) {
	rows, err := n.pool.Query(ctx, "SELECT id, pid, status, title, text, created_at, actived_at, expires_at FROM notifications WHERE pid = $1", pid)
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
			&notification.CreatedAt,
			&notification.ActivedAt,
			&notification.ExpiresAt,
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
		"INSERT INTO notifications (pid, status, title, text, created_at, actived_at, expires_at) VALUES ($1, $2, $3, $4, $5, $6, $7) RETURNING id, pid, status, title, text, created_at, actived_at, expires_at",
		notification.PID,
		notification.Status,
		notification.Title,
		notification.Text,
		notification.CreatedAt,
		notification.ActivedAt,
		notification.ExpiresAt,
	)
	return row.Scan(
		&notification.ID,
		&notification.PID,
		&notification.Status,
		&notification.Title,
		&notification.Text,
		&notification.CreatedAt,
		&notification.ActivedAt,
		&notification.ExpiresAt,
	)
}

func (n *NotificationPostgresRepository) Update(ctx context.Context, notification *domain.Notification) error {
	_, err := n.pool.Exec(
		ctx,
		"UPDATE notifications SET status = $1, title = $2, text = $3, actived_at = $4, expires_at = $5 WHERE id = $6",
		notification.Status,
		notification.Title,
		notification.Text,
		notification.ActivedAt,
		notification.ExpiresAt,
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

package pg

import (
	"context"

	"github.com/doorbash/backend-services-api/api/domain"
	"github.com/jackc/pgx/v4/pgxpool"
)

type RemoteConfigPostgressRepository struct {
	pool *pgxpool.Pool
}

func CreateRemoteConfigTable() (query string) {
	return `CREATE TABLE IF NOT EXISTS remote_config
(
	pid VARCHAR(30) NOT NULL PRIMARY KEY REFERENCES projects(id) ON DELETE CASCADE,
	data JSON NOT NULL
);`
}

func (rc *RemoteConfigPostgressRepository) GetByProjectID(ctx context.Context, pid string) (*domain.RemoteConfig, error) {
	row := rc.pool.QueryRow(ctx, "SELECT pid, data FROM remote_config WHERE pid = $1", pid)
	remoteConfig := domain.RemoteConfig{}
	if err := row.Scan(&remoteConfig.ProjectID, &remoteConfig.Data); err != nil {
		return nil, err
	}
	return &remoteConfig, nil
}

func (rc *RemoteConfigPostgressRepository) Insert(ctx context.Context, remoteConfig *domain.RemoteConfig) error {
	_, err := rc.pool.Exec(ctx, "INSERT INTO remote_config (pid, data) VALUES ($1, $2)", remoteConfig.ProjectID, remoteConfig.Data)
	return err
}

func (rc *RemoteConfigPostgressRepository) Update(ctx context.Context, remoteConfig *domain.RemoteConfig) error {
	_, err := rc.pool.Exec(ctx, "UPDATE remote_config SET data = $1 WHERE pid = $2", remoteConfig.Data, remoteConfig.ProjectID)
	return err
}

func NewRemoteConfigPostgressRepository(pool *pgxpool.Pool) *RemoteConfigPostgressRepository {
	return &RemoteConfigPostgressRepository{
		pool: pool,
	}
}

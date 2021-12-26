package pg

import (
	"context"

	"github.com/doorbash/backend-services/api/domain"
	"github.com/jackc/pgx/v4/pgxpool"
)

type RemoteConfigPostgresRepository struct {
	pool *pgxpool.Pool
}

func CreateRemoteConfigTable() string {
	return `CREATE TABLE IF NOT EXISTS remote_configs
(
	pid VARCHAR(30) NOT NULL PRIMARY KEY REFERENCES projects(id) ON DELETE CASCADE,
	data JSON NOT NULL,
	version INTEGER DEFAULT 1,
	modified BOOLEAN DEFAULT TRUE
);`
}

func (rc *RemoteConfigPostgresRepository) GetByProjectID(ctx context.Context, pid string) (*domain.RemoteConfig, error) {
	row := rc.pool.QueryRow(ctx, "SELECT pid, data, version FROM remote_configs WHERE pid = $1", pid)
	remoteConfig := domain.RemoteConfig{}
	if err := row.Scan(
		&remoteConfig.ProjectID,
		&remoteConfig.Data,
		&remoteConfig.Version,
	); err != nil {
		return nil, err
	}
	return &remoteConfig, nil
}

func (rc *RemoteConfigPostgresRepository) Insert(ctx context.Context, remoteConfig *domain.RemoteConfig) error {
	row := rc.pool.QueryRow(ctx, "INSERT INTO remote_configs (pid, data) VALUES ($1, $2) RETURNING version", remoteConfig.ProjectID, remoteConfig.Data)
	return row.Scan(&remoteConfig.Version)
}

func (rc *RemoteConfigPostgresRepository) Update(ctx context.Context, remoteConfig *domain.RemoteConfig) error {
	row := rc.pool.QueryRow(
		ctx,
		"UPDATE remote_configs SET data = $1, version = version + 1, modified = TRUE WHERE pid = $2 RETURNING version",
		remoteConfig.Data,
		remoteConfig.ProjectID,
	)
	return row.Scan(
		&remoteConfig.Version,
	)
}

func NewRemoteConfigPostgresRepository(pool *pgxpool.Pool) *RemoteConfigPostgresRepository {
	return &RemoteConfigPostgresRepository{
		pool: pool,
	}
}

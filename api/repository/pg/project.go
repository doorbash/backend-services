package pg

import (
	"context"

	"github.com/doorbash/backend-services/api/domain"
	"github.com/jackc/pgx/v4"
	"github.com/jackc/pgx/v4/pgxpool"
)

type ProjectPostgresRepository struct {
	pool *pgxpool.Pool
}

func CreateProjectTable() string {
	return `CREATE TABLE IF NOT EXISTS projects
(
	id VARCHAR(30) NOT NULL PRIMARY KEY CHECK (id ~ '^[A-Za-z0-9._-]+'),
	uid INTEGER NOT NULL REFERENCES users(id) ON DELETE CASCADE,
	name VARCHAR(200) NOT NULL
);`
}

func (rc *ProjectPostgresRepository) GetByID(ctx context.Context, id string) (*domain.Project, error) {
	row := rc.pool.QueryRow(ctx, "SELECT id, uid, name FROM projects WHERE id = $1", id)
	project := domain.Project{}
	if err := row.Scan(&project.ID, &project.UserID, &project.Name); err != nil {
		return nil, err
	}
	return &project, nil
}

func (rc *ProjectPostgresRepository) GetProjectsByUserID(ctx context.Context, uid int) ([]domain.Project, error) {
	rows, err := rc.pool.Query(ctx, "SELECT id, uid, name FROM projects WHERE uid = $1 ORDER BY id ASC", uid)
	if err != nil {
		return nil, err
	}
	ret := make([]domain.Project, 0)
	for rows.Next() {
		project := domain.Project{}
		rows.Scan(&project.ID, &project.UserID, &project.Name)
		ret = append(ret, project)
	}
	return ret, nil
}

func (pr *ProjectPostgresRepository) Insert(ctx context.Context, project *domain.Project) error {
	tx, err := pr.pool.Begin(ctx)
	if err != nil {
		return err
	}
	_, err = tx.Exec(ctx, "INSERT INTO projects (id, uid, name) VALUES ($1, $2, $3)", project.ID, project.UserID, project.Name)
	if err != nil {
		return err
	}
	_, err = tx.Exec(ctx, "UPDATE users SET num_projects = (SELECT COUNT(*) FROM projects WHERE uid = $1) WHERE id = $1", project.UserID)
	if err != nil {
		return err
	}
	return tx.Commit(ctx)
}

func (pr *ProjectPostgresRepository) Update(ctx context.Context, project *domain.Project) error {
	_, err := pr.pool.Exec(ctx, "UPDATE projects SET name = $1 WHERE id = $2 AND uid = $3", project.Name, project.ID, project.UserID)
	return err
}

func (pr *ProjectPostgresRepository) Delete(ctx context.Context, project *domain.Project) error {
	tx, err := pr.pool.Begin(ctx)
	if err != nil {
		return err
	}
	result, err := tx.Exec(ctx, "DELETE FROM projects WHERE id = $1 AND uid = $2", project.ID, project.UserID)
	if err != nil {
		return err
	}
	if result.RowsAffected() == 0 {
		return pgx.ErrNoRows
	}
	_, err = tx.Exec(ctx, "UPDATE users SET num_projects = (SELECT COUNT(*) FROM projects WHERE uid = $1) WHERE id = $1", project.UserID)
	if err != nil {
		return err
	}
	return tx.Commit(ctx)
}

func NewProjectPostgresRepository(pool *pgxpool.Pool) *ProjectPostgresRepository {
	return &ProjectPostgresRepository{
		pool: pool,
	}
}

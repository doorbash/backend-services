package pg

import (
	"context"
	"fmt"

	"github.com/doorbash/backend-services-api/api/domain"
	"github.com/jackc/pgx/v4/pgxpool"
)

type UserPostgressRepository struct {
	pool *pgxpool.Pool
}

func CreateUserTable() string {
	return `CREATE TABLE IF NOT EXISTS users
(
	id SERIAL NOT NULL PRIMARY KEY,
	email VARCHAR(200) NOT NULL UNIQUE CHECK (email ~ '^[A-Za-z0-9._%-]+@[A-Za-z0-9.-]+[.][A-Za-z]+$'),
	project_quota INTEGER NOT NULL DEFAULT 0 CHECK(project_quota >= 0),
	num_projects INTEGER NOT NULL DEFAULT 0 CHECK(num_projects >= 0) CHECK(project_quota = 0 OR project_quota >= num_projects)
);`
}

func (u *UserPostgressRepository) GetByEmail(ctx context.Context, email string) (*domain.User, error) {
	row := u.pool.QueryRow(ctx, "SELECT id, email, project_quota, num_projects FROM users WHERE email = $1", email)
	User := domain.User{}
	if err := row.Scan(&User.ID, &User.Email, &User.ProjectQuota, &User.NumProjects); err != nil {
		return nil, err
	}
	return &User, nil
}

func (u *UserPostgressRepository) Insert(ctx context.Context, user *domain.User) error {
	cmd, err := u.pool.Exec(ctx, "INSERT INTO users(email, project_quota) VALUES($1, $2)", user.Email, user.ProjectQuota)
	if err != nil {
		return err
	}
	if cmd.RowsAffected() != 1 {
		return fmt.Errorf("RowsAffected() = %d", cmd.RowsAffected())
	}
	return nil
}

func (u *UserPostgressRepository) Update(ctx context.Context, user *domain.User) error {
	_, err := u.pool.Exec(ctx, "UPDATE users SET project_quota = $1 WHERE id = $2", user.ProjectQuota, user.ID)
	if err != nil {
		return err
	}
	return nil
}

func NewUserPostgressRepository(pool *pgxpool.Pool) *UserPostgressRepository {
	return &UserPostgressRepository{
		pool: pool,
	}
}

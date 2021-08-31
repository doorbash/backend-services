package pg

import (
	"context"

	"github.com/doorbash/backend-services/api/domain"
	"github.com/jackc/pgx/v4/pgxpool"
)

type UserPostgresRepository struct {
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

func (u *UserPostgresRepository) GetByEmail(ctx context.Context, email string) (*domain.User, error) {
	row := u.pool.QueryRow(ctx, "SELECT id, email, project_quota, num_projects FROM users WHERE email = $1", email)
	User := domain.User{}
	if err := row.Scan(&User.ID, &User.Email, &User.ProjectQuota, &User.NumProjects); err != nil {
		return nil, err
	}
	return &User, nil
}

func (u *UserPostgresRepository) Insert(ctx context.Context, user *domain.User) (int, error) {
	row := u.pool.QueryRow(ctx, "INSERT INTO users(email, project_quota) VALUES($1, $2) RETURNING id", user.Email, user.ProjectQuota)
	var id int
	err := row.Scan(&id)
	return id, err
}

func (u *UserPostgresRepository) Update(ctx context.Context, user *domain.User) error {
	_, err := u.pool.Exec(ctx, "UPDATE users SET project_quota = $1 WHERE id = $2", user.ProjectQuota, user.ID)
	if err != nil {
		return err
	}
	return nil
}

func NewUserPostgresRepository(pool *pgxpool.Pool) *UserPostgresRepository {
	return &UserPostgresRepository{
		pool: pool,
	}
}

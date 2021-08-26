package domain

import (
	"context"
)

type User struct {
	ID           int    `json:"id"`
	Email        string `json:"email"`
	ProjectQuota int    `json:"project_quota"`
	NumProjects  int    `json:"num_projects"`
}

type UserRepository interface {
	GetByEmail(ctx context.Context, email string) (*User, error)
	Insert(ctx context.Context, user *User) error
	Update(ctx context.Context, user *User) error
}

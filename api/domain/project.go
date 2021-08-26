package domain

import (
	"context"
)

type Project struct {
	ID     string `json:"id"`
	UserID int    `json:"uid"`
	Name   string `json:"name"`
}

type ProjectRepository interface {
	GetByID(ctx context.Context, id string) (*Project, error)
	GetProjectsByUserID(ctx context.Context, uid int) ([]Project, error)
	Insert(ctx context.Context, project *Project) error
	Update(ctx context.Context, project *Project) error
	Delete(ctx context.Context, project *Project) error
}

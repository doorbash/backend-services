package domain

import (
	"context"
)

type RemoteConfig struct {
	ProjectID string `json:"pid"`
	Data      string `json:"data"`
}

type RemoteConfigRepository interface {
	GetByProjectID(ctx context.Context, pid string) (*RemoteConfig, error)
	Insert(ctx context.Context, rc *RemoteConfig) error
	Update(ctx context.Context, rc *RemoteConfig) error
}

type RemoteConfigCache interface {
	GetDataByProjectID(ctx context.Context, id string) (string, error)
	Update(ctx context.Context, rc *RemoteConfig) error
}

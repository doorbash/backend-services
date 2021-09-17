package domain

import (
	"context"
	"encoding/json"
)

type RemoteConfig struct {
	ProjectID string
	Data      string
	Version   int
}

func (r *RemoteConfig) MarshalJSON() ([]byte, error) {
	ret := make(map[string]interface{})
	ret["version"] = r.Version
	ret["data"] = json.RawMessage(r.Data)
	b, err := json.Marshal(ret)
	if err != nil {
		return nil, err
	}
	return b, nil
}

type RemoteConfigRepository interface {
	GetByProjectID(ctx context.Context, pid string) (*RemoteConfig, error)
	Insert(ctx context.Context, rc *RemoteConfig) error
	Update(ctx context.Context, rc *RemoteConfig) error
}

type RemoteConfigCache interface {
	LoadScripts(ctx context.Context) error
	GetDataByProjectID(ctx context.Context, pid string) (*string, error)
	GetVersionByProjectID(ctx context.Context, pid string) (*int, error)
	Update(ctx context.Context, rc *RemoteConfig) error
}

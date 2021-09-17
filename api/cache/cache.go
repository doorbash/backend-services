package cache

import "context"

type Cache interface {
	LoadScripts(ctx context.Context) error
}

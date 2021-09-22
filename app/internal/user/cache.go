package user

import (
	"context"
)

type Cache interface {
	Get(ctx context.Context, id string) (u User, err error)
	Set(ctx context.Context, u User) error
	PingClient(ctx context.Context) error
	Close() error
}
package user

import (
	"context"
)

type Cache interface {
	Get(ctx context.Context, id string) (u User, err error)
	Set(ctx context.Context, u User) error
	Del(ctx context.Context, id string) error
	SetByNickname(ctx context.Context, u User) error
	Expire(ctx context.Context, id string) error
	PingClient(ctx context.Context) error
	Close() error
	KeepAlive()
}

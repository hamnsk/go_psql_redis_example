package user

import (
	"context"
)

type Cache interface {
	Get(ctx context.Context, id string) (u User, err error)
	GetAll(ctx context.Context, key string) (users []User, err error)
	Set(ctx context.Context, u User) error
	SetAll(ctx context.Context, key string, val []User) error
	Del(ctx context.Context, id string) error
	SetByNickname(ctx context.Context, u User) error
	Expire(ctx context.Context, id string) error
	ExpireAll(ctx context.Context, key string) error
	PingClient(ctx context.Context) error
	Close() error
	KeepAlive()
}

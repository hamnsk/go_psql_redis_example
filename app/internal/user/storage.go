package user

import "context"

type Storage interface {
	GetByID(id string) (u User, err error)
	FindOneByNickName(nickname string) (u User, err error)
	PingPool(ctx context.Context) error
	Close()
	KeepAlive()
}

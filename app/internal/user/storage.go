package user

import "context"

type Storage interface {
	FindOne(id string) (u User, err error)
	PingPool(ctx context.Context) error
}
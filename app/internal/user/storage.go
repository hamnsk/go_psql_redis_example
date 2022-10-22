package user

import "context"

type Storage interface {
	FindOneByNickName(nickname string) (u User, err error)
	PingPool(ctx context.Context) error
	Close()
	KeepAlive()
	Create(u *User) error
	FindAll(limit, offset int64) (users []User, err error)
	FindOne(id string) (User, error)
	Update(u *User) error
	Delete(id string) error
}

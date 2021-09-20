package user

type Storage interface {
	FindOne(id string) (u User, err error)
}
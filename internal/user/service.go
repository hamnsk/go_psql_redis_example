package user

import (
	"context"
	"fmt"
	"redis/pkg/logging"
)

var _ Service = &service{}

type service struct {
	storage Storage
	cache Cache
	logger logging.Logger
}

type Service interface {
	getByID(id string) (u User, err error)
	error(msg string)
	info(msg string)
}

func NewService(userStorage Storage, userCache Cache, appLogger logging.Logger) (Service, error) {
	return &service{
		storage: userStorage,
		cache: userCache,
		logger: appLogger,
	}, nil
}

func (s service) getByID(id string) (u User, err error) {
	var cstatus string
	defer trace(s.logger, id, &cstatus)()
	u, err = s.cache.Get(context.Background(), id)
	if err == nil {
		//s.logger.Debug("Cache hit for user id: " + id)
		cstatus = "HIT"
		return u, nil
	}
	cstatus = "MISS"
	//s.logger.Debug("Cache miss for user id: " + id)
	u, err = s.storage.FindOne(id)
	if err != nil {
		return User{}, fmt.Errorf("failed to get user by id=%s. error: %w", id, err)
	}
	_ = s.cache.Set(context.Background(), u)
	return u, nil
}

func (s service) error(msg string) {
	s.logger.Error(msg)
}

func (s service) info(msg string) {
	s.logger.Info(msg)
}
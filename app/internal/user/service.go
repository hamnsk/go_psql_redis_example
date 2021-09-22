package user

import (
	"context"
	"fmt"
	"github.com/getsentry/sentry-go"
	"github.com/prometheus/client_golang/prometheus"
	"redis/pkg/logging"
	"time"
)

var _ Service = &service{}

type service struct {
	storage Storage
	cache Cache
	logger logging.Logger
}

type Service interface {
	getByID(id string) (u User, err error)
	error(err error)
	info(err error)
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
	timer := prometheus.NewTimer(userGetDuration.WithLabelValues(id))
	defer timer.ObserveDuration()
	defer trace(s.logger, id, &cstatus)()
	u, err = s.cache.Get(context.Background(), id)
	if err == nil {
		s.logger.Debug("Cache hit for user id: " + id)
		cstatus = "HIT"
		return u, nil
	}
	cstatus = "MISS"
	s.logger.Debug("Cache miss for user id: " + id)
	u, err = s.storage.FindOne(id)
	if err != nil {
		return User{}, fmt.Errorf("failed to get user by id=%s. error: %w", id, err)
	}
	_ = s.cache.Set(context.Background(), u)
	return u, nil
}

func (s service) error(err error) {
	sentry.CaptureException(err)
	sentry.Flush(time.Second * 1)
	s.logger.Error(err.Error())
}

func (s service) info(err error) {
	s.logger.Info(err.Error())
}
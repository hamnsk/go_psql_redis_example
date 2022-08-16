package user

import (
	"context"
	"fmt"
	"github.com/getsentry/sentry-go"
	"github.com/opentracing/opentracing-go"
	"github.com/prometheus/client_golang/prometheus"
	"redis/pkg/logging"
	"sync"
	"time"
)

var _ Service = &service{}

type service struct {
	storage Storage
	cache   Cache
	logger  logging.Logger
	tracer  opentracing.Tracer
	mu      *sync.RWMutex
}

type Service interface {
	getByID(id string) (u User, err error)
	findByNickname(nickname string) (u User, err error)
	getTracer() (t opentracing.Tracer)
	error(err error)
	info(err error)
}

func NewService(userStorage Storage, userCache Cache, appLogger logging.Logger, appTracer opentracing.Tracer) (Service, error) {
	return &service{
		storage: userStorage,
		cache:   userCache,
		logger:  appLogger,
		tracer:  appTracer,
		mu:      new(sync.RWMutex),
	}, nil
}

func (s service) getByID(id string) (u User, err error) {
	var cstatus string
	//s.mu.Lock()
	//defer s.mu.Unlock()
	timer := prometheus.NewTimer(userGetDuration.WithLabelValues(id))
	// register time for all operations steps
	defer timer.ObserveDuration()
	// log time duration for all operations steps without lock/unlock mutex and init prometheus metrics (clean time for get entity)
	defer trace(s.logger, id, &cstatus)()
	s.mu.RLock()
	u, err = s.cache.Get(context.Background(), id)
	s.mu.RUnlock()
	if err == nil {
		s.logger.Debug("Cache hit for user id: " + id)
		cstatus = "HIT"
		// after success get user from cache refresh expire time for him
		defer func() {
			err := s.cache.Expire(context.Background(), id)
			if err != nil {
				s.logger.Error("Set cache expiration failed for user id: " + id)
				s.error(err)
			}
		}()
		return u, nil
	}
	cstatus = "MISS"
	s.logger.Debug("Cache miss for user id: " + id)
	u, err = s.storage.GetByID(id)
	if err != nil {
		return User{}, fmt.Errorf("failed to get user by id=%s. error: %w", id, err)
	}
	// after get user from storage place him to cache with ttl
	defer func() {
		s.mu.Lock()
		err = s.cache.Set(context.Background(), u)
		s.mu.Unlock()
		if err != nil {
			s.logger.Error(err.Error())
		}
		s.logger.Debug("Write to cache user by id: " + id)
	}()
	return u, nil
}

func (s service) findByNickname(nickname string) (u User, err error) {
	var cstatus string
	s.mu.Lock()
	defer s.mu.Unlock()
	timer := prometheus.NewTimer(userGetDuration.WithLabelValues(nickname))
	// register time for all operations steps
	defer timer.ObserveDuration()
	// log time duration for all operations steps without lock/unlock mutex and init prometheus metrics (clean time for get entity)
	defer trace(s.logger, nickname, &cstatus)()
	u, err = s.cache.Get(context.Background(), nickname)
	if err == nil {
		cstatus = "HIT"
		// after success get user from cache refresh expire time for him
		defer func() {
			err := s.cache.Expire(context.Background(), nickname)
			if err != nil {
				s.logger.Error("Set cache expiration failed for user id: " + nickname)
				s.error(err)
			}
		}()
		return u, nil
	}
	cstatus = "MISS"
	s.logger.Debug("Cache miss for user id: " + nickname)
	u, err = s.storage.FindOneByNickName(nickname)
	if err != nil {
		return User{}, fmt.Errorf("failed to get user by id=%s. error: %w", nickname, err)
	}
	// after get user from storage place him to cache with ttl
	defer func() {
		_ = s.cache.SetByNickname(context.Background(), u)
	}()
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

func (s service) getTracer() (t opentracing.Tracer) {
	return s.tracer
}

package user

import (
	"context"
	"fmt"
	"github.com/getsentry/sentry-go"
	"github.com/opentracing/opentracing-go"
	"github.com/opentracing/opentracing-go/ext"
	"golang.org/x/sync/singleflight"
	"redis/pkg/logging"
)

var _ Service = &service{}

type service struct {
	storage Storage
	cache   Cache
	logger  logging.Logger
	tracer  opentracing.Tracer
	sflight *singleflight.Group
}

type Service interface {
	getByID(id string, spanCtx opentracing.SpanContext) (u User, err error)
	findByNickname(nickname string) (u User, err error)
	getTracer() (t opentracing.Tracer)
	getSingleFlightGroup() (sfg *singleflight.Group)
	error(err error)
	info(msg string)
}

func NewService(userStorage Storage, userCache Cache, appLogger logging.Logger, appTracer opentracing.Tracer) (Service, error) {
	return &service{
		storage: userStorage,
		cache:   userCache,
		logger:  appLogger,
		tracer:  appTracer,
		sflight: &singleflight.Group{},
	}, nil
}

func (s service) getByID(id string, spanCtx opentracing.SpanContext) (u User, err error) {
	getByIDSpan := s.tracer.StartSpan("get-by-id-service-call", ext.RPCServerOption(spanCtx))
	defer getByIDSpan.Finish()
	var cstatus string
	defer trace(s.logger, id, &cstatus)()
	getFromCacheSpan := s.tracer.StartSpan("get-user-from-cache", ext.RPCServerOption(spanCtx))
	u, err = s.cache.Get(context.Background(), id)
	if err == nil {
		s.logger.Debug("Cache hit for user id: " + id)
		cstatus = "HIT"
		// after success get user from cache refresh expire time for him
		defer func() {
			invalidateCacheSpan := s.tracer.StartSpan("invalidate-cache", ext.RPCServerOption(spanCtx))
			err := s.cache.Expire(context.Background(), id)
			if err != nil {
				s.logger.Error("Set cache expiration failed for user id: " + id)
				s.error(err)
			}
			invalidateCacheSpan.Finish()
		}()
		getFromCacheSpan.Finish()
		return u, nil
	}

	getFromCacheSpan.Finish()

	cstatus = "MISS"
	s.logger.Debug("Cache miss for user id: " + id)
	getFromStorageSpan := s.tracer.StartSpan("get-user-from-storage", ext.RPCServerOption(spanCtx))
	u, err = s.storage.GetByID(id)
	if err != nil {
		getFromStorageSpan.Finish()
		return User{}, fmt.Errorf("failed to get user by id=%s. error: %w", id, err)
	}
	// after get user from storage place him to cache with ttl
	defer func() {
		setToCacheSpan := s.tracer.StartSpan("set-user-to-cache", ext.RPCServerOption(spanCtx))
		err = s.cache.Set(context.Background(), u)
		if err != nil {
			s.logger.Error(err.Error())
		}
		s.logger.Debug("Write to cache user by id: " + id)
		setToCacheSpan.Finish()
	}()
	getFromStorageSpan.Finish()
	return u, nil
}

func (s service) findByNickname(nickname string) (u User, err error) {
	var cstatus string
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
	// TODO: disable flush migrate to syncHTTPTransport https://docs.sentry.io/platforms/go/guides/http/configuration/transports/
	//sentry.Flush(time.Second * 1)
	s.logger.Error(err.Error())
}

func (s service) info(msg string) {
	s.logger.Info(msg)
}

func (s service) getTracer() (t opentracing.Tracer) {
	return s.tracer
}

func (s service) getSingleFlightGroup() (sfg *singleflight.Group) {
	return s.sflight
}

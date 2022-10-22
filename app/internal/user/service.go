package user

import (
	"context"
	"fmt"
	"github.com/getsentry/sentry-go"
	"github.com/prometheus/client_golang/prometheus"
	tracesdk "go.opentelemetry.io/otel/sdk/trace"
	otrace "go.opentelemetry.io/otel/trace"
	"golang.org/x/sync/singleflight"
	"redis/pkg/logging"
	"strconv"
)

var _ Service = &service{}

type service struct {
	storage Storage
	cache   Cache
	logger  logging.Logger
	tracer  *tracesdk.TracerProvider
	sflight *singleflight.Group
}

type Service interface {
	findOne(id string, ctx context.Context) (u User, err error)
	findAll(limit, offset int64, ctx context.Context) (users []User, nextCursor int64, err error)
	create(u *User, ctx context.Context) error
	delete(id string, ctx context.Context) error
	update(u *User, ctx context.Context) error
	getByID(id string, ctx context.Context) (u User, err error)
	findByNickname(nickname string, ctx context.Context) (u User, err error)
	getTracer() (t *tracesdk.TracerProvider)
	getSingleFlightGroup() (sfg *singleflight.Group)
	error(err error)
	info(msg string)
}

func NewService(userStorage Storage, userCache Cache, appLogger logging.Logger, appTracer *tracesdk.TracerProvider) (Service, error) {
	return &service{
		storage: userStorage,
		cache:   userCache,
		logger:  appLogger,
		tracer:  appTracer,
		sflight: &singleflight.Group{},
	}, nil
}

// Find One User by ID
func (s *service) findOne(id string, ctx context.Context) (u User, err error) {
	opts := []otrace.SpanStartOption{
		otrace.WithSpanKind(otrace.SpanKindServer),
	}

	tr := s.tracer.Tracer("Service.findOne")
	parentCtx, span := tr.Start(ctx, "GetUserById", opts...)
	defer span.End()
	var cstatus string

	traceId := span.SpanContext().TraceID().String()

	timer := prometheus.NewTimer(userGetDuration.WithLabelValues(id))
	// register time for all operations steps
	defer timer.ObserveDuration()
	// log time duration for all operations steps without lock/unlock mutex and init prometheus metrics (clean time for get entity)
	defer trace(s.logger, id, &cstatus, traceId)()
	parentCacheCtx, getFromCacheSpan := tr.Start(parentCtx, "getFromCache", opts...)
	u, err = s.cache.Get(context.Background(), id)
	if err == nil {
		s.logger.Debug("Cache hit for user id: " + id)
		cstatus = "HIT"
		// after success get user from cache refresh expire time for him
		defer func() {
			_, setExpireInCache := tr.Start(parentCacheCtx, "setCacheExpiration", opts...)

			err := s.cache.Expire(context.Background(), id)
			if err != nil {
				s.logger.Error("Set cache expiration failed for user id: " + id)
				s.error(err)
				setExpireInCache.End()
			}
			setExpireInCache.End()
		}()
		getFromCacheSpan.End()
		return u, nil
	}
	getFromCacheSpan.End()

	cstatus = "MISS"
	s.logger.Debug("Cache miss for user id: " + id)
	parentDBCtx, getFromDBSpan := tr.Start(parentCtx, "getFromDB", opts...)
	u, err = s.storage.GetByID(id)
	if err != nil {
		return User{}, fmt.Errorf("failed to get user by id=%s. error: %w", id, err)
	}
	// after get user from storage place him to cache with ttl
	defer func() {
		_, setInCacheSpan := tr.Start(parentDBCtx, "setInCache", opts...)
		err = s.cache.Set(context.Background(), u)
		if err != nil {
			s.logger.Error(err.Error())
			setInCacheSpan.End()
		}
		s.logger.Debug("Write to cache user by id: " + id)
		setInCacheSpan.End()

	}()
	getFromDBSpan.End()
	msg := fmt.Sprintf("[%s] Time for get user by id=%s with trace_id=%s", cstatus, id, traceId)
	s.logger.Info(msg, s.logger.String("cache_status", cstatus), s.logger.String("traceID", traceId))
	return u, nil
}

// Get all users from DB
// TODO: Implement caching in redis
func (s *service) findAll(limit, offset int64, ctx context.Context) (users []User, nextCursor int64, err error) {
	opts := []otrace.SpanStartOption{
		otrace.WithSpanKind(otrace.SpanKindServer),
	}

	tr := s.tracer.Tracer("Service.findAll")
	parentCtx, span := tr.Start(ctx, "GetUsers", opts...)
	defer span.End()
	//var cstatus string

	traceId := span.SpanContext().TraceID().String()

	//timer := prometheus.NewTimer(userGetDuration.WithLabelValues(id))
	// register time for all operations steps
	//defer timer.ObserveDuration()
	// log time duration for all operations steps without lock/unlock mutex and init prometheus metrics (clean time for get entity)
	//defer trace(s.logger, id, &cstatus, traceId)()
	//parentCacheCtx, getFromCacheSpan := tr.Start(parentCtx, "getFromCache", opts...)
	//u, err = s.cache.Get(context.Background(), id)
	//if err == nil {
	//	s.logger.Debug("Cache hit for user id: " + id)
	//	cstatus = "HIT"
	//	// after success get user from cache refresh expire time for him
	//	defer func() {
	//		_, setExpireInCache := tr.Start(parentCacheCtx, "setCacheExpiration", opts...)
	//
	//		err := s.cache.Expire(context.Background(), id)
	//		if err != nil {
	//			s.logger.Error("Set cache expiration failed for user id: " + id)
	//			s.error(err)
	//			setExpireInCache.End()
	//		}
	//		setExpireInCache.End()
	//	}()
	//	getFromCacheSpan.End()
	//	return u, nil
	//}
	//getFromCacheSpan.End()

	//cstatus = "MISS"
	//s.logger.Debug("Cache miss for user id: " + id)
	_, getFromDBSpan := tr.Start(parentCtx, "getFromDB", opts...)
	users, nextCursor, err = s.storage.FindAll(limit, offset)
	if err != nil {
		return []User{}, 0, fmt.Errorf("failed to get users. error: %w", err)
	}
	//after get user from storage place him to cache with ttl
	//defer func() {
	//	_, setInCacheSpan := tr.Start(parentDBCtx, "setInCache", opts...)
	//	err = s.cache.Set(context.Background(), users)
	//	if err != nil {
	//		s.logger.Error(err.Error())
	//		setInCacheSpan.End()
	//	}
	//	s.logger.Debug("Write to cache user by id: " + id)
	//	setInCacheSpan.End()
	//
	//}()
	getFromDBSpan.End()
	msg := fmt.Sprintf("Time for get users with trace_id=%s", traceId)
	s.logger.Info(msg, s.logger.String("traceID", traceId))
	return users, nextCursor, nil
}

// Delete User from DB
func (s *service) delete(id string, ctx context.Context) error {
	opts := []otrace.SpanStartOption{
		otrace.WithSpanKind(otrace.SpanKindServer),
	}

	tr := s.tracer.Tracer("Service.delete")
	parentCtx, span := tr.Start(ctx, "DeleteUserById", opts...)
	defer span.End()

	traceId := span.SpanContext().TraceID().String()

	//timer := prometheus.NewTimer(userGetDuration.WithLabelValues(id))
	// register time for all operations steps
	//defer timer.ObserveDuration()
	// log time duration for all operations steps without lock/unlock mutex and init prometheus metrics (clean time for get entity)
	//defer trace(s.logger, id, &cstatus, traceId)()

	parentDBCtx, deleteFromDBSpan := tr.Start(parentCtx, "deleteFromDB", opts...)
	err := s.storage.Delete(id)
	if err != nil {
		return fmt.Errorf("failed to delete user by id=%s. error: %w", id, err)
	}

	// after get user from storage place him to cache with ttl
	defer func() {
		_, delInCacheSpan := tr.Start(parentDBCtx, "delInCache", opts...)
		err = s.cache.Del(context.Background(), id)
		if err != nil {
			s.logger.Error(err.Error())
			delInCacheSpan.End()
		}
		s.logger.Debug("Del from cache user by id: " + id)
		delInCacheSpan.End()

	}()

	deleteFromDBSpan.End()
	msg := fmt.Sprintf("Time for delete user by id=%s with trace_id=%s", id, traceId)
	s.logger.Info(msg, s.logger.String("traceID", traceId))
	return nil
}

// Create User in DB
func (s *service) create(u *User, ctx context.Context) error {
	opts := []otrace.SpanStartOption{
		otrace.WithSpanKind(otrace.SpanKindServer),
	}

	tr := s.tracer.Tracer("Service.create")
	parentCtx, span := tr.Start(ctx, "CreateUser", opts...)
	defer span.End()

	traceId := span.SpanContext().TraceID().String()

	//timer := prometheus.NewTimer(userGetDuration.WithLabelValues(id))
	// register time for all operations steps
	//defer timer.ObserveDuration()
	// log time duration for all operations steps without lock/unlock mutex and init prometheus metrics (clean time for get entity)
	//defer trace(s.logger, id, &cstatus, traceId)()

	parentDBCtx, getFromDBSpan := tr.Start(parentCtx, "createInDB", opts...)
	err := s.storage.Create(u)
	if err != nil {
		return fmt.Errorf("failed to create user. error: %w", err)
	}
	// after get user from storage place him to cache with ttl
	defer func() {
		_, setInCacheSpan := tr.Start(parentDBCtx, "setInCache", opts...)
		err = s.cache.Set(context.Background(), *u)
		if err != nil {
			s.logger.Error(err.Error())
			setInCacheSpan.End()
		}
		s.logger.Debug(fmt.Sprintf("Write to cache user by id: %d", u.Id))
		setInCacheSpan.End()

	}()
	getFromDBSpan.End()
	msg := fmt.Sprintf("Time for create user with id=%d with trace_id=%s", u.Id, traceId)
	s.logger.Info(msg, s.logger.String("traceID", traceId))
	return nil
}

// Update User in DB
func (s *service) update(u *User, ctx context.Context) error {

	opts := []otrace.SpanStartOption{
		otrace.WithSpanKind(otrace.SpanKindServer),
	}

	tr := s.tracer.Tracer("Service.update")
	parentCtx, span := tr.Start(ctx, "UpdateUserById", opts...)
	defer span.End()

	traceId := span.SpanContext().TraceID().String()

	id := strconv.FormatInt(u.Id, 10)

	//timer := prometheus.NewTimer(userGetDuration.WithLabelValues(u.Id))
	// register time for all operations steps
	//defer timer.ObserveDuration()
	// log time duration for all operations steps without lock/unlock mutex and init prometheus metrics (clean time for get entity)
	//defer trace(s.logger, id, &cstatus, traceId)()

	parentDBCtx, updateInDBSpan := tr.Start(parentCtx, "updateInDB", opts...)
	err := s.storage.Update(u)

	if err != nil {
		return fmt.Errorf("failed to update user. error: %w", err)
	}

	// after get user from storage place him to cache with ttl
	defer func() {
		_, setInCacheSpan := tr.Start(parentDBCtx, "setInCache", opts...)
		err = s.cache.Set(context.Background(), *u)
		if err != nil {
			s.logger.Error(err.Error())
			setInCacheSpan.End()
		}
		s.logger.Debug("Write to cache user by id: " + id)
		setInCacheSpan.End()

	}()
	updateInDBSpan.End()
	msg := fmt.Sprintf("Time for update user by id=%s with trace_id=%s", id, traceId)
	s.logger.Info(msg, s.logger.String("traceID", traceId))
	return nil
}

func (s *service) getByID(id string, ctx context.Context) (u User, err error) {
	opts := []otrace.SpanStartOption{
		otrace.WithSpanKind(otrace.SpanKindServer),
	}

	tr := s.tracer.Tracer("Service.getById")
	parentCtx, span := tr.Start(ctx, "GetUserById", opts...)
	defer span.End()
	var cstatus string

	traceId := span.SpanContext().TraceID().String()

	timer := prometheus.NewTimer(userGetDuration.WithLabelValues(id))
	// register time for all operations steps
	defer timer.ObserveDuration()
	// log time duration for all operations steps without lock/unlock mutex and init prometheus metrics (clean time for get entity)
	defer trace(s.logger, id, &cstatus, traceId)()
	parentCacheCtx, getFromCacheSpan := tr.Start(parentCtx, "getFromCache", opts...)
	u, err = s.cache.Get(context.Background(), id)
	if err == nil {
		s.logger.Debug("Cache hit for user id: " + id)
		cstatus = "HIT"
		// after success get user from cache refresh expire time for him
		defer func() {
			_, setExpireInCache := tr.Start(parentCacheCtx, "setCacheExpiration", opts...)

			err := s.cache.Expire(context.Background(), id)
			if err != nil {
				s.logger.Error("Set cache expiration failed for user id: " + id)
				s.error(err)
				setExpireInCache.End()
			}
			setExpireInCache.End()
		}()
		getFromCacheSpan.End()
		return u, nil
	}
	getFromCacheSpan.End()

	cstatus = "MISS"
	s.logger.Debug("Cache miss for user id: " + id)
	parentDBCtx, getFromDBSpan := tr.Start(parentCtx, "getFromDB", opts...)
	u, err = s.storage.GetByID(id)
	if err != nil {
		return User{}, fmt.Errorf("failed to get user by id=%s. error: %w", id, err)
	}
	// after get user from storage place him to cache with ttl
	defer func() {
		_, setInCacheSpan := tr.Start(parentDBCtx, "setInCache", opts...)
		err = s.cache.Set(context.Background(), u)
		if err != nil {
			s.logger.Error(err.Error())
			setInCacheSpan.End()
		}
		s.logger.Debug("Write to cache user by id: " + id)
		setInCacheSpan.End()

	}()
	getFromDBSpan.End()
	//msg := fmt.Sprintf("[%s] Time for get user by id=%s with trace_id=%s", cstatus, id, traceId)
	//s.logger.Info(msg, s.logger.String("cache_status", cstatus), s.logger.String("traceID", traceId))
	return u, nil
}

func (s *service) findByNickname(nickname string, ctx context.Context) (u User, err error) {
	opts := []otrace.SpanStartOption{
		otrace.WithSpanKind(otrace.SpanKindServer),
	}

	tr := s.tracer.Tracer("Service.findByNickname")
	parentCtx, span := tr.Start(ctx, "FindUserByNicname", opts...)
	defer span.End()

	var cstatus string
	traceId := span.SpanContext().TraceID().String()

	// log time duration for all operations steps without lock/unlock mutex and init prometheus metrics (clean time for get entity)
	defer trace(s.logger, nickname, &cstatus, traceId)()
	parentCacheCtx, getFromCacheSpan := tr.Start(parentCtx, "getFromCache", opts...)
	defer getFromCacheSpan.End()
	u, err = s.cache.Get(context.Background(), nickname)
	if err == nil {
		cstatus = "HIT"
		// after success get user from cache refresh expire time for him
		defer func() {
			_, setExpireInCache := tr.Start(parentCacheCtx, "setCacheExpiration", opts...)
			defer setExpireInCache.End()
			err := s.cache.Expire(context.Background(), nickname)
			if err != nil {
				s.logger.Error("Set cache expiration failed for user nickname: " + nickname)
				s.error(err)
			}
		}()
		return u, nil
	}
	cstatus = "MISS"
	s.logger.Debug("Cache miss for user nickname: " + nickname)
	parentDBCtx, getFromDBSpan := tr.Start(parentCtx, "getFromDB", opts...)
	defer getFromDBSpan.End()
	u, err = s.storage.FindOneByNickName(nickname)
	if err != nil {
		return User{}, fmt.Errorf("failed to get user by id=%s. error: %w", nickname, err)
	}
	// after get user from storage place him to cache with ttl
	defer func() {
		_, setInCacheSpan := tr.Start(parentDBCtx, "setInCache", opts...)
		defer setInCacheSpan.End()
		err = s.cache.SetByNickname(context.Background(), u)
		if err != nil {
			s.logger.Error(err.Error())
		}
		s.logger.Debug("Write to cache user by nickname: " + nickname)
	}()
	return u, nil
}

func (s *service) error(err error) {
	sentry.CaptureException(err)
	// TODO: disable flush migrate to syncHTTPTransport https://docs.sentry.io/platforms/go/guides/http/configuration/transports/
	//sentry.Flush(time.Second * 1)
	s.logger.Error(err.Error())
}

func (s *service) info(msg string) {
	s.logger.Info(msg)
}

func (s *service) getTracer() (t *tracesdk.TracerProvider) {
	return s.tracer
}

func (s *service) getSingleFlightGroup() (sfg *singleflight.Group) {
	return s.sflight
}

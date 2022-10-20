package app

import (
	"context"
	"flag"
	"github.com/getsentry/sentry-go"
	"github.com/gorilla/mux"
	"github.com/heptiolabs/healthcheck"
	"github.com/pkg/profile"
	"net/http"
	"os"
	"os/signal"
	"redis/internal/user"
	"redis/internal/user/cache"
	psql "redis/internal/user/db"
	"redis/pkg/logging"
	"redis/pkg/monitoring"
	"redis/pkg/tracing"
	"syscall"
	"time"
)

type app struct {
	logger               logging.Logger
	tracer               tracing.AppTracer
	storage              user.Storage
	cache                user.Cache
	appRouter, monRouter *mux.Router
	service              user.Service
	appSrv, monSrv       *http.Server
}

func (a *app) initStorage() {
	userStorage, err := psql.NewStorage(&a.logger)
	a.logger.Info("Application storage initialized.")

	if err != nil {
		a.logger.Error(err.Error())
	}
	a.storage = userStorage

	go a.storage.KeepAlive()
}

func (a *app) initCache() {
	userCache, err := cache.New(&a.logger)
	a.logger.Info("Application cache initialized.")

	if err != nil {
		a.logger.Error(err.Error())
	}
	a.cache = userCache
	go a.cache.KeepAlive()
}

func (a *app) initSentry() {
	err := sentry.Init(sentry.ClientOptions{
		Release:     "redis-go@1.0.0",
		Environment: "production",
		Dsn:         os.Getenv("SENTRY_DSN"),
	})
	defer sentry.Flush(2 * time.Second)

	if err != nil {
		a.logger.Fatal("Init Sentry failed: " + err.Error())
	}
}

func (a *app) initTracer() {
	tracer, err := tracing.InitTracing(&a.logger)
	a.logger.Info("Application tracer initialized.")

	if err != nil {
		a.logger.Error(err.Error())
	}
	a.tracer = tracer
}

func (a *app) initService() {
	// TODO: refactor function params
	userService, err := user.NewService(a.storage, a.cache, a.logger, a.tracer.TracerProvider)
	a.logger.Info("Application service initialized.")

	if err != nil {
		a.fatalServer(err)
	}
	a.service = userService
}

func (a *app) startAppHTTPServer() {
	userHandler := user.GetHandler(a.service)
	userHandler.Register(a.appRouter)

	a.logger.Info("Starting server :8080")

	srv := &http.Server{
		Handler:      a.appRouter,
		Addr:         ":8080",
		WriteTimeout: 30 * time.Second,
		ReadTimeout:  30 * time.Second,
	}

	go func(s *http.Server) {
		if err := s.ListenAndServe(); err != nil && err != http.ErrServerClosed {
			a.fatalServer(err)
		}
	}(srv)

	a.appSrv = srv
}

func (a *app) startMonHTTPServer() {
	hc := healthcheck.NewHandler()
	hc.AddLivenessCheck("goroutine-threshold", user.GoroutineCountCheck(10))
	hc.AddReadinessCheck("database", user.DatabasePingCheck(a.storage, 1*time.Second))
	hc.AddReadinessCheck("cache", user.CachePingCheck(a.cache, 1*time.Second))
	metricsHandler := monitoring.GetHandler(a.logger)
	metricsHandler.Register(a.monRouter, hc)

	srvMon := &http.Server{
		Handler:      a.monRouter,
		Addr:         ":8081",
		WriteTimeout: 15 * time.Second,
		ReadTimeout:  15 * time.Second,
	}

	go func(s *http.Server) {
		if err := s.ListenAndServe(); err != nil && err != http.ErrServerClosed {
			a.fatalServer(err)
		}
	}(srvMon)

	a.logger.Info("Starting monitoring server :8081")

	a.monSrv = srvMon
}

func (a *app) init() {
	a.initSentry()
	a.initTracer()
	a.initStorage()
	a.initCache()
	a.initService()
}

func (a *app) start() {
	a.startAppHTTPServer()
	a.startMonHTTPServer()
}

func (a *app) shutdown() {
	a.logger.Info("Shutdown Application...")
	ctx, serverCancel := context.WithTimeout(context.Background(), 15*time.Second)
	err := a.appSrv.Shutdown(ctx)
	if err != nil {
		a.fatalServer(err)
	}
	err = a.monSrv.Shutdown(ctx)
	if err != nil {
		a.fatalServer(err)
	}
	serverCancel()
	a.storage.Close()
	err = a.cache.Close()
	if err != nil {
		a.fatalServer(err)
	}
	a.logger.Info("Application successful shutdown")
}

func (a *app) parseArgs() {
	mode := flag.String("profile.mode", "", "enable profiling mode, one of [cpu, mem, mutex, block]")
	flag.Parse()

	switch *mode {
	case "cpu":
		defer profile.Start(profile.CPUProfile, profile.ProfilePath(".")).Stop()
	case "mem":
		defer profile.Start(profile.MemProfile, profile.ProfilePath(".")).Stop()
	case "mutex":
		defer profile.Start(profile.MutexProfile, profile.ProfilePath(".")).Stop()
	case "block":
		defer profile.Start(profile.BlockProfile, profile.ProfilePath(".")).Stop()
	case "trace":
		defer profile.Start(profile.TraceProfile, profile.ProfilePath(".")).Stop()
	case "goroutine":
		defer profile.Start(profile.GoroutineProfile, profile.ProfilePath(".")).Stop()
	default:

	}
}

func (a *app) fatalServer(err error) {
	sentry.CaptureException(err)
	sentry.Flush(time.Second * 5)
	a.logger.Fatal(err.Error())
}

func newApp() *app {
	logger := logging.GetLogger()
	logger.Info("Application logger initialized.")
	router := mux.NewRouter()
	router.Use(user.PrometheusHTTPDurationMiddleware, logging.ResponseCodeMiddleware(logger))
	logger.Info("Application router initialized.")
	metricsRouter := mux.NewRouter()
	logger.Info("Metrics router initialized.")

	return &app{
		logger:    logger,
		tracer:    tracing.AppTracer{},
		storage:   nil,
		cache:     nil,
		appRouter: router,
		monRouter: metricsRouter,
		service:   nil,
		appSrv:    nil,
		monSrv:    nil,
	}
}

func Run() {

	app := newApp()
	app.parseArgs()
	app.init()
	app.start()

	//gracefull shutdown init here

	c := make(chan os.Signal, 1)
	signal.Notify(c, syscall.SIGABRT, syscall.SIGQUIT, syscall.SIGHUP, os.Interrupt, syscall.SIGTERM, syscall.SIGINT)

	<-c

	app.shutdown()
}

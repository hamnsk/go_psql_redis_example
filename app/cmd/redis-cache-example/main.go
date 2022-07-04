package main

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

func main() {

	parseArgs()

	logger := logging.GetLogger()
	logger.Info("Application logger initialized.")

	err := sentry.Init(sentry.ClientOptions{
		Dsn: os.Getenv("SENTRY_DSN"),
	})

	if err != nil {
		logger.Fatal("Init Sentry failed: " + err.Error())
	}

	err, tracer, tCloser := tracing.InitTracing(&logger)

	if err != nil {
		fatalServer(err, logger)
	} else {
		defer tCloser.Close()
	}

	router := mux.NewRouter()
	router.Use(user.PrometheusHTTPDurationMiddleware, logging.ResponseCodeMiddleware(logger))
	userStorage, err := psql.NewStorage(&logger)
	if err != nil {
		fatalServer(err, logger)
	}
	userCache, err := cache.New()

	if err != nil {
		fatalServer(err, logger)
	}

	userService, err := user.NewService(userStorage, userCache, logger, tracer)

	if err != nil {
		fatalServer(err, logger)
	}

	userHandler := user.GetHandler(userService)
	userHandler.Register(router)

	logger.Info("Starting server :8080")

	srv := &http.Server{
		Handler:      router,
		Addr:         ":8080",
		WriteTimeout: 30 * time.Second,
		ReadTimeout:  30 * time.Second,
	}

	go func(s *http.Server) {
		if err := s.ListenAndServe(); err != nil && err != http.ErrServerClosed {
			fatalServer(err, logger)
		}
	}(srv)

	//metrics init here

	metricsRouter := mux.NewRouter()
	hc := healthcheck.NewHandler()
	hc.AddLivenessCheck("goroutine-threshold", user.GoroutineCountCheck(10))
	hc.AddReadinessCheck("database", user.DatabasePingCheck(userStorage, 1*time.Second))
	hc.AddReadinessCheck("cache", user.CachePingCheck(userCache, 1*time.Second))
	metricsHandler := monitoring.GetHandler(logger)
	metricsHandler.Register(metricsRouter, hc)

	srvMon := &http.Server{
		Handler:      metricsRouter,
		Addr:         ":8081",
		WriteTimeout: 15 * time.Second,
		ReadTimeout:  15 * time.Second,
	}

	go func(s *http.Server) {
		if err := s.ListenAndServe(); err != nil && err != http.ErrServerClosed {
			fatalServer(err, logger)
		}
	}(srvMon)

	logger.Info("Starting monitoring server :8081")

	//gracefull shutdown init here

	c := make(chan os.Signal, 1)
	signal.Notify(c, syscall.SIGABRT, syscall.SIGQUIT, syscall.SIGHUP, os.Interrupt, syscall.SIGTERM, syscall.SIGINT)

	<-c

	shutdown(logger, srv, srvMon, userStorage, userCache)
}

func parseArgs() {
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

func fatalServer(err error, l logging.Logger) {
	sentry.CaptureException(err)
	sentry.Flush(time.Second * 5)
	l.Fatal(err.Error())
}

func shutdown(l logging.Logger, appSrv, monSrv *http.Server, storage user.Storage, cache user.Cache) {

	l.Info("Shutdown Application...")
	ctx, serverCancel := context.WithTimeout(context.Background(), 15*time.Second)
	err := appSrv.Shutdown(ctx)
	if err != nil {
		fatalServer(err, l)
	}
	err = monSrv.Shutdown(ctx)
	if err != nil {
		fatalServer(err, l)
	}
	serverCancel()
	storage.Close()
	err = cache.Close()
	if err != nil {
		fatalServer(err, l)
	}
	l.Info("Application successful shutdown")

}

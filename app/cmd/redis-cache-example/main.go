package main

import (
	"context"
	"github.com/getsentry/sentry-go"
	"github.com/gorilla/mux"
	"github.com/heptiolabs/healthcheck"
	"net/http"
	"os"
	"os/signal"
	"redis/internal/user"
	"redis/internal/user/cache"
	psql "redis/internal/user/db"
	"redis/pkg/logging"
	"redis/pkg/monitoring"
	"syscall"
	"time"
)

func main() {

	logger := logging.GetLogger()
	logger.Info("Application logger initialized.")

	err := sentry.Init(sentry.ClientOptions{
		Dsn: os.Getenv("SENTRY_DSN"),
	})

	if err != nil {
		logger.Fatal("Init Sentry failed: " + err.Error())
	}

	router := mux.NewRouter()
	router.Use(user.PrometheusHTTPDurationMiddleware)
	userStorage, err := psql.NewStorage()
	if err != nil {
		fatalServer(err, logger)
	}
	userCache, err := cache.New()

	if err != nil {
		fatalServer(err, logger)
	}

	userService, err := user.NewService(userStorage, userCache, logger)

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


func fatalServer(err error, l logging.Logger) {
	sentry.CaptureException(err)
	sentry.Flush(time.Second * 5)
	l.Fatal(err.Error())
}

func shutdown(l logging.Logger, appSrv, monSrv *http.Server, storage user.Storage, cache user.Cache ) {

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
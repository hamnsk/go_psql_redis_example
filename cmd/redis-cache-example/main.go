package main

import (
	"context"
	"github.com/gorilla/mux"
	"net/http"
	"os"
	"os/signal"
	"redis/internal/user"
	"redis/internal/user/cache"
	psql "redis/internal/user/db"
	"redis/pkg/logging"
	"syscall"
	"time"
)

func main() {
	logger := logging.GetLogger()
	logger.Info("Application logger initialized.")

	router := mux.NewRouter()
	userStorage, err := psql.NewStorage()
	if err != nil {
		logger.Fatal(err.Error())
	}
	userCache, err := cache.New()

	if err != nil {
		logger.Fatal(err.Error())
	}

	userService, err := user.NewService(userStorage, userCache, logger)

	if err != nil {
		logger.Fatal(err.Error())
	}

	userHandler := user.GetHandler(userService)
	userHandler.Register(router)


	logger.Info("Starting server :8080")

	srv := &http.Server{
		Handler:      router,
		Addr:         ":8080",
		WriteTimeout: 15 * time.Second,
		ReadTimeout:  15 * time.Second,
	}

	go func(s *http.Server) {
		if err := s.ListenAndServe(); err != nil && err != http.ErrServerClosed {
			logger.Sugar().Fatalf("Error on server startup: %s", err.Error())
		}
	}(srv)

	c := make(chan os.Signal, 1)
	signal.Notify(c, syscall.SIGABRT, syscall.SIGQUIT, syscall.SIGHUP, os.Interrupt, syscall.SIGTERM, syscall.SIGINT)

	<-c

	logger.Info("Shutdown Application...")
	ctx, serverCancel := context.WithTimeout(context.Background(), 15*time.Second)
	srv.Shutdown(ctx)
	serverCancel()
	logger.Info("Application successful shutdown")
}
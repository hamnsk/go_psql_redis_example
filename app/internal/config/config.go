package config

import "os"

type cfg struct {
	sentry string
	redis string
	postgres string
	log_file string
	log_level string
}


func GetConfig() *cfg {
	return &cfg{
		sentry: os.Getenv("SENTRY_DSN"),
		redis: os.Getenv("REDIS"),
		postgres: os.Getenv("DATABASE_URL"),
		log_file: os.Getenv("APP_LOG_FILE"),
		log_level: os.Getenv("APP_LOG_LEVEL"),
	}
}
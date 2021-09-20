package logging

import (
	"go.uber.org/zap"
	"go.uber.org/zap/zapcore"
	"os"
	"path"
)

var ee *zap.Logger

type Logger struct {
	*zap.Logger
}

func GetLogger() Logger {
	return Logger{ee}
}

func init() {
	config := zap.NewProductionConfig()
	config.EncoderConfig.TimeKey = "timestamp"
	config.EncoderConfig.EncodeLevel = zapcore.CapitalLevelEncoder
	config.EncoderConfig.EncodeTime = zapcore.ISO8601TimeEncoder
	config.EncoderConfig.MessageKey = "message"
	lf := os.Getenv("APP_LOG_FILE")
	if len(lf) > 0 {
		p, err := os.Getwd()
		if err == nil {
			config.OutputPaths = append(config.OutputPaths, path.Join(p, lf))
			config.ErrorOutputPaths = append(config.ErrorOutputPaths, path.Join(p, lf))
		}
	}

	ee, _ = config.Build()
}

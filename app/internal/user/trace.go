package user

import (
	"fmt"
	"redis/pkg/logging"
	"time"
)

func trace(l logging.Logger, id string, cstatus *string) func () {
	start := time.Now()
	return func () {
		t := time.Since(start)
		msg := fmt.Sprintf("[%s] Time for get user by id=%s operation: %s", *cstatus, id, t)
		l.Info(msg, l.String("cache_status", *cstatus), l.Duration("time_duration", t))
	}
}

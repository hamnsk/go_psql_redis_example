package user

import (
	"fmt"
	"redis/pkg/logging"
	"time"
)

func trace(l logging.Logger, id string, cstatus *string, traceId string) func() {
	start := time.Now()
	return func() {
		t := time.Since(start)
		msg := fmt.Sprintf("[%s] Time for get user by id=%s operation: %s with trace_id=%s", *cstatus, id, t, traceId)
		l.Info(msg, l.String("cache_status", *cstatus), l.Duration("time_duration", t))
	}
}

package user

import (
	"redis/pkg/logging"
	"time"
)

func trace(l logging.Logger, id string, cstatus *string) func () {
	start := time.Now()
	return func () { l.Sugar().Infof("[%s] Time for get user by id=%s operation: %s", *cstatus, id, time.Since(start))}
}

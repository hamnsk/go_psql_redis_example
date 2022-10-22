package user

import (
	"context"
	"fmt"
	"github.com/gorilla/mux"
	"github.com/heptiolabs/healthcheck"
	"github.com/prometheus/client_golang/prometheus"
	"github.com/prometheus/client_golang/prometheus/promauto"
	"net/http"
	"runtime"
	"time"
)

var getUserRequestsTotal prometheus.Gauge
var updateUserRequestsTotal prometheus.Gauge
var deleteUserRequestsTotal prometheus.Gauge
var createUserRequestsTotal prometheus.Gauge
var getAllUsersRequestsTotal prometheus.Gauge
var getUserRequestsError prometheus.Gauge
var updateUserRequestsError prometheus.Gauge
var deleteUserRequestsError prometheus.Gauge
var createUserRequestsError prometheus.Gauge
var getAllUsersRequestsError prometheus.Gauge
var getUserRequestsSuccess prometheus.Gauge
var updateUserRequestsSuccess prometheus.Gauge
var deleteUserRequestsSuccess prometheus.Gauge
var createUserRequestsSuccess prometheus.Gauge
var getAllUsersRequestsSuccess prometheus.Gauge
var httpStatusCodes *prometheus.CounterVec

var (
	httpDuration = promauto.NewHistogramVec(prometheus.HistogramOpts{
		Name: "redis_cache_example_user_http_request_duration_seconds",
		Help: "Duration of HTTP requests.",
	}, []string{"path"})
)

func init() {
	getUserRequestsTotal = prometheus.NewGauge(
		prometheus.GaugeOpts{
			Name: "redis_cache_example_user_get_request_total",
			Help: "Total requests for user endpoint",
		})
	prometheus.MustRegister(getUserRequestsTotal)

	updateUserRequestsTotal = prometheus.NewGauge(
		prometheus.GaugeOpts{
			Name: "redis_cache_example_user_update_request_total",
			Help: "Total requests for user endpoint",
		})
	prometheus.MustRegister(updateUserRequestsTotal)

	deleteUserRequestsTotal = prometheus.NewGauge(
		prometheus.GaugeOpts{
			Name: "redis_cache_example_user_delete_request_total",
			Help: "Total requests for user endpoint",
		})
	prometheus.MustRegister(deleteUserRequestsTotal)

	createUserRequestsTotal = prometheus.NewGauge(
		prometheus.GaugeOpts{
			Name: "redis_cache_example_user_create_request_total",
			Help: "Total requests for user endpoint",
		})
	prometheus.MustRegister(createUserRequestsTotal)

	getAllUsersRequestsTotal = prometheus.NewGauge(
		prometheus.GaugeOpts{
			Name: "redis_cache_example_user_get_all_request_total",
			Help: "Total requests for user endpoint",
		})
	prometheus.MustRegister(getAllUsersRequestsTotal)

	getUserRequestsError = prometheus.NewGauge(
		prometheus.GaugeOpts{
			Name: "redis_cache_example_user_get_request_error",
			Help: "Error requests for user endpoint",
		})
	prometheus.MustRegister(getUserRequestsError)

	updateUserRequestsError = prometheus.NewGauge(
		prometheus.GaugeOpts{
			Name: "redis_cache_example_user_update_request_error",
			Help: "Error requests for user endpoint",
		})
	prometheus.MustRegister(updateUserRequestsError)

	deleteUserRequestsError = prometheus.NewGauge(
		prometheus.GaugeOpts{
			Name: "redis_cache_example_user_delete_request_error",
			Help: "Error requests for user endpoint",
		})
	prometheus.MustRegister(deleteUserRequestsError)

	createUserRequestsError = prometheus.NewGauge(
		prometheus.GaugeOpts{
			Name: "redis_cache_example_user_create_request_error",
			Help: "Error requests for user endpoint",
		})
	prometheus.MustRegister(createUserRequestsError)

	getAllUsersRequestsError = prometheus.NewGauge(
		prometheus.GaugeOpts{
			Name: "redis_cache_example_user_get_all_request_error",
			Help: "Error requests for user endpoint",
		})
	prometheus.MustRegister(getAllUsersRequestsError)

	getUserRequestsSuccess = prometheus.NewGauge(
		prometheus.GaugeOpts{
			Name: "redis_cache_example_user_get_request_success",
			Help: "Success requests for user endpoint",
		})
	prometheus.MustRegister(getUserRequestsSuccess)

	updateUserRequestsSuccess = prometheus.NewGauge(
		prometheus.GaugeOpts{
			Name: "redis_cache_example_user_update_request_success",
			Help: "Success requests for user endpoint",
		})
	prometheus.MustRegister(updateUserRequestsSuccess)

	deleteUserRequestsSuccess = prometheus.NewGauge(
		prometheus.GaugeOpts{
			Name: "redis_cache_example_user_update_request_success",
			Help: "Success requests for user endpoint",
		})
	prometheus.MustRegister(deleteUserRequestsSuccess)

	createUserRequestsSuccess = prometheus.NewGauge(
		prometheus.GaugeOpts{
			Name: "redis_cache_example_user_create_request_success",
			Help: "Success requests for user endpoint",
		})
	prometheus.MustRegister(createUserRequestsSuccess)

	getAllUsersRequestsSuccess = prometheus.NewGauge(
		prometheus.GaugeOpts{
			Name: "redis_cache_example_user_get_all_request_success",
			Help: "Success requests for user endpoint",
		})
	prometheus.MustRegister(getAllUsersRequestsSuccess)

	httpStatusCodes = prometheus.NewCounterVec(
		prometheus.CounterOpts{
			Name: "redis_cache_example_user_get_handler_request_total",
			Help: "Total number of get users by HTTP status code.",
		},
		[]string{"code", "method"})

	prometheus.MustRegister(httpStatusCodes)
}

func GoroutineCountCheck(threshold int) healthcheck.Check {
	return func() error {
		count := runtime.NumGoroutine()
		if count > threshold {
			return fmt.Errorf("too many goroutines (%d > %d)", count, threshold)
		}
		return nil
	}
}

func DatabasePingCheck(db Storage, timeout time.Duration) healthcheck.Check {
	return func() error {
		ctx, cancel := context.WithTimeout(context.Background(), timeout)
		defer cancel()
		if db == nil {
			return fmt.Errorf("database is nil")
		}
		return db.PingPool(ctx)
	}
}

func CachePingCheck(cache Cache, timeout time.Duration) healthcheck.Check {
	return func() error {
		ctx, cancel := context.WithTimeout(context.Background(), timeout)
		defer cancel()
		if cache == nil {
			return fmt.Errorf("database is nil")
		}
		return cache.PingClient(ctx)
	}
}

func PrometheusHTTPDurationMiddleware(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		route := mux.CurrentRoute(r)
		path, _ := route.GetPathTemplate()
		timer := prometheus.NewTimer(httpDuration.WithLabelValues(path))
		next.ServeHTTP(w, r)
		timer.ObserveDuration()
	})
}

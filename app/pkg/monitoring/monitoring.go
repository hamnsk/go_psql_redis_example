package monitoring

import (
	"github.com/gorilla/mux"
	"github.com/heptiolabs/healthcheck"
	"github.com/prometheus/client_golang/prometheus/promhttp"
	"redis/pkg/logging"
)

const (
	metricsURL   = "/metrics"
	livenessURL  = "/live"
	readinessURL = "/ready"
)

var _ Handler = &handler{}

type handler struct {
	logger logging.Logger
}

type Handler interface {
	Register(router *mux.Router, hc healthcheck.Handler)
}

func (h *handler) Register(router *mux.Router, hc healthcheck.Handler) {
	router.Handle(metricsURL, promhttp.Handler())
	router.HandleFunc(livenessURL, hc.LiveEndpoint)
	router.HandleFunc(readinessURL, hc.ReadyEndpoint)
}

func GetHandler(logger logging.Logger) Handler {
	h := handler{
		logger: logger,
	}
	return &h
}
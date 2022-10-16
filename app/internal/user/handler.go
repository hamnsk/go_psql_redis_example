package user

import (
	"encoding/json"
	"fmt"
	"github.com/gorilla/mux"
	"github.com/opentracing/opentracing-go"
	"github.com/opentracing/opentracing-go/ext"
	"github.com/prometheus/client_golang/prometheus"
	"net/http"
	"strconv"
)

const (
	userURL   = "/user/{id}"
	searchURL = "/user/search/"
)

var _ Handler = &userHandler{}

type userHandler struct {
	UserService Service
}

type AppError struct {
	Message string `json:"error"`
}

type Handler interface {
	Register(router *mux.Router)
}

func (h *userHandler) Register(router *mux.Router) {
	router.HandleFunc(userURL, h.getUserById).Methods(http.MethodGet)
	router.HandleFunc(searchURL, h.getUserByNickname).Methods(http.MethodGet)
}

func (h *userHandler) getUserById(w http.ResponseWriter, r *http.Request) {
	tracer := h.UserService.getTracer()
	spanCtx, _ := tracer.Extract(opentracing.HTTPHeaders, opentracing.HTTPHeadersCarrier(r.Header))
	serverSpan := tracer.StartSpan("get-user-by-id", ext.RPCServerOption(spanCtx))
	serverSpan.SetTag("request_uri", r.RequestURI)
	serverSpan.SetTag("request_body", r.Body)
	serverSpan.SetTag("request_header", r.Header)
	serverSpan.SetTag("request_method", r.Method)
	serverSpan.SetTag("request_content_length", r.ContentLength)
	serverSpan.SetTag("trace_id", r.Header.Get("Uber-Trace-Id"))
	w.Header().Set("Content-Type", "application/json")
	id := mux.Vars(r)["id"]
	serverSpan.SetTag("user_id", id)
	timer := prometheus.NewTimer(userGetDuration.WithLabelValues(id))
	// register time for all operations steps

	userIDSpan := tracer.StartSpan("convert-id-to-int", ext.RPCServerOption(spanCtx))

	if _, err := strconv.Atoi(id); err != nil {
		//render result to client
		renderJSON(w, &AppError{Message: fmt.Sprintf("nothing interresing, you trace id: %s", r.Header.Get("Uber-Trace-Id"))}, http.StatusTeapot)
		h.UserService.error(err)

		// after response increment prometheus metrics
		defer func() {
			serverSpan.Finish()
			getUserRequestsTotal.Inc()
			getUserRequestsError.Inc()
			httpStatusCodes.WithLabelValues(strconv.Itoa(http.StatusTeapot), http.MethodGet).Inc()
			timer.ObserveDuration()
			userIDSpan.Finish()
		}()
		return
	}
	userIDSpan.Finish()

	// call user service to get requested user from cache, if not found get from storage and place to cache
	userGetSpan := tracer.StartSpan("singleflight-call-storage", ext.RPCServerOption(spanCtx))
	workHash := fmt.Sprintf("getUserByID:%s", id)

	sflight := h.UserService.getSingleFlightGroup()
	user, err, _ := sflight.Do(workHash, func() (interface{}, error) {
		//h.UserService.info(workHash)
		//h.UserService.info("Call User Service getById")
		return h.UserService.getByID(id, spanCtx, r.Header.Get("Uber-Trace-Id"))
	})
	//h.UserService.info(fmt.Sprintf("Result %s is shared: %t", workHash, shared))

	if err != nil {
		//render result to client
		renderJSON(w, &AppError{fmt.Sprintf("not found, you trace id: %s", r.Header.Get("Uber-Trace-Id"))}, http.StatusNotFound)
		h.UserService.error(err)
		// after response increment prometheus metrics
		defer func() {
			serverSpan.Finish()
			getUserRequestsTotal.Inc()
			getUserRequestsError.Inc()
			httpStatusCodes.WithLabelValues(strconv.Itoa(http.StatusNotFound), http.MethodGet).Inc()
			timer.ObserveDuration()
			userGetSpan.Finish()
		}()
		return
	}
	userGetSpan.Finish()

	// after response increment prometheus metrics
	defer func() {
		serverSpan.Finish()
		getUserRequestsTotal.Inc()
		getUserRequestsSuccess.Inc()
		httpStatusCodes.WithLabelValues(strconv.Itoa(http.StatusOK), http.MethodGet).Inc()
		timer.ObserveDuration()
	}()
	renderJSON(w, &user, http.StatusOK)
}

func (h *userHandler) getUserByNickname(w http.ResponseWriter, r *http.Request) {
	tracer := h.UserService.getTracer()
	spanCtx, _ := tracer.Extract(opentracing.HTTPHeaders, opentracing.HTTPHeadersCarrier(r.Header))
	serverSpan := tracer.StartSpan("get-user-by-nickname", ext.RPCServerOption(spanCtx))
	serverSpan.SetTag("request_uri", r.RequestURI)
	serverSpan.SetTag("request_body", r.Body)
	serverSpan.SetTag("request_header", r.Header)
	serverSpan.SetTag("request_method", r.Method)
	serverSpan.SetTag("request_content_length", r.ContentLength)
	serverSpan.SetTag("trace_id", r.Header.Get("Uber-Trace-Id"))
	w.Header().Set("Content-Type", "application/json")
	nickname := r.FormValue("nickname")
	serverSpan.SetTag("nickname", nickname)
	timer := prometheus.NewTimer(userGetDuration.WithLabelValues(nickname))
	// register time for all operations steps

	// call user service to get requested user from cache, if not found get from storage and place to cache
	workHash := fmt.Sprintf("getUserByNickname:%s", nickname)

	sflight := h.UserService.getSingleFlightGroup()
	user, err, _ := sflight.Do(workHash, func() (interface{}, error) {
		//h.UserService.info(workHash)
		//h.UserService.info("Call User Service getUserByNickname")
		return h.UserService.findByNickname(nickname, spanCtx, r.Header.Get("Uber-Trace-Id"))
	})

	if err != nil {
		//render result to client
		renderJSON(w, &AppError{Message: "not found"}, http.StatusNotFound)
		h.UserService.error(err)

		// after response increment prometheus metrics
		defer func() {
			serverSpan.Finish()
			getUserRequestsTotal.Inc()
			getUserRequestsError.Inc()
			httpStatusCodes.WithLabelValues(strconv.Itoa(http.StatusNotFound), http.MethodGet).Inc()
			timer.ObserveDuration()
		}()

		return
	}

	// after response increment prometheus metrics
	defer func() {
		serverSpan.Finish()
		getUserRequestsTotal.Inc()
		getUserRequestsSuccess.Inc()
		httpStatusCodes.WithLabelValues(strconv.Itoa(http.StatusOK), http.MethodGet).Inc()
		timer.ObserveDuration()
	}()

	//render result to client
	renderJSON(w, &user, http.StatusOK)
}

func GetHandler(userService Service) Handler {
	h := userHandler{
		UserService: userService,
	}
	return &h
}

func renderJSON(w http.ResponseWriter, val interface{}, statusCode int) {
	w.WriteHeader(statusCode)
	_ = json.NewEncoder(w).Encode(val)
}

package user

import (
	"encoding/json"
	"fmt"
	"github.com/gorilla/mux"
	"github.com/opentracing/opentracing-go"
	"github.com/opentracing/opentracing-go/ext"
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
	defer serverSpan.Finish()
	w.Header().Set("Content-Type", "application/json")
	id := mux.Vars(r)["id"]
	serverSpan.SetTag("user_id", id)
	// after response increment prometheus metrics
	defer getUserRequestsTotal.Inc()

	if _, err := strconv.Atoi(id); err != nil {
		// after response increment prometheus metrics
		defer getUserRequestsError.Inc()
		// after response increment prometheus metrics
		defer httpStatusCodes.WithLabelValues(strconv.Itoa(http.StatusTeapot), http.MethodGet).Inc()
		//render result to client
		renderJSON(w, &AppError{Message: fmt.Sprintf("nothing interresing: %s", r.Header.Get("Uber-Trace-Id"))}, http.StatusTeapot)
		h.UserService.error(err)
		return
	}

	// call user service to get requested user from cache, if not found get from storage and place to cache
	workHash := fmt.Sprintf("getUserByID:%s", id)

	s := h.UserService.getSingleFlightGroup()
	user, err, _ := s.Do(workHash, func() (interface{}, error) {
		return h.UserService.getByID(id)
	})

	if err != nil {
		// after response increment prometheus metrics
		defer getUserRequestsError.Inc()
		// after response increment prometheus metrics
		defer httpStatusCodes.WithLabelValues(strconv.Itoa(http.StatusNotFound), http.MethodGet).Inc()
		//render result to client
		renderJSON(w, &AppError{Message: "not found"}, http.StatusNotFound)
		h.UserService.error(err)
		return
	}
	// after response increment prometheus metrics
	defer getUserRequestsSuccess.Inc()
	// after response increment prometheus metrics
	defer httpStatusCodes.WithLabelValues(strconv.Itoa(http.StatusOK), http.MethodGet).Inc()
	//render result to client
	renderJSON(w, &user, http.StatusOK)
}

func (h *userHandler) getUserByNickname(w http.ResponseWriter, r *http.Request) {
	w.Header().Set("Content-Type", "application/json")
	nickname := r.FormValue("nickname")
	// after response increment prometheus metrics
	defer getUserRequestsTotal.Inc()

	// call user service to get requested user from cache, if not found get from storage and place to cache
	workHash := fmt.Sprintf("getUserByNickname:%s", nickname)

	s := h.UserService.getSingleFlightGroup()
	user, err, _ := s.Do(workHash, func() (interface{}, error) {
		return h.UserService.findByNickname(nickname)
	})

	if err != nil {
		// after response increment prometheus metrics
		defer getUserRequestsError.Inc()
		// after response increment prometheus metrics
		defer httpStatusCodes.WithLabelValues(strconv.Itoa(http.StatusNotFound), http.MethodGet).Inc()
		//render result to client
		renderJSON(w, &AppError{Message: "not found"}, http.StatusNotFound)
		h.UserService.error(err)
		return
	}
	// after response increment prometheus metrics
	defer getUserRequestsSuccess.Inc()
	// after response increment prometheus metrics
	defer httpStatusCodes.WithLabelValues(strconv.Itoa(http.StatusOK), http.MethodGet).Inc()
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

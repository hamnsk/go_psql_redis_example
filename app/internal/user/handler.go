package user

import (
	"context"
	"encoding/json"
	"fmt"
	"github.com/gorilla/mux"
	"go.opentelemetry.io/otel/attribute"
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
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()
	tracer := h.UserService.getTracer()
	tr := tracer.Tracer("get-user-by-id-handler")
	_, span := tr.Start(ctx, "get-user")

	span.SetAttributes(attribute.Key("request_uri").String(r.RequestURI))
	//span.SetAttributes(attribute.Key("request_body").String(r.Body))
	//span.SetAttributes(attribute.Key("request_header").String(r.Header))
	span.SetAttributes(attribute.Key("request_method").String(r.Method))
	span.SetAttributes(attribute.Key("request_content_length").Int64(r.ContentLength))
	span.SetAttributes(attribute.Key("user_agent").String(r.Header.Get("User-Agent")))

	defer span.End()
	w.Header().Set("Content-Type", "application/json")
	id := mux.Vars(r)["id"]
	span.SetAttributes(attribute.Key("user_id").String(id))
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
		span.SetStatus(http.StatusTeapot, "Hello from teapot")
		return
	}

	// call user service to get requested user from cache, if not found get from storage and place to cache
	user, err := h.UserService.getByID(id)

	if err != nil {
		// after response increment prometheus metrics
		defer getUserRequestsError.Inc()
		// after response increment prometheus metrics
		defer httpStatusCodes.WithLabelValues(strconv.Itoa(http.StatusNotFound), http.MethodGet).Inc()
		//render result to client
		renderJSON(w, &AppError{Message: "not found"}, http.StatusNotFound)
		h.UserService.error(err)
		span.SetStatus(http.StatusNotFound, "Not found user by id")
		return
	}
	// after response increment prometheus metrics
	defer getUserRequestsSuccess.Inc()
	// after response increment prometheus metrics
	defer httpStatusCodes.WithLabelValues(strconv.Itoa(http.StatusOK), http.MethodGet).Inc()
	//render result to client
	renderJSON(w, &user, http.StatusOK)
	span.SetStatus(http.StatusOK, "All ok!")
}

func (h *userHandler) getUserByNickname(w http.ResponseWriter, r *http.Request) {
	w.Header().Set("Content-Type", "application/json")
	nickname := r.FormValue("nickname")
	fmt.Println(nickname)
	// after response increment prometheus metrics
	defer getUserRequestsTotal.Inc()

	// call user service to get requested user from cache, if not found get from storage and place to cache
	user, err := h.UserService.findByNickname(nickname)

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

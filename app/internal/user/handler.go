package user

import (
	"context"
	"encoding/json"
	"fmt"
	"github.com/gorilla/mux"
	"go.opentelemetry.io/contrib/instrumentation/net/http/httptrace/otelhttptrace"
	"go.opentelemetry.io/otel/attribute"
	semconv "go.opentelemetry.io/otel/semconv/v1.12.0"
	otrace "go.opentelemetry.io/otel/trace"
	"io/ioutil"
	"net/http"
	"strconv"
)

const (
	userURL   = "/user/{id}"
	crudURL   = "/user"
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
	router.HandleFunc(userURL, h.findOneUser).Methods(http.MethodGet)
	router.HandleFunc(crudURL, h.createUser).Methods(http.MethodPost)
	router.HandleFunc(userURL, h.updateUser).Methods(http.MethodPut)
	router.HandleFunc(userURL, h.deleteUser).Methods(http.MethodDelete)
	router.HandleFunc(searchURL, h.getUserByNickname).Methods(http.MethodGet)
}

func (h *userHandler) findAllUsers(w http.ResponseWriter, r *http.Request) {

}

// FindOne User Handler
func (h *userHandler) findOneUser(w http.ResponseWriter, r *http.Request) {
	ctx, cancel := context.WithCancel(r.Context())
	defer cancel()

	tracer := h.UserService.getTracer()
	tr := tracer.Tracer("Handler.findOneUser")
	opts := []otrace.SpanStartOption{
		otrace.WithAttributes(semconv.NetAttributesFromHTTPRequest("tcp", r)...),
		otrace.WithAttributes(semconv.EndUserAttributesFromHTTPRequest(r)...),
		otrace.WithSpanKind(otrace.SpanKindServer),
	}
	_, _, spanContext := otelhttptrace.Extract(ctx, r)
	reqCtx := otrace.ContextWithSpanContext(ctx, spanContext)

	parentCtx, span := tr.Start(reqCtx, "FindUser", opts...)
	defer span.End()

	span.SetAttributes(attribute.Key("request_uri").String(r.RequestURI))
	span.SetAttributes(attribute.Key("request_method").String(r.Method))
	span.SetAttributes(attribute.Key("request_content_length").Int64(r.ContentLength))
	span.SetAttributes(attribute.Key("user_agent").String(r.Header.Get("User-Agent")))

	w.Header().Set("Content-Type", "application/json")
	id := mux.Vars(r)["id"]
	span.SetAttributes(attribute.Key("user_id").String(id))
	// after response increment prometheus metrics
	defer getUserRequestsTotal.Inc()

	_, convertAtoiSpan := tr.Start(parentCtx, "StringToInt", opts...)

	if _, err := strconv.Atoi(id); err != nil {
		// after response increment prometheus metrics
		defer getUserRequestsError.Inc()
		// after response increment prometheus metrics
		defer httpStatusCodes.WithLabelValues(strconv.Itoa(http.StatusTeapot), http.MethodGet).Inc()
		//render result to client
		renderJSON(w, &AppError{Message: fmt.Sprintf("nothing interresing: %s", r.Header.Get("Uber-Trace-Id"))}, http.StatusTeapot)
		h.UserService.error(err)
		span.SetStatus(http.StatusTeapot, "Hello from teapot")
		convertAtoiSpan.End()
		return
	}
	convertAtoiSpan.End()

	callUserServiceCtx, userServiceCallSpan := tr.Start(parentCtx, "CallUserService", opts...)
	workHash := fmt.Sprintf("getUserByID:%s", id)
	sflight := h.UserService.getSingleFlightGroup()
	// call user service to get requested user from cache, if not found get from storage and place to cache
	user, err, _ := sflight.Do(workHash, func() (interface{}, error) {
		return h.UserService.getByID(id, callUserServiceCtx)
	})

	if err != nil {
		// after response increment prometheus metrics
		defer getUserRequestsError.Inc()
		// after response increment prometheus metrics
		defer httpStatusCodes.WithLabelValues(strconv.Itoa(http.StatusNotFound), http.MethodGet).Inc()
		//render result to client
		renderJSON(w, &AppError{Message: "not found"}, http.StatusNotFound)
		h.UserService.error(err)
		span.SetStatus(http.StatusNotFound, "Not found user by id")
		userServiceCallSpan.End()
		return
	}
	userServiceCallSpan.End()

	// after response increment prometheus metrics
	defer getUserRequestsSuccess.Inc()
	// after response increment prometheus metrics
	defer httpStatusCodes.WithLabelValues(strconv.Itoa(http.StatusOK), http.MethodGet).Inc()
	//render result to client
	renderJSON(w, &user, http.StatusOK)
	span.SetStatus(http.StatusOK, "All ok!")
}

// Create User Handler
func (h *userHandler) createUser(w http.ResponseWriter, r *http.Request) {
	ctx, cancel := context.WithCancel(r.Context())
	defer cancel()

	tracer := h.UserService.getTracer()
	tr := tracer.Tracer("Handler.createUser")
	opts := []otrace.SpanStartOption{
		otrace.WithAttributes(semconv.NetAttributesFromHTTPRequest("tcp", r)...),
		otrace.WithAttributes(semconv.EndUserAttributesFromHTTPRequest(r)...),
		otrace.WithSpanKind(otrace.SpanKindServer),
	}
	_, _, spanContext := otelhttptrace.Extract(ctx, r)
	reqCtx := otrace.ContextWithSpanContext(ctx, spanContext)

	parentCtx, span := tr.Start(reqCtx, "CreateUser", opts...)
	defer span.End()

	span.SetAttributes(attribute.Key("request_uri").String(r.RequestURI))
	span.SetAttributes(attribute.Key("request_method").String(r.Method))
	span.SetAttributes(attribute.Key("request_content_length").Int64(r.ContentLength))
	span.SetAttributes(attribute.Key("user_agent").String(r.Header.Get("User-Agent")))

	w.Header().Set("Content-Type", "application/json")

	user := &User{}
	parseBody(r, user)

	callUserServiceCtx, userServiceCallSpan := tr.Start(parentCtx, "CallUserService", opts...)
	workHash := fmt.Sprintf("createUser:%s%s%s", user.FistName, user.LastName, user.NickName)
	sflight := h.UserService.getSingleFlightGroup()
	// call user service to get requested user from cache, if not found get from storage and place to cache
	_, err, _ := sflight.Do(workHash, func() (interface{}, error) {
		return nil, h.UserService.create(user, callUserServiceCtx)
	})

	if err != nil {
		// after response increment prometheus metrics
		//defer getUserRequestsError.Inc()
		// after response increment prometheus metrics
		//defer httpStatusCodes.WithLabelValues(strconv.Itoa(http.StatusBadRequest), http.MethodPut).Inc()
		//render result to client
		renderJSON(w, &AppError{Message: "failed to create"}, http.StatusBadRequest)
		h.UserService.error(err)
		span.SetStatus(http.StatusBadRequest, "Failed to create user")
		//userServiceCallSpan.End()
		return
	}
	userServiceCallSpan.End()

	// after response increment prometheus metrics
	//defer getUserRequestsSuccess.Inc()
	// after response increment prometheus metrics
	//defer httpStatusCodes.WithLabelValues(strconv.Itoa(http.StatusOK), http.MethodGet).Inc()
	//render result to client
	renderJSON(w, &user, http.StatusCreated)
	span.SetStatus(http.StatusCreated, "All ok!")
}

// Update User Handler
func (h *userHandler) updateUser(w http.ResponseWriter, r *http.Request) {
	ctx, cancel := context.WithCancel(r.Context())
	defer cancel()

	tracer := h.UserService.getTracer()
	tr := tracer.Tracer("Handler.updateUser")
	opts := []otrace.SpanStartOption{
		otrace.WithAttributes(semconv.NetAttributesFromHTTPRequest("tcp", r)...),
		otrace.WithAttributes(semconv.EndUserAttributesFromHTTPRequest(r)...),
		otrace.WithSpanKind(otrace.SpanKindServer),
	}
	_, _, spanContext := otelhttptrace.Extract(ctx, r)
	reqCtx := otrace.ContextWithSpanContext(ctx, spanContext)

	parentCtx, span := tr.Start(reqCtx, "UpdateUser", opts...)
	defer span.End()

	span.SetAttributes(attribute.Key("request_uri").String(r.RequestURI))
	span.SetAttributes(attribute.Key("request_method").String(r.Method))
	span.SetAttributes(attribute.Key("request_content_length").Int64(r.ContentLength))
	span.SetAttributes(attribute.Key("user_agent").String(r.Header.Get("User-Agent")))

	w.Header().Set("Content-Type", "application/json")
	id := mux.Vars(r)["id"]
	span.SetAttributes(attribute.Key("user_id").String(id))
	// after response increment prometheus metrics
	//defer getUserRequestsTotal.Inc() TODO: Create Metric

	_, convertAtoiSpan := tr.Start(parentCtx, "StringToInt", opts...)

	if _, err := strconv.Atoi(id); err != nil {
		// after response increment prometheus metrics
		//defer getUserRequestsError.Inc()
		// after response increment prometheus metrics
		defer httpStatusCodes.WithLabelValues(strconv.Itoa(http.StatusTeapot), http.MethodGet).Inc()
		//render result to client
		renderJSON(w, &AppError{Message: fmt.Sprintf("nothing interresing: %s", r.Header.Get("Uber-Trace-Id"))}, http.StatusTeapot)
		h.UserService.error(err)
		span.SetStatus(http.StatusTeapot, "Hello from teapot")
		convertAtoiSpan.End()
		return
	}
	convertAtoiSpan.End()

	user := &User{}
	parseBody(r, user)
	if uid, err := strconv.Atoi(id); err == nil {
		user.Id = int64(uid)
	}

	callUserServiceCtx, userServiceCallSpan := tr.Start(parentCtx, "CallUserService", opts...)
	workHash := fmt.Sprintf("updateUserByID:%s", id)
	sflight := h.UserService.getSingleFlightGroup()
	// call user service to get requested user from cache, if not found get from storage and place to cache
	_, err, _ := sflight.Do(workHash, func() (interface{}, error) {
		return nil, h.UserService.update(user, callUserServiceCtx)
	})

	if err != nil {
		// after response increment prometheus metrics
		//defer getUserRequestsError.Inc()
		// after response increment prometheus metrics
		//defer httpStatusCodes.WithLabelValues(strconv.Itoa(http.StatusNotFound), http.MethodGet).Inc()
		//render result to client
		renderJSON(w, &AppError{Message: "not found"}, http.StatusNotFound)
		h.UserService.error(err)
		span.SetStatus(http.StatusNotFound, "Not found user by id")
		userServiceCallSpan.End()
		return
	}
	userServiceCallSpan.End()

	// after response increment prometheus metrics
	//defer getUserRequestsSuccess.Inc()
	// after response increment prometheus metrics
	defer httpStatusCodes.WithLabelValues(strconv.Itoa(http.StatusOK), http.MethodGet).Inc()
	//render result to client
	renderJSON(w, &user, http.StatusOK)
	span.SetStatus(http.StatusOK, "All ok!")
}

// Delete User Handler
func (h *userHandler) deleteUser(w http.ResponseWriter, r *http.Request) {
	ctx, cancel := context.WithCancel(r.Context())
	defer cancel()

	tracer := h.UserService.getTracer()
	tr := tracer.Tracer("Handler.deleteUser")
	opts := []otrace.SpanStartOption{
		otrace.WithAttributes(semconv.NetAttributesFromHTTPRequest("tcp", r)...),
		otrace.WithAttributes(semconv.EndUserAttributesFromHTTPRequest(r)...),
		otrace.WithSpanKind(otrace.SpanKindServer),
	}
	_, _, spanContext := otelhttptrace.Extract(ctx, r)
	reqCtx := otrace.ContextWithSpanContext(ctx, spanContext)

	parentCtx, span := tr.Start(reqCtx, "DeleteUser", opts...)
	defer span.End()

	span.SetAttributes(attribute.Key("request_uri").String(r.RequestURI))
	span.SetAttributes(attribute.Key("request_method").String(r.Method))
	span.SetAttributes(attribute.Key("request_content_length").Int64(r.ContentLength))
	span.SetAttributes(attribute.Key("user_agent").String(r.Header.Get("User-Agent")))

	w.Header().Set("Content-Type", "application/json")
	id := mux.Vars(r)["id"]
	span.SetAttributes(attribute.Key("user_id").String(id))
	// after response increment prometheus metrics
	//defer getUserRequestsTotal.Inc() //TODO: Create Metric

	_, convertAtoiSpan := tr.Start(parentCtx, "StringToInt", opts...)

	if _, err := strconv.Atoi(id); err != nil {
		// after response increment prometheus metrics
		//defer getUserRequestsError.Inc() //TODO: Create Metric
		// after response increment prometheus metrics
		//defer httpStatusCodes.WithLabelValues(strconv.Itoa(http.StatusTeapot), http.MethodGet).Inc() //TODO: Create Metric
		//render result to client
		renderJSON(w, &AppError{Message: fmt.Sprintf("nothing interresing: %s", r.Header.Get("Uber-Trace-Id"))}, http.StatusTeapot)
		h.UserService.error(err)
		span.SetStatus(http.StatusTeapot, "Hello from teapot")
		convertAtoiSpan.End()
		return
	}
	convertAtoiSpan.End()

	callUserServiceCtx, userServiceCallSpan := tr.Start(parentCtx, "CallUserService", opts...)
	workHash := fmt.Sprintf("deleteUser:%s", id)
	sflight := h.UserService.getSingleFlightGroup()

	// call user service to get requested user from cache, if not found get from storage and place to cache
	_, err, _ := sflight.Do(workHash, func() (interface{}, error) {
		return nil, h.UserService.delete(id, callUserServiceCtx)
	})

	if err != nil {
		// after response increment prometheus metrics
		//defer getUserRequestsError.Inc()
		// after response increment prometheus metrics
		//defer httpStatusCodes.WithLabelValues(strconv.Itoa(http.StatusNotFound), http.MethodGet).Inc()
		//render result to client
		renderJSON(w, &AppError{Message: "not found"}, http.StatusNotFound)
		h.UserService.error(err)
		span.SetStatus(http.StatusNotFound, "Can not delete user by id")
		userServiceCallSpan.End()
		return
	}
	userServiceCallSpan.End()

	// after response increment prometheus metrics
	//defer getUserRequestsSuccess.Inc()
	// after response increment prometheus metrics
	//defer httpStatusCodes.WithLabelValues(strconv.Itoa(http.StatusOK), http.MethodGet).Inc()
	//render result to client
	renderJSON(w, User{}, http.StatusOK)
	span.SetStatus(http.StatusOK, "All ok!")
}

func (h *userHandler) getUserById(w http.ResponseWriter, r *http.Request) {

	ctx, cancel := context.WithCancel(r.Context())
	defer cancel()

	tracer := h.UserService.getTracer()
	tr := tracer.Tracer("Handler.getUserById")
	opts := []otrace.SpanStartOption{
		otrace.WithAttributes(semconv.NetAttributesFromHTTPRequest("tcp", r)...),
		otrace.WithAttributes(semconv.EndUserAttributesFromHTTPRequest(r)...),
		otrace.WithSpanKind(otrace.SpanKindServer),
	}
	_, _, spanContext := otelhttptrace.Extract(ctx, r)
	reqCtx := otrace.ContextWithSpanContext(ctx, spanContext)

	parentCtx, span := tr.Start(reqCtx, "GetUser", opts...)
	defer span.End()

	span.SetAttributes(attribute.Key("request_uri").String(r.RequestURI))
	span.SetAttributes(attribute.Key("request_method").String(r.Method))
	span.SetAttributes(attribute.Key("request_content_length").Int64(r.ContentLength))
	span.SetAttributes(attribute.Key("user_agent").String(r.Header.Get("User-Agent")))

	w.Header().Set("Content-Type", "application/json")
	id := mux.Vars(r)["id"]
	span.SetAttributes(attribute.Key("user_id").String(id))
	// after response increment prometheus metrics
	defer getUserRequestsTotal.Inc()

	_, convertAtoiSpan := tr.Start(parentCtx, "StringToInt", opts...)

	if _, err := strconv.Atoi(id); err != nil {
		// after response increment prometheus metrics
		defer getUserRequestsError.Inc()
		// after response increment prometheus metrics
		defer httpStatusCodes.WithLabelValues(strconv.Itoa(http.StatusTeapot), http.MethodGet).Inc()
		//render result to client
		renderJSON(w, &AppError{Message: fmt.Sprintf("nothing interresing: %s", r.Header.Get("Uber-Trace-Id"))}, http.StatusTeapot)
		h.UserService.error(err)
		span.SetStatus(http.StatusTeapot, "Hello from teapot")
		convertAtoiSpan.End()
		return
	}
	convertAtoiSpan.End()

	callUserServiceCtx, userServiceCallSpan := tr.Start(parentCtx, "CallUserService", opts...)
	workHash := fmt.Sprintf("getUserByID:%s", id)
	sflight := h.UserService.getSingleFlightGroup()
	// call user service to get requested user from cache, if not found get from storage and place to cache
	user, err, _ := sflight.Do(workHash, func() (interface{}, error) {
		return h.UserService.getByID(id, callUserServiceCtx)
	})

	//user, err := h.UserService.getByID(id, callUserServiceCtx)

	if err != nil {
		// after response increment prometheus metrics
		defer getUserRequestsError.Inc()
		// after response increment prometheus metrics
		defer httpStatusCodes.WithLabelValues(strconv.Itoa(http.StatusNotFound), http.MethodGet).Inc()
		//render result to client
		renderJSON(w, &AppError{Message: "not found"}, http.StatusNotFound)
		h.UserService.error(err)
		span.SetStatus(http.StatusNotFound, "Not found user by id")
		userServiceCallSpan.End()
		return
	}
	userServiceCallSpan.End()

	// after response increment prometheus metrics
	defer getUserRequestsSuccess.Inc()
	// after response increment prometheus metrics
	defer httpStatusCodes.WithLabelValues(strconv.Itoa(http.StatusOK), http.MethodGet).Inc()
	//render result to client
	renderJSON(w, &user, http.StatusOK)
	span.SetStatus(http.StatusOK, "All ok!")
}

func (h *userHandler) getUserByNickname(w http.ResponseWriter, r *http.Request) {

	ctx, cancel := context.WithCancel(r.Context())
	defer cancel()

	tracer := h.UserService.getTracer()
	tr := tracer.Tracer("Handler.getUserById")
	opts := []otrace.SpanStartOption{
		otrace.WithAttributes(semconv.NetAttributesFromHTTPRequest("tcp", r)...),
		otrace.WithAttributes(semconv.EndUserAttributesFromHTTPRequest(r)...),
		otrace.WithSpanKind(otrace.SpanKindServer),
	}
	_, _, spanContext := otelhttptrace.Extract(ctx, r)
	reqCtx := otrace.ContextWithSpanContext(ctx, spanContext)

	_, span := tr.Start(reqCtx, "GetUser", opts...)

	span.SetAttributes(attribute.Key("request_uri").String(r.RequestURI))
	span.SetAttributes(attribute.Key("request_method").String(r.Method))
	span.SetAttributes(attribute.Key("request_content_length").Int64(r.ContentLength))
	span.SetAttributes(attribute.Key("user_agent").String(r.Header.Get("User-Agent")))

	defer span.End()

	w.Header().Set("Content-Type", "application/json")
	nickname := r.FormValue("nickname")
	fmt.Println(nickname)
	span.SetAttributes(attribute.Key("user_nickname").String(nickname))
	// after response increment prometheus metrics
	defer getUserRequestsTotal.Inc()

	// call user service to get requested user from cache, if not found get from storage and place to cache
	workHash := fmt.Sprintf("getUserByNickname:%s", nickname)

	sflight := h.UserService.getSingleFlightGroup()
	user, err, _ := sflight.Do(workHash, func() (interface{}, error) {
		return h.UserService.findByNickname(nickname, reqCtx)
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

func parseBody(r *http.Request, x interface{}) {
	if body, err := ioutil.ReadAll(r.Body); err == nil {
		if err := json.Unmarshal([]byte(body), x); err != nil {
			return
		}
	}
}

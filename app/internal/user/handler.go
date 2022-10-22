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
	withParamsUserURL    = "/user/{id:[0-9]+}"
	withOutParamsUserURL = "/user"
	searchURL            = "/user/search/"
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

// TODO: Refactor Handlers move out boilerplate code

func (h *userHandler) Register(router *mux.Router) {
	router.HandleFunc(withOutParamsUserURL, h.findAllUsers).Methods(http.MethodGet)
	router.HandleFunc(withParamsUserURL, h.findOneUser).Methods(http.MethodGet)
	router.HandleFunc(withOutParamsUserURL, h.createUser).Methods(http.MethodPost)
	router.HandleFunc(withParamsUserURL, h.updateUser).Methods(http.MethodPut)
	router.HandleFunc(withParamsUserURL, h.deleteUser).Methods(http.MethodDelete)
	router.HandleFunc(searchURL, h.getUserByNickname).Methods(http.MethodGet)
}

func (h *userHandler) findAllUsers(w http.ResponseWriter, r *http.Request) {
	ctx, cancel := context.WithCancel(r.Context())
	defer cancel()

	tracer := h.UserService.getTracer()
	tr := tracer.Tracer("Handler.findAllUsers")
	opts := []otrace.SpanStartOption{
		otrace.WithAttributes(semconv.NetAttributesFromHTTPRequest("tcp", r)...),
		otrace.WithAttributes(semconv.EndUserAttributesFromHTTPRequest(r)...),
		otrace.WithSpanKind(otrace.SpanKindServer),
	}
	_, _, spanContext := otelhttptrace.Extract(ctx, r)
	reqCtx := otrace.ContextWithSpanContext(ctx, spanContext)

	parentCtx, span := tr.Start(reqCtx, "FindAllUsers", opts...)
	defer span.End()

	span.SetAttributes(attribute.Key("request_uri").String(r.RequestURI))
	span.SetAttributes(attribute.Key("request_method").String(r.Method))
	span.SetAttributes(attribute.Key("request_content_length").Int64(r.ContentLength))
	span.SetAttributes(attribute.Key("user_agent").String(r.Header.Get("User-Agent")))

	w.Header().Set("Content-Type", "application/json")
	limitVar := r.URL.Query().Get("limit")
	offsetVar := r.Header.Get("X-NextCursor")

	// after response increment prometheus metrics
	defer getAllUsersRequestsTotal.Inc()

	_, convertAtoiSpan := tr.Start(parentCtx, "LimitOffsetStringToInt", opts...)

	limit, err := strconv.Atoi(limitVar)

	if err != nil && limitVar != "" {
		// after response increment prometheus metrics
		defer getAllUsersRequestsError.Inc()
		// after response increment prometheus metrics
		defer httpStatusCodes.WithLabelValues(strconv.Itoa(http.StatusTeapot), http.MethodGet).Inc()
		//render result to client
		renderJSON(w, &AppError{Message: fmt.Sprintf("nothing interresing: %s", r.Header.Get("Uber-Trace-Id"))}, http.StatusTeapot)
		h.UserService.error(err)
		span.SetStatus(http.StatusTeapot, "Hello from teapot")
		convertAtoiSpan.End()
		return
	}

	if limit == 0 {
		limit = 10
	}

	offset, err := strconv.Atoi(offsetVar)

	if err != nil && offsetVar != "" {
		// after response increment prometheus metrics
		defer getAllUsersRequestsError.Inc()
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
	workHash := fmt.Sprintf("findAllUser:%s%s", limitVar, offsetVar)
	sflight := h.UserService.getSingleFlightGroup()
	//// call user service to get requested user from cache, if not found get from storage and place to cache
	users, err, _ := sflight.Do(workHash, func() (interface{}, error) {
		return h.UserService.findAll(int64(limit), int64(offset), callUserServiceCtx)
	})

	if err != nil {
		// after response increment prometheus metrics
		defer getAllUsersRequestsError.Inc()
		// after response increment prometheus metrics
		defer httpStatusCodes.WithLabelValues(strconv.Itoa(http.StatusNotFound), http.MethodGet).Inc()
		//render result to client
		renderJSON(w, &AppError{Message: "not found"}, http.StatusNotFound)
		h.UserService.error(err)
		span.SetStatus(http.StatusNotFound, "Not found users")
		userServiceCallSpan.End()
		return
	}
	userServiceCallSpan.End()

	// after response increment prometheus metrics
	defer getAllUsersRequestsSuccess.Inc()
	// after response increment prometheus metrics
	defer httpStatusCodes.WithLabelValues(strconv.Itoa(http.StatusOK), http.MethodGet).Inc()
	//render result to client
	var nextCursor int64
	if len(users.([]User)) > 0 {
		nextCursor = users.([]User)[len(users.([]User))-1].Id
	}
	w.Header().Set("X-NextCursor", fmt.Sprintf("%d", nextCursor))
	renderJSON(w, users, http.StatusOK)
	span.SetStatus(http.StatusOK, "All ok!")
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
		return h.UserService.findOne(id, callUserServiceCtx)
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

	defer createUserRequestsTotal.Inc()

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
		defer createUserRequestsError.Inc()
		// after response increment prometheus metrics
		defer httpStatusCodes.WithLabelValues(strconv.Itoa(http.StatusBadRequest), http.MethodPost).Inc()
		//render result to client
		renderJSON(w, &AppError{Message: "failed to create"}, http.StatusBadRequest)
		h.UserService.error(err)
		span.SetStatus(http.StatusBadRequest, "Failed to create user")
		//userServiceCallSpan.End()
		return
	}
	userServiceCallSpan.End()

	// after response increment prometheus metrics
	defer createUserRequestsSuccess.Inc()
	// after response increment prometheus metrics
	defer httpStatusCodes.WithLabelValues(strconv.Itoa(http.StatusCreated), http.MethodPost).Inc()
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
	defer updateUserRequestsTotal.Inc()

	_, convertAtoiSpan := tr.Start(parentCtx, "StringToInt", opts...)

	if _, err := strconv.Atoi(id); err != nil {
		// after response increment prometheus metrics
		defer updateUserRequestsError.Inc()
		// after response increment prometheus metrics
		defer httpStatusCodes.WithLabelValues(strconv.Itoa(http.StatusTeapot), http.MethodPut).Inc()
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
		defer updateUserRequestsError.Inc()
		// after response increment prometheus metrics
		defer httpStatusCodes.WithLabelValues(strconv.Itoa(http.StatusNotFound), http.MethodPut).Inc()
		//render result to client
		renderJSON(w, &AppError{Message: "not found"}, http.StatusNotFound)
		h.UserService.error(err)
		span.SetStatus(http.StatusNotFound, "Not found user by id")
		userServiceCallSpan.End()
		return
	}
	userServiceCallSpan.End()

	// after response increment prometheus metrics
	defer updateUserRequestsSuccess.Inc()
	// after response increment prometheus metrics
	defer httpStatusCodes.WithLabelValues(strconv.Itoa(http.StatusOK), http.MethodPut).Inc()
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
	defer deleteUserRequestsTotal.Inc()

	_, convertAtoiSpan := tr.Start(parentCtx, "StringToInt", opts...)

	if _, err := strconv.Atoi(id); err != nil {
		// after response increment prometheus metrics
		defer deleteUserRequestsError.Inc()
		// after response increment prometheus metrics
		defer httpStatusCodes.WithLabelValues(strconv.Itoa(http.StatusTeapot), http.MethodDelete).Inc()
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
		defer deleteUserRequestsError.Inc()
		// after response increment prometheus metrics
		defer httpStatusCodes.WithLabelValues(strconv.Itoa(http.StatusNotFound), http.MethodDelete).Inc()
		//render result to client
		renderJSON(w, &AppError{Message: "not found"}, http.StatusNotFound)
		h.UserService.error(err)
		span.SetStatus(http.StatusNotFound, "Can not delete user by id")
		userServiceCallSpan.End()
		return
	}
	userServiceCallSpan.End()

	// after response increment prometheus metrics
	defer deleteUserRequestsSuccess.Inc()
	// after response increment prometheus metrics
	defer httpStatusCodes.WithLabelValues(strconv.Itoa(http.StatusOK), http.MethodDelete).Inc()
	//render result to client
	renderJSON(w, User{}, http.StatusOK)
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
		if err := json.Unmarshal(body, x); err != nil {
			return
		}
	}
}

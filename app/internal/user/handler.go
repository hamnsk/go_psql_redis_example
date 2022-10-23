package user

import (
	"context"
	"encoding/json"
	"fmt"
	"github.com/gorilla/mux"
	"go.opentelemetry.io/contrib/instrumentation/net/http/httptrace/otelhttptrace"
	"go.opentelemetry.io/otel/attribute"
	"go.opentelemetry.io/otel/codes"
	semconv "go.opentelemetry.io/otel/semconv/v1.12.0"
	otrace "go.opentelemetry.io/otel/trace"
	"io/ioutil"
	"net/http"
	"strconv"
)

const (
	withParamsUserURL    = "/user/{id}"
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

type respData struct {
	w          http.ResponseWriter
	span       otrace.Span
	statusCode int
	httpMethod string
	payload    interface{}
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

// Find All Users with SingleFlight
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

	h.setSpanAttributes(span, r)

	limitVar := r.URL.Query().Get("limit")
	offsetVar := r.Header.Get("X-NextCursor")

	// after response increment prometheus metrics
	defer getAllUsersRequestsTotal.Inc()

	_, convertAtoiSpan := tr.Start(parentCtx, "LimitOffsetStringToInt", opts...)

	limit, err := strconv.Atoi(limitVar)

	if err != nil && limitVar != "" {
		h.handleErrorResponse(&respData{
			w:          w,
			span:       span,
			statusCode: http.StatusTeapot,
			httpMethod: http.MethodGet,
			payload:    err,
		})
		// after response increment prometheus metrics
		getAllUsersRequestsError.Inc()
		convertAtoiSpan.End()
		return
	}

	if limit == 0 || limit < 0 {
		limit = 10
	}

	offset, err := strconv.Atoi(offsetVar)

	if offset < 0 {
		offset = 0
	}

	if err != nil && offsetVar != "" {
		h.handleErrorResponse(&respData{
			w:          w,
			span:       span,
			statusCode: http.StatusTeapot,
			httpMethod: http.MethodGet,
			payload:    err,
		})
		// after response increment prometheus metrics
		getAllUsersRequestsError.Inc()
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
		h.handleErrorResponse(&respData{
			w:          w,
			span:       span,
			statusCode: http.StatusNotFound,
			httpMethod: http.MethodGet,
			payload:    err,
		})
		// after response increment prometheus metrics
		getAllUsersRequestsError.Inc()
		userServiceCallSpan.End()
		return
	}
	userServiceCallSpan.End()

	// after response increment prometheus metrics
	getAllUsersRequestsSuccess.Inc()
	//render result to client
	var nextCursor, prevCursor int64
	if len(users.([]User)) > 0 {
		nextCursor = users.([]User)[len(users.([]User))-1].Id
		prevCursor = users.([]User)[0].Id - int64(limit) - 1
		if prevCursor < 0 {
			prevCursor = 0
		}
	}
	w.Header().Set("X-NextCursor", fmt.Sprintf("%d", nextCursor))
	w.Header().Set("X-PrevCursor", fmt.Sprintf("%d", prevCursor))
	h.handleSuccessResponse(&respData{
		w:          w,
		span:       span,
		statusCode: http.StatusOK,
		httpMethod: http.MethodGet,
		payload:    &users,
	})
}

// FindOne User Handler with SingleFlight
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

	h.setSpanAttributes(span, r)

	id := mux.Vars(r)["id"]
	span.SetAttributes(attribute.Key("user_id").String(id))
	// after response increment prometheus metrics
	defer getUserRequestsTotal.Inc()

	_, convertAtoiSpan := tr.Start(parentCtx, "StringToInt", opts...)

	if _, err := strconv.Atoi(id); err != nil {
		h.handleErrorResponse(&respData{
			w:          w,
			span:       span,
			statusCode: http.StatusTeapot,
			httpMethod: http.MethodGet,
			payload:    err,
		})
		// after response increment prometheus metrics
		getUserRequestsError.Inc()
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
		h.handleErrorResponse(&respData{
			w:          w,
			span:       span,
			statusCode: http.StatusNotFound,
			httpMethod: http.MethodGet,
			payload:    err,
		})
		// after response increment prometheus metrics
		getUserRequestsError.Inc()
		userServiceCallSpan.End()
		return
	}
	userServiceCallSpan.End()

	// after response increment prometheus metrics
	getUserRequestsSuccess.Inc()

	h.handleSuccessResponse(&respData{
		w:          w,
		span:       span,
		statusCode: http.StatusOK,
		httpMethod: http.MethodGet,
		payload:    &user,
	})
}

// Create User Handler with SingleFlight
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

	h.setSpanAttributes(span, r)

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
		h.handleErrorResponse(&respData{
			w:          w,
			span:       span,
			statusCode: http.StatusBadRequest,
			httpMethod: http.MethodPost,
			payload:    err,
		})
		// after response increment prometheus metrics
		createUserRequestsError.Inc()
		userServiceCallSpan.End()
		return
	}
	userServiceCallSpan.End()

	// after response increment prometheus metrics
	createUserRequestsSuccess.Inc()
	h.handleSuccessResponse(&respData{
		w:          w,
		span:       span,
		statusCode: http.StatusCreated,
		httpMethod: http.MethodPost,
		payload:    &user,
	})
}

// Update User Handler with SingleFlight
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

	h.setSpanAttributes(span, r)

	id := mux.Vars(r)["id"]
	span.SetAttributes(attribute.Key("user_id").String(id))
	// after response increment prometheus metrics
	defer updateUserRequestsTotal.Inc()

	_, convertAtoiSpan := tr.Start(parentCtx, "StringToInt", opts...)

	if _, err := strconv.Atoi(id); err != nil {
		h.handleErrorResponse(&respData{
			w:          w,
			span:       span,
			statusCode: http.StatusTeapot,
			httpMethod: http.MethodPut,
			payload:    err,
		})
		// after response increment prometheus metrics
		updateUserRequestsError.Inc()
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
		h.handleErrorResponse(&respData{
			w:          w,
			span:       span,
			statusCode: http.StatusNotFound,
			httpMethod: http.MethodPut,
			payload:    err,
		})
		// after response increment prometheus metrics
		updateUserRequestsError.Inc()
		userServiceCallSpan.End()
		return
	}
	userServiceCallSpan.End()

	// after response increment prometheus metrics
	defer updateUserRequestsSuccess.Inc()
	h.handleSuccessResponse(&respData{
		w:          w,
		span:       span,
		statusCode: http.StatusOK,
		httpMethod: http.MethodPut,
		payload:    &user,
	})
}

// Delete User Handler with SingleFlight
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

	h.setSpanAttributes(span, r)

	id := mux.Vars(r)["id"]
	span.SetAttributes(attribute.Key("user_id").String(id))
	// after response increment prometheus metrics
	defer deleteUserRequestsTotal.Inc()

	_, convertAtoiSpan := tr.Start(parentCtx, "StringToInt", opts...)

	if _, err := strconv.Atoi(id); err != nil {
		h.handleErrorResponse(&respData{
			w:          w,
			span:       span,
			statusCode: http.StatusTeapot,
			httpMethod: http.MethodDelete,
			payload:    err,
		})
		// after response increment prometheus metrics
		deleteUserRequestsError.Inc()
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
		h.handleErrorResponse(&respData{
			w:          w,
			span:       span,
			statusCode: http.StatusNotFound,
			httpMethod: http.MethodDelete,
			payload:    err,
		})
		// after response increment prometheus metrics
		deleteUserRequestsError.Inc()
		userServiceCallSpan.End()
		return
	}
	userServiceCallSpan.End()

	// after response increment prometheus metrics
	defer deleteUserRequestsSuccess.Inc()
	h.handleSuccessResponse(&respData{
		w:          w,
		span:       span,
		statusCode: http.StatusOK,
		httpMethod: http.MethodDelete,
		payload:    User{},
	})
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

	h.setSpanAttributes(span, r)

	defer span.End()

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
		h.handleErrorResponse(&respData{
			w:          w,
			span:       span,
			statusCode: http.StatusNotFound,
			httpMethod: http.MethodGet,
			payload:    err,
		})
		// after response increment prometheus metrics
		getUserRequestsError.Inc()
		return
	}
	// after response increment prometheus metrics
	getUserRequestsSuccess.Inc()
	h.handleSuccessResponse(&respData{
		w:          w,
		span:       span,
		statusCode: http.StatusOK,
		httpMethod: http.MethodGet,
		payload:    user,
	})
}

func (h *userHandler) setSpanAttributes(span otrace.Span, r *http.Request) {
	span.SetAttributes(attribute.Key("request_uri").String(r.RequestURI))
	span.SetAttributes(attribute.Key("request_method").String(r.Method))
	span.SetAttributes(attribute.Key("request_content_length").Int64(r.ContentLength))
	span.SetAttributes(attribute.Key("user_agent").String(r.Header.Get("User-Agent")))
}

func (h *userHandler) handleErrorResponse(he *respData) {
	traceId := he.span.SpanContext().TraceID().String()
	he.span.SetStatus(codes.Code(he.statusCode), "request processing ended with an error")
	// after response increment prometheus metrics
	httpStatusCodes.WithLabelValues(strconv.Itoa(he.statusCode), he.httpMethod).Inc()
	//render result to client
	renderJSON(he.w, &AppError{Message: fmt.Sprintf("request processing ended with an error, "+
		"contact support by passing them the request ID: %s", traceId)}, he.statusCode)
	h.UserService.error(he.payload.(error))
}

func (h *userHandler) handleSuccessResponse(hs *respData) {
	// after response increment prometheus metrics
	httpStatusCodes.WithLabelValues(strconv.Itoa(hs.statusCode), hs.httpMethod).Inc()
	//render result to client
	renderJSON(hs.w, &hs.payload, hs.statusCode)
	hs.span.SetStatus(codes.Code(hs.statusCode), "All ok!")
}

func GetHandler(userService Service) Handler {
	h := userHandler{
		UserService: userService,
	}
	return &h
}

func renderJSON(w http.ResponseWriter, val interface{}, statusCode int) {
	w.Header().Set("Content-Type", "application/json")
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

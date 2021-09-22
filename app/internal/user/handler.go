package user

import (
	"encoding/json"
	"github.com/gorilla/mux"
	"net/http"
	"strconv"
	"sync"
)

const (
	userURL   = "/user/{id}"
)

var _ Handler = &userHandler{}

type userHandler struct {
	mu sync.Mutex
	UserService Service
}

type AppError struct {
	Message string `json:"error"`
}

type Handler interface {
	Register(router *mux.Router)
}

func (h *userHandler) Register(router *mux.Router) {
	router.HandleFunc(userURL, h.getUserById)
}

func (h *userHandler) getUserById(w http.ResponseWriter, r *http.Request) {
	w.Header().Set("Content-Type", "application/json")
	id := mux.Vars(r)["id"]

	if _, err :=strconv.Atoi(id); err !=nil{
		getUserRequestsTotal.Inc()
		getUserRequestsError.Inc()
		httpStatusCodes.WithLabelValues(strconv.Itoa(http.StatusTeapot), http.MethodGet).Inc()
		renderJSON(w, &AppError{Message: "nothing interresing"}, http.StatusTeapot)
		h.UserService.error(err)
		return
	}

	h.mu.Lock()
	user, err := h.UserService.getByID(id)
	h.mu.Unlock()
	getUserRequestsTotal.Inc()
	if err != nil {
		getUserRequestsError.Inc()
		httpStatusCodes.WithLabelValues(strconv.Itoa(http.StatusNotFound), http.MethodGet).Inc()
		renderJSON(w, &AppError{Message: "not found"}, http.StatusNotFound)
		h.UserService.error(err)
		return
	}
	getUserRequestsSuccess.Inc()
	httpStatusCodes.WithLabelValues(strconv.Itoa(http.StatusOK), http.MethodGet).Inc()
	renderJSON(w, &user, http.StatusOK)
}

func GetHandler(userService Service) Handler {
	h := userHandler{
		UserService: userService,
	}
	return &h
}


func renderJSON (w http.ResponseWriter, val interface{}, statusCode int) {
	w.WriteHeader(statusCode)
	_ = json.NewEncoder(w).Encode(val)
}
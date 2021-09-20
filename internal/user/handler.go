package user

import (
	"encoding/json"
	"github.com/gorilla/mux"
	"net/http"
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

	h.mu.Lock()
	user, err := h.UserService.getByID(id)
	h.mu.Unlock()
	if err != nil {
		renderJSON(w, &AppError{Message: err.Error()}, http.StatusBadRequest)
		h.UserService.error(err.Error())
		return
	}
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
package handlers

import (
	"encoding/json"
	"net/http"

	models "github.com/nikhil/eaven/internal/models/users"
	services "github.com/nikhil/eaven/internal/service/auth"
)

type AuthHandler struct {
	Service *services.AuthService
}

// NewAuthHandler creates a new instance of AuthHandler
func NewAuthHandler(service *services.AuthService) *AuthHandler {
	return &AuthHandler{Service: service}
}

// Signup handles the user registration request
func (h *AuthHandler) Signup(w http.ResponseWriter, r *http.Request) {
	w.Header().Set("Content-Type", "application/json")
	var user models.User
	err := json.NewDecoder(r.Body).Decode(&user)
	if err != nil {
		http.Error(w, "Invalid request payload", http.StatusBadRequest)
		return
	}
	userid, err := h.Service.Signup(user)
	if err != nil {
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}

	user.UserID = userid
	user.Password = ""
	token, err := h.Service.GenerateJWT(user.Email, user.UserID)
	if err != nil {
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}

	w.WriteHeader(http.StatusCreated)
	json.NewEncoder(w).Encode(map[string]interface{}{"code": "200", "message": "User created successfully", "user_details": user, "token": token})
}

// Login handles the user authentication request
func (h *AuthHandler) Login(w http.ResponseWriter, r *http.Request) {
	var credentials models.User
	w.Header().Set("Content-Type", "application/json")
	err := json.NewDecoder(r.Body).Decode(&credentials)
	if err != nil {
		http.Error(w, "Invalid request payload", http.StatusBadRequest)
		return
	}

	token, userDetails, err := h.Service.Login(credentials.Email, credentials.Password)
	if err != nil {
		http.Error(w, err.Error(), http.StatusUnauthorized)
		return
	}

	w.WriteHeader(http.StatusOK)
	json.NewEncoder(w).Encode(map[string]interface{}{"token": token, "user_details": userDetails})
}

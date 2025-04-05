package profileService

import (
	"database/sql"
	"encoding/json"
	"net/http"

	"github.com/golang-jwt/jwt/v5"
	"github.com/nikhil/eaven/internal/database.go"
	"github.com/nikhil/eaven/internal/middleware"
	models "github.com/nikhil/eaven/internal/models/users"
)

type ProfileService struct {
	DB *sql.DB
}

func NewProfileService() *ProfileService {
	return &ProfileService{
		DB: database.DB,
	}
}
func (profile *ProfileService) GetUserProfile(w http.ResponseWriter, r *http.Request) {
	userDetails, ok := r.Context().Value(middleware.UserContextKey).(jwt.MapClaims)
	if !ok {
		http.Error(w, "Invalid token", http.StatusUnauthorized)
		return
	}
	// var user map[string]interface{}
	// query := "Select * from users where user_id =  ?"
	// profile.DB.QueryRow(query, userDetails["user_id"]).Scan(&user)
	user, _ := database.GetSqlQueryRow("Select user_id , email , contact_number , first_name , last_name, created_at from users where user_id =  ?", userDetails["user_id"])
	user["name"] = user["first_name"].(string) + " " + user["last_name"].(string)
	json.NewEncoder(w).Encode(map[string]interface{}{"code": "200", "message": "User details", "user_details": user})
}

func (profile *ProfileService) UpdateUserProfile(w http.ResponseWriter, r *http.Request) {
	userDetails, ok := r.Context().Value(middleware.UserContextKey).(jwt.MapClaims)
	if !ok {
		http.Error(w, "Invalid token", http.StatusUnauthorized)
		return
	}
	var user models.User
	err := json.NewDecoder(r.Body).Decode(&user)
	if err != nil {
		http.Error(w, "Invalid request payload", http.StatusBadRequest)
		return
	}
	query := "UPDATE users SET contact_number = ? , first_name = ? , last_name = ? WHERE user_id = ?"
	err = database.SendSqlStatement(query, user.ContactNumber, user.FirstName, user.LastName, userDetails["user_id"])
	if err != nil {
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}

	w.WriteHeader(http.StatusOK)
	json.NewEncoder(w).Encode(map[string]interface{}{"code": "200", "message": "User details updated successfully"})
}
